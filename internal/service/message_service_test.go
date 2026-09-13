package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
	"github.com/go-chi/chi"
	"github.com/google/uuid"
)

type fakeMessageRepo struct {
	messages     model.Messages
	err          error
	created      *model.Message
	target       *model.Message
	mutationErr  error
	markedStatus string
	createdErr   error
	editedID     string
	deletedID    string
	markedRead   string
	unread       map[string]int
}

func (f *fakeMessageRepo) CreateMessage(_ context.Context, msg *model.Message) error {
	f.created = msg
	return f.createdErr
}

func (f *fakeMessageRepo) GetMessagesByReceiver(context.Context, string, int, int) (model.Messages, error) {
	return nil, nil
}

func (f *fakeMessageRepo) GetMessagesBetweenUsers(context.Context, string, string, int, int) (model.Messages, error) {
	return f.messages, f.err
}
func (f *fakeMessageRepo) GetMessage(context.Context, string) (*model.Message, error) {
	return f.target, f.err
}
func (f *fakeMessageRepo) GetByClientMessageID(context.Context, string, string, string) (*model.Message, error) {
	return nil, nil
}
func (f *fakeMessageRepo) EditMessage(_ context.Context, id, _, _ string) error {
	f.editedID = id
	return f.mutationErr
}
func (f *fakeMessageRepo) DeleteMessage(_ context.Context, id, _ string) error {
	f.deletedID = id
	return f.mutationErr
}
func (f *fakeMessageRepo) CreateReply(_ context.Context, msg *model.Message) error {
	f.created = msg
	return f.mutationErr
}
func (f *fakeMessageRepo) MarkDelivery(_ context.Context, _, _, status string, _ time.Time) error {
	f.markedStatus = status
	return f.mutationErr
}
func (f *fakeMessageRepo) MarkRead(_ context.Context, _, _, id string, _ time.Time) error {
	f.markedRead = id
	return f.mutationErr
}
func (f *fakeMessageRepo) UnreadSummary(context.Context, string) (map[string]int, error) {
	if f.unread == nil {
		f.unread = map[string]int{}
	}
	return f.unread, f.err
}

type fakeFriendRepo struct {
	areFriends bool
	err        error
}

func (f fakeFriendRepo) CreateFriendship(context.Context, string, string) error { return nil }

func (f fakeFriendRepo) AreFriends(context.Context, string, string) (bool, error) {
	return f.areFriends, f.err
}

func (f fakeFriendRepo) ListFriends(context.Context, string, int, int) (model.FriendsDTO, error) {
	return nil, nil
}

type fakeBlockRepo struct {
	blocked bool
	err     error
}

func (f fakeBlockRepo) BlockUser(context.Context, string, string) error { return nil }

func (f fakeBlockRepo) UnblockUser(context.Context, string, string) error { return nil }

func (f fakeBlockRepo) IsBlocked(context.Context, string, string) (bool, error) {
	return f.blocked, f.err
}

func TestGetMessagesUsesMiddlewareUserIDKey(t *testing.T) {
	repo := fakeMessageRepo{
		messages: model.Messages{
			{
				ID:         uuid.NewString(),
				SenderID:   "user-1",
				ReceiverID: "user-2",
				Body:       "hello",
				CreatedAt:  time.Now().UTC(),
				ModifiedAt: time.Now().UTC(),
			},
		},
	}

	service := NewMessageServiceImpl(&repo, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-2&limit=50&offset=0", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))

	status, resp, err := service.GetMessages(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if resp == nil || resp.Data == nil {
		t.Fatalf("expected response data, got %#v", resp)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map response data, got %T", resp.Data)
	}
	if got := data["limit"]; got != 50 {
		t.Fatalf("expected limit 50, got %#v", got)
	}
	if got := data["offset"]; got != 0 {
		t.Fatalf("expected offset 0, got %#v", got)
	}
}

func TestGetMessagesRejectsMissingMiddlewareUserIDKey(t *testing.T) {
	repo := fakeMessageRepo{}
	service := NewMessageServiceImpl(&repo, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-2", nil)
	req = req.WithContext(context.WithValue(req.Context(), "userID", "user-1"))

	status, resp, err := service.GetMessages(httptest.NewRecorder(), req)
	if status != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, status)
	}
	if resp != nil {
		t.Fatalf("expected nil response, got %#v", resp)
	}
	if err == nil || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected unauthorized error, got %v", err)
	}
}

