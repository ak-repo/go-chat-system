package service

import (
	"encoding/json"
	"net/http"

	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/shared/errs"
	"github.com/ak-repo/go-chat-system/internal/shared/utils"
	"github.com/ak-repo/go-chat-system/internal/transport/middleware"
)

type BlockService interface {
	UnblockUser(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
	BlockUser(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error)
}

type BlockServiceImpl struct {
	repo repository.BlockRepository
}

func BlockServiceInit(repo repository.BlockRepository) *BlockServiceImpl {
	return &BlockServiceImpl{repo: repo}
}

// POST
func (s *BlockServiceImpl) BlockUser(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {

	var body struct {
		Target string `json:"target"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, errs.Wrap("service.BlockService.BlockUser", err)
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}
	if body.Target == "" {
		return http.StatusBadRequest, nil, errs.ErrBadRequest
	}

	if userID == body.Target {
		return http.StatusConflict, nil, errs.ErrSelfAction
	}

	if err := s.repo.BlockUser(r.Context(), userID, body.Target); err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.BlockService.BlockUser", err)
	}

	return http.StatusOK, nil, nil

}

func (s *BlockServiceImpl) UnblockUser(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	var body struct {
		Target string `json:"target"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, errs.Wrap("service.BlockService.UnblockUser", err)
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errs.ErrUnauthorized
	}
	if body.Target == "" {
		return http.StatusBadRequest, nil, errs.ErrBadRequest
	}

	if err := s.repo.UnblockUser(r.Context(), userID, body.Target); err != nil {
		return http.StatusInternalServerError, nil, errs.Wrap("service.BlockService.UnblockUser", err)
	}
	return http.StatusOK, nil, nil
}
