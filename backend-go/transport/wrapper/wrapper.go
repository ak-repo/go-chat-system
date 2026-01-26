package wrapper

import (
	"encoding/json"
	"net/http"

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
			utils.ErrorResponse(w, "error occurred", err, statusCode)
			return
		}

		writeJSON(w, statusCode, obj)
	}
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
