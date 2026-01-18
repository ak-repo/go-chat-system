package injector

import (
	"github.com/ak-repo/go-chat-system/database"
	"github.com/ak-repo/go-chat-system/repository"
	"github.com/ak-repo/go-chat-system/service"
)

// Container holds all dependencies  want to share across  app.
// This is "DI container" (manual injection).
type Container struct {

	// Repositories
	FriendRepo repository.FriendRepository
	UserRepo   repository.UserRepository

	// Service
	FriendService service.FriendService
	UserService   service.UserService
}

// Init creates and wires dependencies.
// This is the only place where  do NewXxx() calls.
func Init() *Container {
	// 0) any dependecies
	db := database.GetDB()

	// 1) Create repositories (DB layer)
	friendRepo := repository.NewFriendRepositoryImpl(db)
	userRepo := repository.NewUserRepositoryImpl(db)

	// 2) Create services (business layer)
	friendService := service.NewFriendServiceImpl(friendRepo)
	userService := service.NewUserServiceImpl(userRepo)

	return &Container{
		FriendRepo:    friendRepo,
		FriendService: friendService,
		UserRepo:      userRepo,
		UserService:   userService,
	}
}
