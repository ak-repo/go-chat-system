package utils

import (
	"encoding/json"
	"net/http"

	"github.com/ak-repo/go-chat-system/model"
)

func SuccessResponse[T any](data T) model.ApiResponse {
	return model.ApiResponse{
		Status: "ok",
		Data:   data,
	}

}

func ErrorResponse(w http.ResponseWriter, message string, err error, statusCode int, r *http.Request) {

	response := map[string]interface{}{
		"status":  "error",
		"message": message,
	}
	if err != nil {
		response["error"] = err.Error()
	}

	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)

}
