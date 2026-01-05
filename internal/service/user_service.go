package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ak-repo/go-chat-system/internal/domain"
	"github.com/ak-repo/go-chat-system/pkg/utils"
	"github.com/google/uuid"
)

type userService struct {
	repo domain.UserRepo
}

func NewUserService(repo domain.UserRepo) domain.UserService {
	return &userService{repo: repo}
}

func (s *userService) Register(ctx context.Context, username, email, password string) error {

	if exists, err := s.repo.GetUser(ctx, "email", email); err == nil || exists != nil {
		log.Println("11 error", err)

		return fmt.Errorf("failed to register :%w ", err)
	}

	hash, err := utils.HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password :%w", err)
	}
	user := &domain.User{
		ID:           uuid.New().String(),
		Username:     username,
		Email:        email,
		PasswordHash: hash,
		Role:         "user",
		CreatedAt:    time.Now().UTC(),
		UpdatedAt:    time.Now().UTC(),
	}
	if err := s.repo.Create(ctx, user); err != nil {
		log.Println("db error",err)
		return fmt.Errorf("failed to register :%w ", err)
	}
	return nil
}

func (s *userService) Login(ctx context.Context, email, password string) (*domain.User, error) {

	user, err := s.repo.GetUser(ctx, "email", email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user :%w", err)
	}

	if ok := utils.ComparePassword(user.PasswordHash, password); !ok {
		return nil, fmt.Errorf("password mismatching")
	}

	return user, nil
}
