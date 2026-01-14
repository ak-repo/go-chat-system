package handler

import (
	"encoding/json"
	"net/http"

	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/ak-repo/go-chat-system/transport/middleware"
)

func Home(w http.ResponseWriter, r *http.Request) {
	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok {
		utils.ErrorResponse(w, "unauthorised access", nil, http.StatusUnauthorized, r)
		return
	}

	responseData := map[string]interface{}{
		"message": "welcome home",
		"user_id": userID,
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(utils.SuccessResponse(responseData))

}