func TestGetMessagesReturnsEmptySliceWhenNoMessages(t *testing.T) {
	repo := fakeMessageRepo{messages: nil}
	service := NewMessageServiceImpl(&repo, nil, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages?user_id=user-2", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))

	status, resp, err := service.GetMessages(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}

	data, ok := resp.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map response data, got %T", resp.Data)
	}
	msgs, ok := data["messages"].(model.Messages)
	if !ok {
		t.Fatalf("expected model.Messages, got %T", data["messages"])
	}
	if msgs == nil {
		t.Fatalf("expected empty slice, got nil")
	}
	if len(msgs) != 0 {
		t.Fatalf("expected empty slice, got %d items", len(msgs))
	}
}

func TestCreateMessageRequiresFriendship(t *testing.T) {
	repo := &fakeMessageRepo{}
	service := NewMessageServiceImpl(repo, fakeFriendRepo{areFriends: false}, fakeBlockRepo{})

	_, err := service.CreateMessage(context.Background(), "user-1", "user-2", "hello", false)
	if !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("expected forbidden error, got %v", err)
	}
	if repo.created != nil {
		t.Fatalf("expected message not to be persisted")
	}
}

func TestCreateMessageRejectsBlockedRelationship(t *testing.T) {
	repo := &fakeMessageRepo{}
	service := NewMessageServiceImpl(repo, fakeFriendRepo{areFriends: true}, fakeBlockRepo{blocked: true})

	_, err := service.CreateMessage(context.Background(), "user-1", "user-2", "hello", false)
	if !errors.Is(err, errs.ErrBlockedRelationship) {
		t.Fatalf("expected blocked relationship error, got %v", err)
	}
	if repo.created != nil {
		t.Fatalf("expected message not to be persisted")
	}
}

func TestCreateMessagePersistsForFriends(t *testing.T) {
	repo := &fakeMessageRepo{}
	service := NewMessageServiceImpl(repo, fakeFriendRepo{areFriends: true}, fakeBlockRepo{})

	msg, err := service.CreateMessage(context.Background(), "user-1", "user-2", " hello ", false)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if repo.created == nil {
		t.Fatalf("expected message to be persisted")
	}
	if msg.Body != "hello" {
		t.Fatalf("expected trimmed body, got %q", msg.Body)
	}
}

func TestCreateMessageRejectsOversizedBody(t *testing.T) {
	repo := &fakeMessageRepo{}
	service := NewMessageServiceImpl(repo, fakeFriendRepo{areFriends: true}, fakeBlockRepo{})
	_, err := service.CreateMessage(context.Background(), "user-1", "user-2", string(make([]byte, 4097)), false)
	if !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if repo.created != nil {
		t.Fatal("oversized message was persisted")
	}
}

func TestHandleRealtimeAuthorizesMemberAndSender(t *testing.T) {
	conversation := &fakeConversationRepo{}
	message := &fakeMessageRepo{target: &model.Message{ID: "message-1", SenderID: "user-1", ReceiverID: "user-2", ConversationID: "conversation-1"}}
	service := NewMessageServiceImpl(message, fakeFriendRepo{areFriends: true}, fakeBlockRepo{})
	service.SetConversationRepository(conversation)
	result, err := service.HandleRealtimeResult(context.Background(), "user-1", RealtimeEvent{Event: "message.edited", ConversationID: "conversation-1", MessageID: "message-1", ReceiverID: "user-2", Content: " edited "})
	if err != nil || result == nil || result.Content != "edited" {
		t.Fatalf("expected authorized edit, got %#v, %v", result, err)
	}
	message.target.SenderID = "user-2"
	_, err = service.HandleRealtimeResult(context.Background(), "user-1", RealtimeEvent{Event: "message.edited", ConversationID: "conversation-1", MessageID: "message-1", ReceiverID: "user-2", Content: "edited"})
	if !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("expected sender authorization failure, got %v", err)
	}
}

