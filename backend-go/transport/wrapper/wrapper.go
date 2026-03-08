package wrapper

import (
	"encoding/json"
	"net/http"

	"github.com/ak-repo/go-chat-system/pkg/logger"
	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/go-chi/chi/middleware"
	"go.uber.org/zap"
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
			requestID := middleware.GetReqID(r.Context())
			logger.Logger.Error("handler error",
				zap.String("request_id", requestID),
				zap.String("path", r.URL.Path),
				zap.Int("status", statusCode),
				zap.Error(err),
			)
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
