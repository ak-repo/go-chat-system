package handler

import (
	"log"
	"net/http"

	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/ak-repo/go-chat-system/service"
)

type UserHandler struct {
	userService service.UserService
}

func NewUserHandler(userService service.UserService) *UserHandler {
	return &UserHandler{
		userService: userService,
	}
}

func (h *UserHandler) SearchUser(w http.ResponseWriter, r *http.Request) {

	filter := r.URL.Query().Get("filter")

	respObj, err := h.userService.SearchUser(r.Context(), filter)
	if err != nil {
		utils.ErrorResponse(w, "failed to get users", err, http.StatusInternalServerError, r)
		return
	}

	responseData := map[string]any{
		"users": respObj,
	}
	log.Println("users: ", responseData)

	utils.SuccessResponse(responseData)

}
