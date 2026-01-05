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
	http_pkg "github.com/ak-repo/go-chat-system/internal/transport/http"

	"github.com/ak-repo/go-chat-system/pkg/db"
	"github.com/ak-repo/go-chat-system/pkg/logger"

	"go.uber.org/zap"
)

func main() {

	// Config
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("failed to load config :", err)
	}

	// Logger
	logger.Init(cfg.Logging.Level, cfg.Logging.Format)
	log := logger.New()

	//database
	db, err := db.NewPostgresDB(context.Background(), cfg)
	if err != nil {
		log.Fatal("failed to connect db", zap.Error(err))
	}
	defer db.Close()

	handler := http_pkg.NewRoutes(db, cfg)

	httpServer := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           handler,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// start server
	go func() {
		log.Info("http server started at: ", zap.Any("port", cfg.Server.Port))
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP serever error", zap.Error(err))
		}

	}()

	// Graceful shutdown
	quite := make(chan os.Signal, 1)
	signal.Notify(quite, syscall.SIGINT, syscall.SIGTERM)
	<-quite
	log.Info("Shutting down system....")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Error("HTTP server shutdown error", zap.Error(err))
	}
	log.Info("Servers stopped")

}
