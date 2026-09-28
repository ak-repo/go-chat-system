package injector

import (
	"strings"

	"github.com/ak-repo/go-chat-system/internal/platform/config"
	"github.com/ak-repo/go-chat-system/internal/platform/database"
	"github.com/ak-repo/go-chat-system/internal/repository"
	"github.com/ak-repo/go-chat-system/internal/service"
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
	ConversationRepo  repository.ConversationRepository
	NotificationRepo  repository.NotificationRepository

	// Service
	UserService              service.UserService
	FriendService            service.FriendService
	FriendRequestService     service.FriendRequestService
	BlockService             service.BlockService
	MessageService           service.MessageService
	MessageHTTPService       service.MessageHTTPService
	ConversationService      service.ConversationService
	GroupConversationService service.GroupConversationService
	AccountTokenService      *service.AccountTokenService
	PresenceService          service.PresenceService
	NotificationService      *service.NotificationService
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
	conversationRepo := repository.NewConversationRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)

	// 2) Create services (business layer)
	friendService := service.NewFriendServiceImpl(friendRepo)
	userService := service.NewUserServiceImpl(userRepo)
	blockService := service.BlockServiceInit(blockRepo)
	friendReqService := service.FriendRequestServiceInit(friendReqRepo, friendRepo, blockRepo)
	messageService := service.NewMessageServiceImpl(messageRepo, friendRepo, blockRepo)
	conversationService := service.NewConversationService(conversationRepo, friendRepo, blockRepo)
	groupConversationService := service.NewGroupConversationService(conversationRepo, conversationRepo)
	presenceService := service.NewPresenceService(conversationRepo)
	notificationService := service.NewNotificationService(notificationRepo)
	messageService.SetConversationRepository(conversationRepo)
	messageService.SetNotificationPublisher(notificationService.PublishCreated)
	delivery := accountDelivery(config.Config.App.Environment, config.Config.Email)
	accountTokenService := service.NewAccountTokenService(userRepo, delivery)
	userService.SetVerificationSender(accountTokenService)

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
		MessageHTTPService:   messageService,
		ConversationRepo:     conversationRepo, ConversationService: conversationService,
		GroupConversationService: groupConversationService,
		AccountTokenService:      accountTokenService,
		PresenceService:          presenceService,
		NotificationRepo:         notificationRepo,
		NotificationService:      notificationService,
	}
}

func accountDelivery(environment string, email config.EmailConfig) service.Delivery {
	if email.SMTPHost != "" {
		return &service.SMTPDelivery{Host: email.SMTPHost, Port: email.SMTPPort, Username: email.Username, Password: email.Password, From: email.From, AppURL: email.AppURL}
	}
	if strings.EqualFold(strings.TrimSpace(environment), "development") {
		appURL := email.AppURL
		if appURL == "" {
			appURL = "http://localhost:5173"
		}
		return &service.DevelopmentDelivery{AppURL: appURL}
	}
	// A missing SMTP setup must never silently select a token-exposing development adapter.
	return &service.SMTPDelivery{}
}
