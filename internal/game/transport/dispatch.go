package transport

import (
	"fmt"
	"strings"

	"github.com/RoseKhlifa/FarmBot/internal/game/pb"
	"google.golang.org/protobuf/proto"
)

// EventType identifies a decoded gateway event. Values preserve the event
// names consumed by the Node implementation where there is an equivalent.
type EventType string

const (
	EventKickout                   EventType = "kickout"
	EventLandsChanged              EventType = "landsChanged"
	EventItemChanged               EventType = "itemChanged"
	EventBasicChanged              EventType = "basicChanged"
	EventFriendApplicationReceived EventType = "friendApplicationReceived"
	EventFriendAdded               EventType = "friendAdded"
	EventGoodsUnlockNotify         EventType = "goodsUnlockNotify"
	EventTaskInfoNotify            EventType = "taskInfoNotify"
	EventSell                      EventType = "sell"
	EventFarmHarvested             EventType = "farmHarvested"
	EventUnknownNotify             EventType = "notify"
	EventDisconnect                EventType = "disconnect"
	EventDecodeError               EventType = "decodeError"
)

// Event is the transport-to-runtime notification envelope. Payload is one of
// the generated protobuf notification types for known events; Raw is retained
// for unknown event types so a later protocol extension does not lose bytes.
type Event struct {
	Type       EventType
	Name       string
	Payload    proto.Message
	Meta       *pb.Meta
	Raw        []byte
	Reason     string
	ReasonCode int64
	Err        error
}

// GatewayError is returned for a non-zero response meta.error_code.
type GatewayError struct {
	Meta *pb.Meta
}

func (e *GatewayError) Error() string {
	if e == nil || e.Meta == nil {
		return "game gateway request failed"
	}
	return fmt.Sprintf("%s.%s error: code=%d %s", e.Meta.ServiceName, e.Meta.MethodName, e.Meta.ErrorCode, e.Meta.ErrorMessage)
}

func dispatchNotify(client *Client, meta *pb.Meta, body []byte) {
	if len(body) == 0 {
		client.emit(Event{Type: EventUnknownNotify, Meta: meta})
		return
	}
	eventMessage := new(pb.EventMessage)
	if err := proto.Unmarshal(body, eventMessage); err != nil {
		client.emit(Event{Type: EventDecodeError, Meta: meta, Raw: append([]byte(nil), body...), Err: fmt.Errorf("decode notify envelope: %w", err)})
		return
	}
	name := eventMessage.MessageType
	normalized := strings.ToLower(name)
	event := Event{Name: name, Meta: meta, Raw: append([]byte(nil), eventMessage.Body...)}

	decode := func(message proto.Message, eventType EventType) {
		if err := proto.Unmarshal(eventMessage.Body, message); err != nil {
			event.Type = EventDecodeError
			event.Err = fmt.Errorf("decode %s notification: %w", name, err)
			client.emit(event)
			return
		}
		event.Type = eventType
		event.Payload = message
		client.emit(event)
	}

	switch {
	case strings.Contains(normalized, "kickout"):
		message := new(pb.KickoutNotify)
		if err := proto.Unmarshal(eventMessage.Body, message); err != nil {
			event.Type = EventDecodeError
			event.Err = fmt.Errorf("decode kickout notification: %w", err)
		} else {
			event.Type = EventKickout
			event.Payload = message
			event.Reason = message.ReasonMessage
			event.ReasonCode = message.Reason
		}
		client.emit(event)
	case strings.Contains(normalized, "landsnotify"):
		decode(new(pb.LandsNotify), EventLandsChanged)
	case strings.Contains(normalized, "itemnotify"):
		decode(new(pb.ItemNotify), EventItemChanged)
	case strings.Contains(normalized, "basicnotify"):
		decode(new(pb.BasicNotify), EventBasicChanged)
	case strings.Contains(normalized, "friendapplicationreceivednotify"):
		decode(new(pb.FriendApplicationReceivedNotify), EventFriendApplicationReceived)
	case strings.Contains(normalized, "friendaddednotify"):
		decode(new(pb.FriendAddedNotify), EventFriendAdded)
	case strings.Contains(normalized, "goodsunlocknotify"):
		decode(new(pb.GoodsUnlockNotify), EventGoodsUnlockNotify)
	case strings.Contains(normalized, "taskinfonotify"):
		decode(new(pb.TaskInfoNotify), EventTaskInfoNotify)
	default:
		event.Type = EventUnknownNotify
		client.emit(event)
	}
}

func (c *Client) emit(event Event) {
	if c == nil {
		return
	}
	if c.onEvent != nil {
		c.onEvent(event)
	}
	c.eventMu.RLock()
	defer c.eventMu.RUnlock()
	if c.eventsDone {
		return
	}
	select {
	case c.events <- event:
	default:
	}
}

// Publish forwards a Runtime-originated event through the same per-client
// channel used for gateway notifications. This keeps domain hooks such as
// sell/farmHarvested instance-scoped without putting domain logic in transport.
func (c *Client) Publish(event Event) { c.emit(event) }
