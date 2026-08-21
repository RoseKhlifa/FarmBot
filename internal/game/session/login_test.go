package session

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/RoseKhlifa/FarmBot/internal/game/ace"
	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"github.com/RoseKhlifa/FarmBot/internal/game/transport"
	"github.com/RoseKhlifa/FarmBot/internal/game/tsdk"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
)

type fakeSecurityRuntime struct {
	mu        sync.Mutex
	ready     bool
	destroyed bool
	bound     string
	initToken string
}

func (r *fakeSecurityRuntime) Init(context.Context) error {
	r.mu.Lock()
	r.ready = true
	r.mu.Unlock()
	return nil
}

func (r *fakeSecurityRuntime) Destroy() error {
	r.mu.Lock()
	r.ready = false
	r.destroyed = true
	r.mu.Unlock()
	return nil
}

func (r *fakeSecurityRuntime) BindUser(openID string) error {
	r.mu.Lock()
	r.bound = openID
	r.mu.Unlock()
	return nil
}

func (r *fakeSecurityRuntime) GetEncryptedInitInfo() (string, error)     { return r.initToken, nil }
func (r *fakeSecurityRuntime) Encrypt(value []byte) ([]byte, error)      { return xor(value), nil }
func (r *fakeSecurityRuntime) Decrypt(value []byte) ([]byte, error)      { return xor(value), nil }
func (r *fakeSecurityRuntime) ProcessReceivedData() error                { return nil }
func (r *fakeSecurityRuntime) HeartbeatTick() error                      { return nil }
func (r *fakeSecurityRuntime) DetectSpeedHack(time.Duration) error       { return nil }
func (r *fakeSecurityRuntime) SendStatus() error                         { return nil }
func (r *fakeSecurityRuntime) CheckFunctionArray([]string, uint32) error { return nil }
func (r *fakeSecurityRuntime) GetDataToServer() ([]byte, error)          { return nil, nil }
func (r *fakeSecurityRuntime) SendDataFromServer([]byte) error           { return nil }

func (r *fakeSecurityRuntime) GetStatus() tsdk.Status {
	r.mu.Lock()
	defer r.mu.Unlock()
	return tsdk.Status{Ready: r.ready, Destroyed: r.destroyed}
}

func xor(value []byte) []byte {
	result := append([]byte(nil), value...)
	for index := range result {
		result[index] ^= 0x5a
	}
	return result
}

type observedRequest struct {
	service string
	method  string
	token   string
	body    []byte
}

func TestLoginCompletesHandshakeAndSupportsFriendOperation(t *testing.T) {
	t.Setenv("FARM_MASTER_KEY", "test-master-key")
	release := make(chan struct{})
	var releaseOnce sync.Once
	releaseServer := func() { releaseOnce.Do(func() { close(release) }) }
	requests := make(chan observedRequest, 3)
	serverErrors := make(chan error, 1)
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Query().Get("code") != "code with spaces" || request.URL.Query().Get("platform") != "qq" || request.URL.Query().Get("os") != "iOS" || request.URL.Query().Get("ver") != "test-version" {
			serverErrors <- errors.New("gateway query does not contain the login contract")
			return
		}
		if request.Header.Get("Origin") != defaultOrigin || request.Header.Get("User-Agent") != "test-agent" {
			serverErrors <- errors.New("gateway headers do not contain the login contract")
			return
		}
		connection, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			serverErrors <- err
			return
		}
		defer func() { _ = connection.Close() }()

		for index := 0; index < 3; index++ {
			_, frame, err := connection.ReadMessage()
			if err != nil {
				serverErrors <- err
				return
			}
			message := new(pb.Message)
			if err := proto.Unmarshal(frame, message); err != nil {
				serverErrors <- err
				return
			}
			body := xor(message.Body)
			requests <- observedRequest{message.Meta.ServiceName, message.Meta.MethodName, message.AuthToken, body}

			var response proto.Message
			switch index {
			case 0:
				response = &pb.LoginReply{Basic: &pb.BasicInfo{Gid: 42, Name: "farmer", Level: 7, Gold: 99, Exp: 12, OpenId: "openid-42", AvatarUrl: "avatar"}}
			case 1:
				response = &pb.AllLandsReply{Lands: []*pb.LandInfo{{Id: 1}}}
			case 2:
				response = &pb.EnterReply{Basic: &pb.BasicInfo{Gid: 77, Name: "friend"}}
			}
			responseBody, err := proto.Marshal(response)
			if err != nil {
				serverErrors <- err
				return
			}
			reply, err := proto.Marshal(&pb.Message{
				Meta: &pb.Meta{ServiceName: message.Meta.ServiceName, MethodName: message.Meta.MethodName, MessageType: int32(pb.MessageType_Response), ClientSeq: message.Meta.ClientSeq, ServerSeq: int64(index + 1)},
				// The production gateway returns plaintext protobuf responses. The
				// session fallback must recover these after P2-04 decrypts them.
				Body: responseBody,
			})
			if err != nil {
				serverErrors <- err
				return
			}
			if err := connection.WriteMessage(websocket.BinaryMessage, reply); err != nil {
				serverErrors <- err
				return
			}
		}
		<-release
	}))
	t.Cleanup(server.Close)
	t.Cleanup(releaseServer)

	runtime := &fakeSecurityRuntime{initToken: "one-time-init-info"}
	session, err := LoginWithOptions(context.Background(), " code with spaces ", Options{
		AccountID:     "account-42",
		UIN:           "10001",
		GatewayURL:    "ws" + strings.TrimPrefix(server.URL, "http"),
		ClientVersion: "test-version",
		Platform:      "qq",
		OS:            "iOS",
		UserAgent:     "test-agent",
		Runtime:       runtime,
		ACEIntervals:  slowACEIntervals(),
	})
	if err != nil {
		t.Fatalf("LoginWithOptions: %v", err)
	}
	if !session.Online() || session.OpenID != "openid-42" || session.UIN != "10001" || session.GID != 42 {
		t.Fatalf("unexpected online session: %+v online=%v", session, session.Online())
	}
	state := session.State()
	if state.Name != "farmer" || state.Level != 7 || state.OpenID != "openid-42" {
		t.Fatalf("unexpected user state: %+v", state)
	}
	if lands := session.InitialLands(); lands == nil || len(lands.Lands) != 1 || lands.Lands[0].Id != 1 {
		t.Fatalf("unexpected initial lands: %+v", lands)
	}
	if status := session.ACEStatus(); !status.Running || !status.Runtime.Ready {
		t.Fatalf("ACE did not start after login: %+v", status)
	}

	friendReply, err := session.SendMsg(context.Background(), transport.Command{
		ServiceName: "gamepb.visitpb.VisitService",
		MethodName:  "Enter",
		Response:    new(pb.EnterReply),
	}, &pb.EnterRequest{HostGid: 77, Reason: int32(pb.EnterReason_ENTER_REASON_FRIEND)})
	if err != nil {
		t.Fatalf("friend operation: %v", err)
	}
	if friendReply.(*pb.EnterReply).GetBasic().GetGid() != 77 {
		t.Fatalf("unexpected friend reply: %+v", friendReply)
	}

	first, second, third := <-requests, <-requests, <-requests
	if first.service != userService || first.method != loginMethod {
		t.Fatalf("first request = %s.%s", first.service, first.method)
	}
	var loginRequest pb.LoginRequest
	if err := proto.Unmarshal(first.body, &loginRequest); err != nil || loginRequest.GetSceneId() != "1256" || loginRequest.GetDeviceInfo().GetClientVersion() != "test-version" {
		t.Fatalf("unexpected login request: %+v err=%v", &loginRequest, err)
	}
	if second.service != plantService || second.method != landsMethod || second.token != "one-time-init-info" {
		t.Fatalf("initial AllLands did not consume init credential: %+v", second)
	}
	if first.token == "" || first.token == second.token || third.token == "" || third.token == second.token {
		t.Fatalf("unexpected gateway token sequence: login=%q lands=%q friend=%q", first.token, second.token, third.token)
	}
	runtime.mu.Lock()
	bound := runtime.bound
	runtime.mu.Unlock()
	if bound != "openid-42" {
		t.Fatalf("TSDK bound openid = %q", bound)
	}

	if err := session.Close(); err != nil {
		t.Fatalf("close session: %v", err)
	}
	releaseServer()
	select {
	case err := <-serverErrors:
		t.Fatalf("gateway server: %v", err)
	default:
	}
	if session.Online() || session.ACEStatus().Running {
		t.Fatal("session resources remained online after close")
	}
}

