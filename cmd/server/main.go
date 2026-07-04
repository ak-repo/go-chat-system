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

	"github.com/ak-repo/go-chat-system/config"
	"github.com/ak-repo/go-chat-system/database"
	"github.com/ak-repo/go-chat-system/transport/routes"

	"github.com/ak-repo/go-chat-system/pkg/logger"

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
	database.ConnectDB()
	database.InitRedis()

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
	quite := make(chan os.Signal, 1)
	signal.Notify(quite, syscall.SIGINT, syscall.SIGTERM)
	<-quite
	log.Println("Shutting down system....")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Println("HTTP server shutdown error", zap.Error(err))
	}
	log.Println("Servers stopped")

}
