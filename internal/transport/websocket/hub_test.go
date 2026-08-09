package websocket

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
)

type fakeHubMessageService struct {
	msg *model.Message
	err error
}

func (f fakeHubMessageService) CreateMessage(context.Context, string, string, string, bool) (*model.Message, error) {
	return f.msg, f.err
}

func (f fakeHubMessageService) GetConversation(context.Context, string, string, int, int) (model.Messages, error) {
	return nil, nil
}

func (f fakeHubMessageService) GetMessages(http.ResponseWriter, *http.Request) (int, *utils.APIResponse, error) {
	return http.StatusOK, nil, nil
}

func TestExtractMessageTextSupportsTextAndContent(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "text field",
			data: `{"text":"hello"}`,
			want: "hello",
		},
		{
			name: "content field",
			data: `{"content":"hello"}`,
			want: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractMessageText(json.RawMessage(tt.data))
			if err != nil {
				t.Fatalf("extractMessageText returned error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestRouteMessageDeliversPersistedMessage(t *testing.T) {
	hub := NewHub(fakeHubMessageService{msg: &model.Message{
		ID:         "server-msg-1",
		SenderID:   "sender-1",
		ReceiverID: "receiver-1",
		Body:       "hello",
		CreatedAt:  time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC),
	}})
	receiver := &Client{userID: "receiver-1", send: make(chan *WSMessage, 1)}
	sender := &Client{userID: "sender-1", send: make(chan *WSMessage, 1)}
	hub.clients["receiver-1"] = map[*Client]bool{receiver: true}
	hub.clients["sender-1"] = map[*Client]bool{sender: true}

	hub.routeMessage(&WSMessage{
		Event:        "message",
		SenderID:     "sender-1",
		ReceiverID:   "receiver-1",
		ReceiverType: ReceiverUser,
		Data:         json.RawMessage(`{"content":"client text"}`),
	})

	select {
	case got := <-receiver.send:
		if got.Event != "message" {
			t.Fatalf("expected message event, got %q", got.Event)
		}
		var data struct {
			MessageID string `json:"message_id"`
			Content   string `json:"content"`
		}
		if err := json.Unmarshal(got.Data, &data); err != nil {
			t.Fatalf("failed to unmarshal message data: %v", err)
		}
		if data.MessageID != "server-msg-1" || data.Content != "hello" {
			t.Fatalf("expected persisted message data, got %#v", data)
		}
	default:
		t.Fatalf("expected receiver delivery")
	}

	select {
	case got := <-sender.send:
		if got.Event != "ack" {
			t.Fatalf("expected ack event, got %q", got.Event)
		}
	default:
		t.Fatalf("expected sender ack")
	}
}

func TestRouteMessageSendsErrorWhenPersistenceFails(t *testing.T) {
	hub := NewHub(fakeHubMessageService{err: errors.New("persist failed")})
	receiver := &Client{userID: "receiver-1", send: make(chan *WSMessage, 1)}
	sender := &Client{userID: "sender-1", send: make(chan *WSMessage, 1)}
	hub.clients["receiver-1"] = map[*Client]bool{receiver: true}
	hub.clients["sender-1"] = map[*Client]bool{sender: true}

	hub.routeMessage(&WSMessage{
		Event:        "message",
		SenderID:     "sender-1",
		ReceiverID:   "receiver-1",
		ReceiverType: ReceiverUser,
		Data:         json.RawMessage(`{"content":"hello"}`),
	})

	select {
	case <-receiver.send:
		t.Fatalf("did not expect receiver delivery on persistence failure")
	default:
	}

	select {
	case got := <-sender.send:
		if got.Event != "error" {
			t.Fatalf("expected error event, got %q", got.Event)
		}
	default:
		t.Fatalf("expected sender error")
	}
}
