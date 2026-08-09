package service

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ak-repo/go-chat-system/internal/domain/model"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
)

type fakeFriendRequestRepo struct {
	acceptedRequestID string
	acceptedReceiver  string
	rejectedRequestID string
	rejectedReceiver  string
}

func (f *fakeFriendRequestRepo) CreateRequest(context.Context, *model.FriendRequest) error {
	return nil
}

func (f *fakeFriendRequestRepo) GetPendingRequest(context.Context, string, string) (*model.FriendRequest, error) {
	return nil, nil
}

func (f *fakeFriendRequestRepo) GetAllRequests(context.Context, string) (model.FriendRequestsDTO, error) {
	return nil, nil
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

func (f *fakeFriendRequestRepo) CancelRequest(context.Context, string, string) error {
	return nil
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
