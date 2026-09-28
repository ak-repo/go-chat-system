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
	msg        *model.Message
	err        error
	recipients []string
	mentions   []model.MessageMention
}

func (f *fakeHubMessageService) CreateConversationMessageWithMentions(_ context.Context, _, _, _, _, _ string, mentions []model.MessageMention) (*model.Message, []string, error) {
	f.mentions = mentions
	f.msg.Mentions = mentions
	return f.msg, f.recipients, f.err
}

type fakePresenceService struct{ audience map[string][]string }

func (f fakePresenceService) Audience(_ context.Context, userID string) ([]string, error) {
	return f.audience[userID], nil
}

func (f fakeHubMessageService) CreateConversationMessage(context.Context, string, string, string, string, string) (*model.Message, []string, error) {
	return f.msg, f.recipients, f.err
}
func (f fakeHubMessageService) RealtimeRecipients(context.Context, string, string) ([]string, error) {
	return f.recipients, f.err
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

func TestRouteMessageCarriesStructuredMentions(t *testing.T) {
	service := &fakeHubMessageService{msg: &model.Message{ID: "server", Body: "hello @sam", ConversationID: "conversation", CreatedAt: time.Now()}, recipients: []string{"member"}}
	hub := NewHub(service)
	receiver := &Client{userID: "member", send: make(chan *WSMessage, 1)}
	hub.clients["member"] = map[*Client]bool{receiver: true}
	hub.routeMessage(&WSMessage{Event: EventMessage, SenderID: "actor", ReceiverType: ReceiverGroup, Data: json.RawMessage(`{"content":"hello @sam","conversation_id":"conversation","mentions":[{"kind":"user","user_id":"member","offset":6,"length":4}]}`)})
	if len(service.mentions) != 1 {
		t.Fatalf("mentions were not passed to persistence: %#v", service.mentions)
	}
	var payload struct {
		Mentions []model.MessageMention `json:"mentions"`
	}
	if err := json.Unmarshal((<-receiver.send).Data, &payload); err != nil || len(payload.Mentions) != 1 {
		t.Fatalf("mentions missing from event: %#v err=%v", payload, err)
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
		Data:         json.RawMessage(`{"content":"hello","client_message_id":"7de32ef5-6cdf-4167-8834-95f7089a17c1","conversation_id":"conversation-1"}`),
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
		var data struct {
			Code            string `json:"code"`
			Message         string `json:"message"`
			ClientMessageID string `json:"client_message_id"`
			ConversationID  string `json:"conversation_id"`
		}
		if err := json.Unmarshal(got.Data, &data); err != nil {
			t.Fatal(err)
		}
		if data.Code != "internal_error" || data.Message != "event could not be processed" || data.ClientMessageID != "7de32ef5-6cdf-4167-8834-95f7089a17c1" || data.ConversationID != "conversation-1" {
			t.Fatalf("unexpected safe correlated error: %#v", data)
		}
	default:
		t.Fatalf("expected sender error")
	}
}

func TestRouteMessageValidationErrorIncludesAvailableCorrelation(t *testing.T) {
	hub := NewHub(fakeHubMessageService{})
	sender := &Client{userID: "sender-1", send: make(chan *WSMessage, 1)}
	hub.clients[sender.userID] = map[*Client]bool{sender: true}
	hub.routeMessage(&WSMessage{Event: EventMessage, SenderID: sender.userID, ReceiverID: "receiver-1", ReceiverType: ReceiverUser, Data: json.RawMessage(`{"client_message_id":"7de32ef5-6cdf-4167-8834-95f7089a17c1","conversation_id":"conversation-1"}`)})

	var data struct {
		Code            string `json:"code"`
		ClientMessageID string `json:"client_message_id"`
		ConversationID  string `json:"conversation_id"`
	}
	got := <-sender.send
	if err := json.Unmarshal(got.Data, &data); err != nil {
		t.Fatal(err)
	}
	if got.Event != EventError || data.Code != "invalid_event" || data.ClientMessageID == "" || data.ConversationID != "conversation-1" {
		t.Fatalf("unexpected validation error: event=%s data=%#v", got.Event, data)
	}
}

func TestRouteGroupMessageFansOutOnlyToPersistedRecipients(t *testing.T) {
	cid := "conversation-1"
	hub := NewHub(fakeHubMessageService{msg: &model.Message{ID: "group-message", SenderID: "sender", Body: "hello", ConversationID: cid, IsGroup: true, CreatedAt: time.Now()}, recipients: []string{"member-1", "member-2"}})
	for _, id := range []string{"sender", "member-1", "member-2", "outsider"} {
		c := &Client{userID: id, send: make(chan *WSMessage, 2)}
		hub.clients[id] = map[*Client]bool{c: true}
	}
	hub.routeMessage(&WSMessage{Event: EventMessage, actorID: "sender", ReceiverType: ReceiverGroup, Data: json.RawMessage(`{"conversation_id":"conversation-1","content":"hello"}`)})
	for _, id := range []string{"member-1", "member-2"} {
		for c := range hub.clients[id] {
			select {
			case got := <-c.send:
				if got.ReceiverID != id || got.Event != EventMessage {
					t.Fatalf("bad delivery to %s: %#v", id, got)
				}
			default:
				t.Fatalf("missing delivery to %s", id)
			}
		}
	}
	for c := range hub.clients["outsider"] {
		select {
		case <-c.send:
			t.Fatal("outsider received group message")
		default:
		}
	}
	for c := range hub.clients["sender"] {
		select {
		case got := <-c.send:
			if got.Event != EventAck {
				t.Fatalf("sender expected ack, got %s", got.Event)
			}
		default:
			t.Fatal("sender missing ack")
		}
	}
}

func TestRouteMessageUsesAuthenticatedActorOverWireSender(t *testing.T) {
	hub := NewHub(fakeHubMessageService{msg: &model.Message{ID: "id", SenderID: "trusted", ReceiverID: "receiver", Body: "hello", CreatedAt: time.Now()}})
	sender := &Client{userID: "trusted", send: make(chan *WSMessage, 2)}
	receiver := &Client{userID: "receiver", send: make(chan *WSMessage, 1)}
	hub.clients["trusted"] = map[*Client]bool{sender: true}
	hub.clients["receiver"] = map[*Client]bool{receiver: true}
	hub.routeMessage(&WSMessage{Event: EventMessage, SenderID: "attacker", actorID: "trusted", ReceiverID: "attacker-selected-receiver", ReceiverType: ReceiverUser, Data: json.RawMessage(`{"content":"hello","conversation_id":"conversation"}`)})
	if got := (<-receiver.send).SenderID; got != "trusted" {
		t.Fatalf("sender identity was spoofed: %q", got)
	}
	if clients := hub.clients["attacker-selected-receiver"]; len(clients) != 0 {
		t.Fatal("wire receiver was used instead of the persisted recipient")
	}
}

func TestWSMessageRejectsUnknownAndClientAckEvents(t *testing.T) {
	for _, event := range []string{"unknown", EventAck} {
		if err := (&WSMessage{Event: event, Data: json.RawMessage(`{}`)}).Validate(); err == nil {
			t.Fatalf("expected %q to be rejected", event)
		}
	}
}

func TestWSMessageValidatesReactionPayload(t *testing.T) {
	valid := &WSMessage{Event: EventReactionAdded, ReceiverType: ReceiverGroup, Data: json.RawMessage(`{"conversation_id":"conversation-1","message_id":"message-1","reaction":"👍"}`)}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid reaction rejected: %v", err)
	}
	valid.Data = json.RawMessage(`{"conversation_id":"conversation-1","message_id":"message-1"}`)
	if err := valid.Validate(); err == nil {
		t.Fatal("reaction without value accepted")
	}
}

