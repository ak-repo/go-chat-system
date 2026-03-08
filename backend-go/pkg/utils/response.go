package utils

import (
	"encoding/json"
	"net/http"
)

type APIResponse struct {
	Status  string `json:"status"`
	Data    any    `json:"data,omitempty"`
	Error   any    `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

func SuccessResponse[T any](data T) *APIResponse {

	return &APIResponse{
		Status: "ok",
		Data:   data,
	}
}

// ErrorResponse writes a JSON error. Never exposes err.Error() to the client to avoid leaking internal details.
// Use message (and optional clientSafeCode) for client-facing feedback; log err server-side.
func ErrorResponse(w http.ResponseWriter, message string, err error, statusCode int) {
	response := APIResponse{
		Status:  "error",
		Message: message,
	}
	// Do not send raw err to client (security and contract stability)
	if statusCode >= 500 {
		response.Message = "internal error"
	}

	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(&response)
}
