package injector

import (
	"context"
	"encoding/json"

	"github.com/ak-repo/go-chat-system/database"
	"github.com/ak-repo/go-chat-system/model"
	"github.com/ak-repo/go-chat-system/repository"
	"github.com/ak-repo/go-chat-system/service"
	"github.com/ak-repo/go-chat-system/transport/websocket"
)

// Container holds all dependencies to share across the app (DI container).
type Container struct {
	UserRepo          repository.UserRepository
	FriendRepo        repository.FriendRepository
	FriendRequestRepo repository.FriendRequestRepository
	BlockRepo         repository.BlockRepository
	ChatRepo          repository.ChatRepository
	MessageRepo       repository.MessageRepository

	UserService          service.UserService
	FriendService        service.FriendService
	FriendRequestService service.FriendRequestService
	BlockService         service.BlockService
	ChatService          service.ChatService
	MessageService       service.MessageService

	Hub *websocket.Hub
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
	chatRepo := repository.NewChatRepositoryImpl(db)
	messageRepo := repository.NewMessageRepositoryImpl(db)

	// 2) Create services (business layer)
	friendService := service.NewFriendServiceImpl(friendRepo)
	userService := service.NewUserServiceImpl(userRepo)
	blockService := service.BlockServiceInit(blockRepo)
	friendReqService := service.FriendRequestServiceInit(friendReqRepo, friendRepo, blockRepo)
	chatService := service.NewChatServiceImpl(chatRepo)
	messageService := service.NewMessageServiceImpl(messageRepo, chatRepo)

	hub := websocket.NewHub()
	hub.PersistDM = func(senderID, receiverID, content string) (*websocket.WSMessage, error) {
		ctx := context.Background()
		chat, err := chatRepo.GetOrCreateDMChat(ctx, senderID, receiverID)
		if err != nil {
			return nil, err
		}
		msg := &model.Message{ChatID: chat.ID, SenderID: senderID, Content: content}
		if err := messageRepo.Create(ctx, msg); err != nil {
			return nil, err
		}
		data, _ := json.Marshal(map[string]any{
			"id":         msg.ID,
			"chat_id":    chat.ID,
			"sender_id":  senderID,
			"content":    content,
			"created_at": msg.CreatedAt,
		})
		return &websocket.WSMessage{
			Event:        "chat.message",
			SenderID:     senderID,
			ReceiverID:   receiverID,
			ReceiverType: websocket.ReceiverUser,
			Data:         data,
		}, nil
	}
	go hub.Run()

	return &Container{
		FriendRepo:           friendRepo,
		FriendService:        friendService,
		UserRepo:             userRepo,
		UserService:          userService,
		FriendRequestRepo:    friendReqRepo,
		FriendRequestService: friendReqService,
		BlockRepo:            blockRepo,
		BlockService:         blockService,
		ChatRepo:             chatRepo,
		MessageRepo:          messageRepo,
		ChatService:          chatService,
		MessageService:      messageService,
		Hub:                  hub,
	}
}
