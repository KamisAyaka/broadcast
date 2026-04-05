package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"broadcast/internal/api"
	"broadcast/internal/config"
	"broadcast/internal/logger"

	"github.com/joho/godotenv"
	"go.uber.org/zap"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config failed: %v\n", err)
		os.Exit(1)
	}

	log, err := logger.New(cfg.LogLevel)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init logger failed: %v\n", err)
		os.Exit(1)
	}
	defer log.Sync()

	r := api.NewRouter(cfg)
	server := &http.Server{
		Addr:              ":" + cfg.ServerPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info("server started", zap.String("addr", server.Addr), zap.String("env", cfg.AppEnv))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("server listen failed", zap.Error(err))
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("server shutdown failed", zap.Error(err))
		os.Exit(1)
	}

	log.Info("server exited")
}
