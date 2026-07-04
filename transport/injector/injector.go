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
	UserRepo          repository.UserRepository
	FriendRepo        repository.FriendRepository
	FriendRequestRepo repository.FriendRequestRepository
	BlockRepo         repository.BlockRepository
	MessageRepo       repository.MessageRepository

	// Service
	UserService          service.UserService
	FriendService        service.FriendService
	FriendRequestService service.FriendRequestService
	BlockService         service.BlockService
	MessageService       service.MessageService
}

// Init creates and wires dependencies.
// This is the only place where  do NewXxx() calls.
func Init() *Container {
	// 0) any dependecies
	db := database.GetDB()

	// 1) Create repositories (DB layer)
	friendRepo := repository.NewFriendRepositoryImpl(db)
	userRepo := repository.NewUserRepositoryImpl(db)
	blockRepo := repository.BlockRepositoryInit(db)
	friendReqRepo := repository.FriendRequestRepositoryInit(db)
	messageRepo := repository.NewMessageRepositoryImpl(db)

	// 2) Create services (business layer)
	friendService := service.NewFriendServiceImpl(friendRepo)
	userService := service.NewUserServiceImpl(userRepo)
	blockService := service.BlockServiceInit(blockRepo)
	friendReqService := service.FriendRequestServiceInit(friendReqRepo, friendRepo, blockRepo)
	messageService := service.NewMessageServiceImpl(messageRepo)

	return &Container{
		FriendRepo:           friendRepo,
		FriendService:        friendService,
		UserRepo:             userRepo,
		UserService:          userService,
		FriendRequestRepo:    friendReqRepo,
		FriendRequestService: friendReqService,
		BlockRepo:            blockRepo,
		BlockService:         blockService,
		MessageRepo:          messageRepo,
		MessageService:       messageService,
	}
}
