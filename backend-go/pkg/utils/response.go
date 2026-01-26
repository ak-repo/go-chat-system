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

func ErrorResponse(w http.ResponseWriter, message string, err error, statusCode int) {

	response := APIResponse{
		Status:  "error",
		Message: message,
	}
	if err != nil {
		response.Error = err.Error()
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(&response)

}
