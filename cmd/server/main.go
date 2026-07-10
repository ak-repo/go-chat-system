package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ak-repo/go-chat-system/internal/platform/config"
	"github.com/ak-repo/go-chat-system/internal/platform/database"
	"github.com/ak-repo/go-chat-system/internal/transport/routes"

	"github.com/ak-repo/go-chat-system/internal/shared/logger"

	"go.uber.org/zap"
)

func main() {
	// Logger
	logger.Init()

	// Config
	if err := config.Load(); err != nil {
		log.Fatal("failed to load config :", err)
	}

	//database
	if err := database.ConnectDB(); err != nil {
		log.Fatal("failed to connect to database:", err)
	}
	if err := database.InitRedis(); err != nil {
		log.Fatal("failed to connect to Redis:", err)
	}

	router := routes.Router()

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", config.Config.Server.Port),
		Handler:           router,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// start server
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP serever error", zap.Error(err))
		}

	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down system....")

	// Stop WebSocket hub first
	if routes.GlobalHub != nil {
		routes.GlobalHub.Stop()
		log.Println("WebSocket hub stopped")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Println("HTTP server shutdown error", zap.Error(err))
	}
	log.Println("Servers stopped")

}
