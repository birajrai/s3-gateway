package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"s3server/config"
	"s3server/handlers"
	"s3server/logger"
	"s3server/storage"
)

func main() {
	logger.Init()

	cfg, err := config.Load()
	if err != nil {
		logger.Error("Failed to load config: %v", err)
		os.Exit(1)
	}

	logger.Info("=== S3 Gateway Starting ===")
	logger.Info("Config loaded - DEBUG: %v", cfg.Debug)

	store, err := storage.NewFileStorage(cfg)
	if err != nil {
		logger.Error("Failed to initialize storage: %v", err)
		os.Exit(1)
	}

	router := handlers.NewRouter(store, cfg)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info("Server starting on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Server error: %v", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logger.Error("Server forced to shutdown: %v", err)
	}

	logger.Info("Server stopped")
}