func TestHandleRealtimeRejectsUnknownEventAndDependencyError(t *testing.T) {
	service := NewMessageServiceImpl(&fakeMessageRepo{}, fakeFriendRepo{areFriends: true}, fakeBlockRepo{})
	service.SetConversationRepository(&fakeConversationRepo{memberErr: errors.New("database unavailable")})
	_, err := service.HandleRealtimeResult(context.Background(), "user-1", RealtimeEvent{Event: "message.unknown", ConversationID: "conversation-1", MessageID: "message-1", ReceiverID: "user-2"})
	if err == nil {
		t.Fatalf("expected membership dependency error")
	}
	service.SetConversationRepository(&fakeConversationRepo{})
	_, err = service.HandleRealtimeResult(context.Background(), "user-1", RealtimeEvent{Event: "message.unknown", ConversationID: "conversation-1", MessageID: "message-1"})
	if !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("expected missing receiver to fail authorization, got %v", err)
	}
}

func messageRouteRequest(method, path, body, userID string) *http.Request {
	req := authenticatedRequest(method, body, userID)
	ctx := chi.NewRouteContext()
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "conversations" {
			ctx.URLParams.Add("conversationID", parts[i+1])
		}
		if parts[i] == "messages" {
			ctx.URLParams.Add("messageID", parts[i+1])
		}
	}
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, ctx))
}

