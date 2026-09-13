package service

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
)

type fakeFriendRequestRepo struct {
	acceptedRequestID string
	acceptedReceiver  string
	rejectedRequestID string
	rejectedReceiver  string
	created           *model.FriendRequest
	createErr         error
	pending           *model.FriendRequest
	pendingErr        error
	allErr            error
	all               model.FriendRequestsDTO
	cancelID          string
	cancelUser        string
	cancelErr         error
}

func (f *fakeFriendRequestRepo) CreateRequest(_ context.Context, req *model.FriendRequest) error {
	f.created = req
	return f.createErr
}

func (f *fakeFriendRequestRepo) GetPendingRequest(context.Context, string, string) (*model.FriendRequest, error) {
	return f.pending, f.pendingErr
}

func (f *fakeFriendRequestRepo) GetAllRequests(context.Context, string) (model.FriendRequestsDTO, error) {
	return f.all, f.allErr
}

func (f *fakeFriendRequestRepo) AcceptRequest(_ context.Context, requestID, receiverID string) error {
	f.acceptedRequestID = requestID
	f.acceptedReceiver = receiverID
	return nil
}

func (f *fakeFriendRequestRepo) RejectRequest(_ context.Context, requestID, receiverID string) error {
	f.rejectedRequestID = requestID
	f.rejectedReceiver = receiverID
	return nil
}

func (f *fakeFriendRequestRepo) CancelRequest(_ context.Context, requestID, userID string) error {
	f.cancelID, f.cancelUser = requestID, userID
	return f.cancelErr
}

func TestAcceptRequestUsesAuthenticatedReceiverID(t *testing.T) {
	repo := &fakeFriendRequestRepo{}
	service := FriendRequestServiceInit(repo, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/friend-requests/accept", bytes.NewBufferString(`{"request_id":"req-1","received_id":"attacker-controlled"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "receiver-1"))

	status, _, err := service.AcceptRequest(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if repo.acceptedRequestID != "req-1" {
		t.Fatalf("expected request id req-1, got %q", repo.acceptedRequestID)
	}
	if repo.acceptedReceiver != "receiver-1" {
		t.Fatalf("expected authenticated receiver id, got %q", repo.acceptedReceiver)
	}
}

func TestRejectRequestUsesAuthenticatedReceiverID(t *testing.T) {
	repo := &fakeFriendRequestRepo{}
	service := FriendRequestServiceInit(repo, nil, nil)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/friend-requests/reject", bytes.NewBufferString(`{"request_id":"req-1","receiver_id":"attacker-controlled"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "receiver-1"))

	status, _, err := service.RejectRequest(httptest.NewRecorder(), req)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if status != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, status)
	}
	if repo.rejectedRequestID != "req-1" {
		t.Fatalf("expected request id req-1, got %q", repo.rejectedRequestID)
	}
	if repo.rejectedReceiver != "receiver-1" {
		t.Fatalf("expected authenticated receiver id, got %q", repo.rejectedReceiver)
	}
}

func TestCreateRequestUsesAuthenticatedSenderAndRejectsSelf(t *testing.T) {
	repo := &fakeFriendRequestRepo{}
	service := FriendRequestServiceInit(repo, fakeFriendRepo{areFriends: false}, fakeBlockRepo{})
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"to":"user-2"}`))
	req = req.WithContext(context.WithValue(req.Context(), middleware.UserIDKey, "user-1"))
	status, _, err := service.CreateRequest(httptest.NewRecorder(), req)
	if status != http.StatusCreated || err != nil || repo.created == nil || repo.created.SenderID != "user-1" || repo.created.ReceiverID != "user-2" {
		t.Fatalf("expected request from authenticated user, got %d, %v, %#v", status, err, repo.created)
	}
	status, _, err = service.CreateRequest(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"to":"user-1"}`, "user-1"))
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrSelfAction) {
		t.Fatalf("expected self-request rejection, got %d, %v", status, err)
	}
}

func TestCreateRequestRejectsExistingPendingAndDependencyErrors(t *testing.T) {
	pending := &model.FriendRequest{ID: "request-1"}
	service := FriendRequestServiceInit(&fakeFriendRequestRepo{pending: pending}, fakeFriendRepo{areFriends: false}, fakeBlockRepo{})
	status, _, err := service.CreateRequest(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"to":"user-2"}`, "user-1"))
	if status != http.StatusConflict || !errors.Is(err, errs.ErrConflict) {
		t.Fatalf("expected pending conflict, got %d, %v", status, err)
	}
	service = FriendRequestServiceInit(&fakeFriendRequestRepo{pendingErr: errors.New("database unavailable")}, fakeFriendRepo{areFriends: false}, fakeBlockRepo{})
	status, _, err = service.CreateRequest(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"to":"user-2"}`, "user-1"))
	if status != http.StatusInternalServerError || err == nil {
		t.Fatalf("expected pending lookup failure, got %d, %v", status, err)
	}
}

func TestFriendRequestPendingAndAll(t *testing.T) {
	repo := &fakeFriendRequestRepo{pending: &model.FriendRequest{ID: "request-1"}, all: model.FriendRequestsDTO{}}
	service := FriendRequestServiceInit(repo, nil, nil)
	status, response, err := service.GetPendingRequest(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"sender_id":"sender-1"}`, "receiver-1"))
	if status != http.StatusOK || response == nil || err != nil {
		t.Fatalf("expected pending request, got %d %v", status, err)
	}
	status, response, err = service.GetAllRequests(httptest.NewRecorder(), authenticatedRequest(http.MethodGet, "", "receiver-1"))
	if status != http.StatusOK || response == nil || err != nil {
		t.Fatalf("expected all requests, got %d %v", status, err)
	}
	status, _, err = service.GetPendingRequest(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{}`, "receiver-1"))
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrBadRequest) {
		t.Fatalf("expected pending validation, got %d %v", status, err)
	}
	status, _, err = service.GetPendingRequest(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"sender_id":"sender-1"}`)))
	if status != http.StatusUnauthorized || !errors.Is(err, errs.ErrUnauthorized) {
		t.Fatalf("expected pending auth error, got %d %v", status, err)
	}
}

func TestFriendRequestCancelUsesAuthenticatedUserAndMapsErrors(t *testing.T) {
	repo := &fakeFriendRequestRepo{}
	service := FriendRequestServiceInit(repo, nil, nil)
	status, _, err := service.CancelRequest(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"request_id":"request-1"}`, "sender-1"))
	if status != http.StatusOK || err != nil || repo.cancelID != "request-1" || repo.cancelUser != "sender-1" {
		t.Fatalf("expected cancellation, got %d %v %#v", status, err, repo)
	}
	status, _, err = service.CancelRequest(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{}`, "sender-1"))
	if status != http.StatusBadRequest || !errors.Is(err, errs.ErrBadRequest) {
		t.Fatalf("expected cancel validation, got %d %v", status, err)
	}
	repo.cancelErr = errors.New("database unavailable")
	status, _, err = service.CancelRequest(httptest.NewRecorder(), authenticatedRequest(http.MethodPost, `{"request_id":"request-1"}`, "sender-1"))
	if status != http.StatusInternalServerError || err == nil {
		t.Fatalf("expected cancel dependency error, got %d %v", status, err)
	}
}
