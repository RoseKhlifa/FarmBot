package transport

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

type xorCipher struct{ key byte }

func (c xorCipher) Encrypt(value []byte) ([]byte, error) { return xorBytes(value, c.key), nil }
func (c xorCipher) Decrypt(value []byte) ([]byte, error) { return xorBytes(value, c.key), nil }

func xorBytes(value []byte, key byte) []byte {
	result := append([]byte(nil), value...)
	for index := range result {
		result[index] ^= key
	}
	return result
}

func wsTestServer(t *testing.T, handler func(*websocket.Conn)) string {
	t.Helper()
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			t.Errorf("upgrade websocket: %v", err)
			return
		}
		handler(connection)
	}))
	t.Cleanup(server.Close)
	return "ws" + strings.TrimPrefix(server.URL, "http")
}

func TestSendMsgEncryptsCorrelatesAndDecodes(t *testing.T) {
	var requestMu sync.Mutex
	var sequences []int64
	var authTokens []string
	serverURL := wsTestServer(t, func(connection *websocket.Conn) {
		defer connection.Close()
		cipher := xorCipher{key: 0x5a}
		for index := 0; index < 2; index++ {
			_, frame, err := connection.ReadMessage()
			if err != nil {
				t.Errorf("read request: %v", err)
				return
			}
			request := new(pb.Message)
			if err := proto.Unmarshal(frame, request); err != nil {
				t.Errorf("decode request: %v", err)
				return
			}
			requestMu.Lock()
			sequences = append(sequences, request.Meta.ClientSeq)
			authTokens = append(authTokens, request.AuthToken)
			requestMu.Unlock()
			body, err := cipher.Decrypt(request.Body)
			if err != nil {
				t.Errorf("decrypt request: %v", err)
				return
			}
			loginRequest := new(pb.LoginRequest)
			if err := proto.Unmarshal(body, loginRequest); err != nil {
				t.Errorf("decode request body: %v", err)
				return
			}
			if loginRequest.SharerId != int64(index+1) {
				t.Errorf("request body sharer_id = %d, want %d", loginRequest.SharerId, index+1)
			}
			replyBody, err := proto.Marshal(&pb.LoginReply{TimeNowMillis: int64(index + 100)})
			if err != nil {
				t.Errorf("encode reply: %v", err)
				return
			}
			replyBody = cipher.EncryptMust(replyBody)
			reply := &pb.Message{
				Meta: &pb.Meta{
					ServiceName: request.Meta.ServiceName,
					MethodName:  request.Meta.MethodName,
					MessageType: int32(pb.MessageType_Response),
					ClientSeq:   request.Meta.ClientSeq,
					ServerSeq:   int64(index + 1),
				},
				Body: replyBody,
			}
			encoded, _ := proto.Marshal(reply)
			if err := connection.WriteMessage(websocket.BinaryMessage, encoded); err != nil {
				t.Errorf("write reply: %v", err)
				return
			}
		}
	})

	client, err := Dial(context.Background(), serverURL, Options{
		Cipher:           xorCipher{key: 0x5a},
		InitialAuthToken: "one-time-init",
	})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()

	for index := 0; index < 2; index++ {
		response, err := client.SendMsg(context.Background(), Command{
			ServiceName: "gamepb.userpb.UserService",
			MethodName:  "Login",
			Response:    new(pb.LoginReply),
		}, &pb.LoginRequest{SharerId: int64(index + 1)})
		if err != nil {
			t.Fatalf("send request %d: %v", index, err)
		}
		if response.(*pb.LoginReply).TimeNowMillis != int64(index+100) {
			t.Fatalf("response %d time_now_millis = %d", index, response.(*pb.LoginReply).TimeNowMillis)
		}
	}

	requestMu.Lock()
	defer requestMu.Unlock()
	if want := []int64{1, 2}; !equalInt64s(sequences, want) {
		t.Fatalf("sequences = %v, want %v", sequences, want)
	}
	if authTokens[0] != "one-time-init" {
		t.Fatalf("first auth token = %q", authTokens[0])
	}
	if len(authTokens[1]) < 65 || len(authTokens[1]) > 128 || !strings.HasSuffix(authTokens[1], "=") {
		t.Fatalf("second auth token has invalid shape: %q", authTokens[1])
	}
}

func TestSendMsgTimeoutRemovesPending(t *testing.T) {
	serverURL := wsTestServer(t, func(connection *websocket.Conn) {
		defer connection.Close()
		_, _, _ = connection.ReadMessage()
		time.Sleep(150 * time.Millisecond)
	})
	client, err := Dial(context.Background(), serverURL, Options{DefaultTimeout: 30 * time.Millisecond})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	_, err = client.SendMsg(context.Background(), Command{
		ServiceName: "service",
		MethodName:  "method",
		Response:    new(pb.LoginReply),
	}, nil)
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("timeout error = %v", err)
	}
	client.pendingMu.Lock()
	pending := len(client.pending)
	client.pendingMu.Unlock()
	if pending != 0 {
		t.Fatalf("pending requests after timeout = %d", pending)
	}
}

func TestKickoutNotifyIsDispatched(t *testing.T) {
	serverURL := wsTestServer(t, func(connection *websocket.Conn) {
		defer connection.Close()
		_, frame, err := connection.ReadMessage()
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		request := new(pb.Message)
		if err := proto.Unmarshal(frame, request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		kickout, _ := proto.Marshal(&pb.KickoutNotify{Reason: 7, ReasonMessage: "another login"})
		eventBody, _ := proto.Marshal(&pb.EventMessage{MessageType: "gamepb.userpb.KickoutNotify", Body: kickout})
		notify, _ := proto.Marshal(&pb.Message{
			Meta: &pb.Meta{MessageType: int32(pb.MessageType_Notify), ServerSeq: 9},
			Body: eventBody,
		})
		_ = connection.WriteMessage(websocket.BinaryMessage, notify)
	})
	client, err := Dial(context.Background(), serverURL, Options{EventBuffer: 4})
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer client.Close()
	_, err = client.SendMsg(context.Background(), Command{
		ServiceName: "service",
		MethodName:  "method",
		Response:    new(pb.LoginReply),
		Timeout:     100 * time.Millisecond,
	}, nil)
	if err == nil {
		t.Fatal("expected request to time out because notify has no response")
	}
	select {
	case event := <-client.Events():
		if event.Type != EventKickout || event.Reason != "another login" || event.ReasonCode != 7 {
			t.Fatalf("event = %+v", event)
		}
		if _, ok := event.Payload.(*pb.KickoutNotify); !ok {
			t.Fatalf("payload type = %T", event.Payload)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for kickout event")
	}
}

func equalInt64s(got, want []int64) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}

func (c xorCipher) EncryptMust(value []byte) []byte {
	result, err := c.Encrypt(value)
	if err != nil {
		panic(err)
	}
	return result
}
