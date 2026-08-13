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
	// Config
	if err := config.Load(); err != nil {
		log.Fatal("failed to load config :", err)
	}

	// Logger
	logger.Init()
	defer logger.Sync()

	//database
	if err := database.ConnectDB(); err != nil {
		logger.L().Fatal("failed to connect to database", zap.Error(err))
	}
	if err := database.InitRedis(); err != nil {
		logger.L().Fatal("failed to connect to Redis", zap.Error(err))
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
			logger.L().Fatal("HTTP server error", zap.Error(err))
		}

	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.L().Info("shutting down system")

	// Stop WebSocket hub first
	if routes.GlobalHub != nil {
		routes.GlobalHub.Stop()
		logger.L().Info("WebSocket hub stopped")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		logger.L().Error("HTTP server shutdown error", zap.Error(err))
	}
	logger.L().Info("servers stopped")

}