func TestClientUnregisterIsIdempotent(t *testing.T) {
	hub := NewHub(fakeHubMessageService{})
	c := &Client{hub: hub, userID: "user", send: make(chan *WSMessage, 1)}
	go c.unregister()
	select {
	case got := <-hub.unregister:
		if got != c {
			t.Fatal("unexpected client unregister event")
		}
	case <-time.After(time.Second):
		t.Fatal("client did not unregister")
	}
	c.unregister()
	select {
	case <-hub.unregister:
		t.Fatal("client emitted duplicate unregister event")
	default:
	}
}

func TestTypingRequiresConversation(t *testing.T) {
	m := &WSMessage{Event: EventTyping, ReceiverType: ReceiverUser, ReceiverID: "u", Data: json.RawMessage(`{"state":"started"}`)}
	if err := m.Validate(); err == nil {
		t.Fatal("typing event without conversation must be rejected")
	}
}

func TestMessageRejectsInvalidClientUUID(t *testing.T) {
	_, _, err := extractMessage(json.RawMessage(`{"content":"hello","client_message_id":"not-a-uuid"}`))
	if err == nil {
		t.Fatal("expected invalid client UUID to be rejected")
	}
}

func TestPresenceAudienceAndMultiSocketLifecycle(t *testing.T) {
	hub := NewHub(fakeHubMessageService{}, fakePresenceService{audience: map[string][]string{"user": {"observer"}}})
	go hub.Run()
	defer hub.Stop()

	observer := &Client{userID: "observer", send: make(chan *WSMessage, 8)}
	first := &Client{userID: "user", send: make(chan *WSMessage, 8)}
	second := &Client{userID: "user", send: make(chan *WSMessage, 8)}
	hub.Register(observer)
	waitEvent(t, observer.send, EventPresenceSnapshot)
	hub.Register(first)
	waitEvent(t, observer.send, EventOnline)
	waitEvent(t, first.send, EventPresenceSnapshot)
	hub.Register(second)
	waitEvent(t, second.send, EventPresenceSnapshot)

	hub.unregister <- first
	select {
	case event := <-observer.send:
		if event.Event == EventOffline {
			t.Fatal("first of two sockets emitted offline")
		}
	case <-time.After(30 * time.Millisecond):
	}
	hub.unregister <- second
	waitEvent(t, observer.send, EventOffline)
}

func TestPresenceDoesNotReachUsersOutsideAudience(t *testing.T) {
	hub := NewHub(fakeHubMessageService{})
	authorized := &Client{userID: "authorized", send: make(chan *WSMessage, 1)}
	outsider := &Client{userID: "outsider", send: make(chan *WSMessage, 1)}
	hub.clients[authorized.userID] = map[*Client]bool{authorized: true}
	hub.clients[outsider.userID] = map[*Client]bool{outsider: true}
	hub.deliverPresence(presenceResult{userID: "actor", event: EventOnline, timestamp: time.Now(), audience: []string{"authorized"}})
	if got := <-authorized.send; got.Event != EventOnline {
		t.Fatalf("authorized user got %q", got.Event)
	}
	select {
	case <-outsider.send:
		t.Fatal("presence leaked outside authorized audience")
	default:
	}
}

func waitEvent(t *testing.T, events <-chan *WSMessage, name string) *WSMessage {
	t.Helper()
	deadline := time.After(time.Second)
	for {
		select {
		case event := <-events:
			if event.Event == name {
				return event
			}
		case <-deadline:
			t.Fatalf("timed out waiting for %s", name)
		}
	}
}
