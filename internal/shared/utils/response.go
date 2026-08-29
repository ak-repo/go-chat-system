package utils

import (
	"encoding/json"
	"errors"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"net/http"
)

type APIResponse struct {
	Status  string `json:"status"`
	Data    any    `json:"data,omitempty"`
	Error   any    `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Code    string `json:"code,omitempty"`
}

func SuccessResponse[T any](data T) *APIResponse {

	return &APIResponse{
		Status: "ok",
		Data:   data,
	}
}

func ErrorResponse(w http.ResponseWriter, message string, err error, statusCode int) {

	response := APIResponse{
		Status:  "error",
		Message: message,
		Code:    errorCode(err),
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(&response)

}

func errorCode(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, errs.ErrValidation):
		return "validation_failed"
	case errors.Is(err, errs.ErrUnauthorized):
		return "unauthorized"
	case errors.Is(err, errs.ErrForbidden):
		return "forbidden"
	case errors.Is(err, errs.ErrNotFound):
		return "not_found"
	case errors.Is(err, errs.ErrConflict):
		return "conflict"
	case errors.Is(err, errs.ErrInactiveUser):
		return "inactive_user"
	case errors.Is(err, errs.ErrTokenReused):
		return "token_reused"
	case errors.Is(err, errs.ErrNotMember):
		return "not_member"
	default:
		return "internal_error"
	}
}
