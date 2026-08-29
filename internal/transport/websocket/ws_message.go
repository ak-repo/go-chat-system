package websocket

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"strings"
)

type ReceiverType string

const (
	ReceiverUser  ReceiverType = "user"
	ReceiverGroup ReceiverType = "group"
)

type WSMessage struct {
	Event        string          `json:"event"`
	SenderID     string          `json:"sender_id,omitempty"`
	ReceiverID   string          `json:"receiver_id,omitempty"`
	ReceiverType ReceiverType    `json:"receiver_type,omitempty"`
	Data         json.RawMessage `json:"data"`
	// actorID is populated by ReadPump and is deliberately not serialised. It
	// is the authenticated identity, unlike SenderID which is wire data.
	actorID string
}

const (
	EventMessage   = "message"
	EventEdited    = "message.edited"
	EventDeleted   = "message.deleted"
	EventReplied   = "message.replied"
	EventDelivered = "message.delivered"
	EventRead      = "message.read"
	EventTyping    = "typing"
	EventAck       = "ack"
	EventError     = "error"
)

// Validate checks the envelope and the event's discriminated payload. Unknown
// events are rejected rather than accidentally routed as arbitrary messages.
func (m *WSMessage) Validate() error {
	if m == nil || strings.TrimSpace(m.Event) == "" || len(m.Data) == 0 || !json.Valid(m.Data) {
		return errors.New("invalid websocket event")
	}
	if m.Event == EventAck || m.Event == EventError {
		return errors.New("client cannot send this event")
	}
	var p map[string]json.RawMessage
	if json.Unmarshal(m.Data, &p) != nil {
		return errors.New("event data must be an object")
	}
	required := func(names ...string) error {
		for _, n := range names {
			if v, ok := p[n]; !ok || len(v) == 0 || string(v) == "null" {
				return errors.New("missing event field: " + n)
			}
		}
		return nil
	}
	switch m.Event {
	case EventMessage:
		if m.ReceiverType != ReceiverUser || m.ReceiverID == "" {
			return errors.New("message requires a user receiver")
		}
		return required("content")
	case EventEdited, EventReplied:
		if m.ReceiverType != ReceiverUser || m.ReceiverID == "" {
			return errors.New("mutation requires a user receiver")
		}
		if err := required("message_id", "content", "conversation_id"); err != nil {
			return err
		}
		var x struct{ Content, ClientMessageID string }
		if json.Unmarshal(m.Data, &x) != nil || len(x.Content) > 4096 || len(x.ClientMessageID) > 128 {
			return errors.New("event data too large")
		}
		return nil
	case EventDeleted, EventDelivered:
		if m.ReceiverType != ReceiverUser || m.ReceiverID == "" {
			return errors.New("mutation requires a user receiver")
		}
		return required("message_id", "conversation_id")
	case EventRead:
		if m.ReceiverType != ReceiverUser || m.ReceiverID == "" {
			return errors.New("read requires a user receiver")
		}
		return required("message_id", "conversation_id")
	case EventTyping:
		if m.ReceiverType != ReceiverUser || m.ReceiverID == "" {
			return errors.New("typing requires a user receiver")
		}
		return required("state", "conversation_id")
	default:
		return errors.New("unknown websocket event")
	}
}

func validClientID(v string) bool { return v == "" || (len(v) <= 128 && uuid.Validate(v) == nil) }

// Event → routing & intent
// SenderID → injected by server only
// ReceiverID + ReceiverType → supports user + group
// Data → flexible payload per event
