package wrapper

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ak-repo/go-chat-system/pkg/errs"
	"github.com/ak-repo/go-chat-system/pkg/utils"
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
