package service

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/ak-repo/go-chat-system/repository"
	"github.com/ak-repo/go-chat-system/transport/middleware"
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
		return http.StatusBadRequest, nil, err
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errors.New("user id missing ")
	}

	if userID == body.Target {
		return http.StatusConflict, nil, errors.New("cannot block yourself")
	}

	if err := s.repo.BlockUser(r.Context(), userID, body.Target); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, nil, nil

}

func (s *BlockServiceImpl) UnblockUser(w http.ResponseWriter, r *http.Request) (int, *utils.APIResponse, error) {
	var body struct {
		Target string `json:"target"`
	}

	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return http.StatusBadRequest, nil, err
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(string)
	if !ok || userID == "" {
		return http.StatusUnauthorized, nil, errors.New("user id missing")
	}

	if err := s.repo.UnblockUser(r.Context(), userID, body.Target); err != nil {
		return http.StatusInternalServerError, nil, err
	}
	return http.StatusOK, nil, nil
}
