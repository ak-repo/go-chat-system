package wrapper

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
)

type WrappedFn func(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)

func HTTPResponseWrapper(fn WrappedFn) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		statusCode, obj, err := fn(w, r)

		// Default status codes
		if statusCode == 0 {
			if err != nil {
				statusCode = http.StatusInternalServerError
			} else {
				statusCode = http.StatusOK
			}
		}

		if err != nil {
			userMsg := getUserFriendlyMessage(err)
			utils.ErrorResponse(w, userMsg, nil, statusCode)
			return
		}

		writeJSON(w, statusCode, obj)
	}
}

func getUserFriendlyMessage(err error) string {
	if errors.Is(err, errs.ErrValidation) {
		return "validation failed"
	}
	if errors.Is(err, errs.ErrInvalidEmail) {
		return "invalid email format"
	}
	if errors.Is(err, errs.ErrWeakPassword) {
		return "password must be at least 8 characters"
	}
	if errors.Is(err, errs.ErrUnauthorized) {
		return "unauthorized"
	}
	if errors.Is(err, errs.ErrForbidden) {
		return "forbidden"
	}
	if errors.Is(err, errs.ErrNotFound) {
		return "resource not found"
	}
	if errors.Is(err, errs.ErrConflict) {
		return "conflict"
	}
	if errors.Is(err, errs.ErrBadRequest) {
		return "bad request"
	}
	// Friend-specific errors
	if errors.Is(err, errs.ErrSelfAction) {
		return "cannot send friend request to yourself"
	}
	if errors.Is(err, errs.ErrAlreadyFriends) {
		return "users are already friends"
	}
	if errors.Is(err, errs.ErrBlockedRelationship) {
		return "one of the users has blocked the other"
	}
	if errors.Is(err, errs.ErrRequestNotFound) {
		return "friend request not found"
	}
	return "an error occurred"
}

func writeJSON(w http.ResponseWriter, statusCode int, obj *utils.APIResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if obj == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
		return
	}

	_ = json.NewEncoder(w).Encode(obj)
}