func TestLoginFailureDestroysAccountRuntime(t *testing.T) {
	t.Setenv("FARM_MASTER_KEY", "test-master-key")
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		connection, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			return
		}
		defer func() { _ = connection.Close() }()
		_, frame, err := connection.ReadMessage()
		if err != nil {
			return
		}
		message := new(pb.Message)
		if proto.Unmarshal(frame, message) != nil {
			return
		}
		body, _ := proto.Marshal(&pb.LoginReply{})
		reply, _ := proto.Marshal(&pb.Message{Meta: &pb.Meta{MessageType: int32(pb.MessageType_Response), ClientSeq: message.Meta.ClientSeq}, Body: xor(body)})
		_ = connection.WriteMessage(websocket.BinaryMessage, reply)
	}))
	defer server.Close()

	runtime := &fakeSecurityRuntime{initToken: "init"}
	_, err := LoginWithOptions(context.Background(), "code", Options{
		GatewayURL:   "ws" + strings.TrimPrefix(server.URL, "http"),
		Runtime:      runtime,
		ACEIntervals: slowACEIntervals(),
	})
	if err == nil || !strings.Contains(err.Error(), "no basic account information") {
		t.Fatalf("login error = %v", err)
	}
	runtime.mu.Lock()
	destroyed := runtime.destroyed
	runtime.mu.Unlock()
	if !destroyed {
		t.Fatal("failed login did not destroy TSDK runtime")
	}
}

func TestBuildGatewayURLRejectsNonWebSocketURL(t *testing.T) {
	_, err := buildGatewayURL(Options{GatewayURL: "https://example.test/game"}, "code")
	if err == nil {
		t.Fatal("accepted a non-WebSocket gateway URL")
	}
	got, err := buildGatewayURL(Options{GatewayURL: "wss://example.test/game?existing=1", Platform: "qq", OS: "iOS", ClientVersion: "v"}, "a+b&c")
	if err != nil {
		t.Fatal(err)
	}
	parsed, _ := url.Parse(got)
	if parsed.Query().Get("code") != "a+b&c" || parsed.Query().Get("existing") != "1" {
		t.Fatalf("gateway query was not safely merged: %s", got)
	}
}

func slowACEIntervals() ace.Intervals {
	return ace.Intervals{
		Process: time.Hour, Poll: time.Hour, ACEHeartbeat: time.Hour,
		UserHeartbeat: time.Hour, SpeedCheck: time.Hour, Status: time.Hour,
		FunctionCheck: time.Hour, Request: time.Second,
		BackoffMin: time.Millisecond, BackoffMax: time.Second,
	}
}