func TestMessageHTTPMethodsMutateAndAuthorize(t *testing.T) {
	cid, mid := uuid.NewString(), uuid.NewString()
	conversation := &fakeConversationRepo{conversation: &model.Conversation{ID: cid, UserOneID: "user-1", UserTwoID: "user-2"}}
	target := &model.Message{ID: mid, SenderID: "user-1", ReceiverID: "user-2", ConversationID: cid}
	repo := &fakeMessageRepo{target: target, unread: map[string]int{cid: 2}}
	service := NewMessageServiceImpl(repo, fakeFriendRepo{areFriends: true}, fakeBlockRepo{})
	service.SetConversationRepository(conversation)
	status, _, err := service.History(httptest.NewRecorder(), messageRouteRequest(http.MethodGet, "/conversations/"+cid, "", "user-1"))
	if status != http.StatusOK || err != nil {
		t.Fatalf("expected history, got %d %v", status, err)
	}
	status, _, err = service.Send(httptest.NewRecorder(), messageRouteRequest(http.MethodPost, "/conversations/"+cid, `{"content":" hello "}`, "user-1"))
	if status != http.StatusCreated || err != nil || repo.created == nil {
		t.Fatalf("expected send, got %d %v", status, err)
	}
	status, _, err = service.Edit(httptest.NewRecorder(), messageRouteRequest(http.MethodPatch, "/messages/"+mid, `{"content":"edited"}`, "user-1"))
	if status != http.StatusOK || err != nil || repo.editedID != mid {
		t.Fatalf("expected edit, got %d %v", status, err)
	}
	status, _, err = service.Delete(httptest.NewRecorder(), messageRouteRequest(http.MethodDelete, "/messages/"+mid, "", "user-1"))
	if status != http.StatusOK || err != nil || repo.deletedID != mid {
		t.Fatalf("expected delete, got %d %v", status, err)
	}
	status, _, err = service.Delivery(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"message_id":"`+mid+`","status":"delivered"}`, "user-2"))
	if status != http.StatusOK || err != nil || repo.markedStatus != "delivered" {
		t.Fatalf("expected delivery, got %d %v", status, err)
	}
	status, _, err = service.Read(httptest.NewRecorder(), messageRouteRequest(http.MethodPost, "/conversations/"+cid, `{"message_id":"`+mid+`"}`, "user-2"))
	if status != http.StatusOK || err != nil || repo.markedRead != mid {
		t.Fatalf("expected read, got %d %v", status, err)
	}
	status, response, err := service.Unread(httptest.NewRecorder(), authenticatedRequest(http.MethodGet, "", "user-2"))
	if status != http.StatusOK || response == nil || err != nil {
		t.Fatalf("expected unread summary, got %d %v", status, err)
	}
}

func TestMessageHTTPMethodsValidateAndMapAuthorization(t *testing.T) {
	cid, mid := uuid.NewString(), uuid.NewString()
	repo := &fakeMessageRepo{target: &model.Message{ID: mid, SenderID: "user-1", ReceiverID: "user-2", ConversationID: cid}}
	conversation := &fakeConversationRepo{conversation: &model.Conversation{ID: cid, UserOneID: "user-1", UserTwoID: "user-2"}}
	service := NewMessageServiceImpl(repo, fakeFriendRepo{areFriends: false}, fakeBlockRepo{})
	service.SetConversationRepository(conversation)
	status, _, err := service.Send(httptest.NewRecorder(), messageRouteRequest(http.MethodPost, "/conversations/not-a-uuid", `{"content":"hello"}`, "user-1"))
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("expected send validation, got %d %v", status, err)
	}
	status, _, err = service.Delivery(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"message_id":"`+mid+`","status":"delivered"}`, "other"))
	if status != http.StatusForbidden || !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("expected delivery authorization, got %d %v", status, err)
	}
	status, _, err = service.Read(httptest.NewRecorder(), messageRouteRequest(http.MethodPost, "/conversations/"+cid, `{"message_id":"`+mid+`"}`, "user-1"))
	if status != http.StatusForbidden || !errors.Is(err, errs.ErrForbidden) {
		t.Fatalf("expected read authorization, got %d %v", status, err)
	}
	status, _, err = service.Unread(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if status != http.StatusUnauthorized || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected unread auth, got %d %v", status, err)
	}
}

func TestRealtimeMutationsAuthorizeAndPersist(t *testing.T) {
	cid := uuid.NewString()
	target := &model.Message{ID: uuid.NewString(), SenderID: "user-1", ReceiverID: "user-2", ConversationID: cid}
	repo := &fakeMessageRepo{target: target}
	service := NewMessageServiceImpl(repo, fakeFriendRepo{areFriends: true}, fakeBlockRepo{})
	service.SetConversationRepository(&fakeConversationRepo{})
	for _, event := range []string{"message.deleted", "message.delivered", "message.read"} {
		actor := "user-1"
		if event != "message.deleted" {
			actor = "user-2"
		}
		result, err := service.HandleRealtimeResult(context.Background(), actor, RealtimeEvent{Event: event, ConversationID: cid, MessageID: target.ID, ReceiverID: "user-2"})
		if err != nil || result == nil || result.Event != event {
			t.Fatalf("expected realtime %s, got %#v %v", event, result, err)
		}
	}
	reply, err := service.HandleRealtimeResult(context.Background(), "user-2", RealtimeEvent{Event: "message.replied", ConversationID: cid, MessageID: target.ID, ReceiverID: "user-1", Content: " reply "})
	if err != nil || reply == nil || reply.Content != "reply" || repo.created == nil {
		t.Fatalf("expected realtime reply, got %#v %v", reply, err)
	}
	_, err = service.HandleRealtimeResult(context.Background(), "user-2", RealtimeEvent{Event: "message.replied", ConversationID: cid, MessageID: target.ID, ReceiverID: "user-1"})
	if !errors.Is(err, errs.ErrValidation) {
		t.Fatalf("expected reply validation, got %v", err)
	}
}
