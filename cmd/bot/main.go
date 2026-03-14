package main

import (
	"context"
	"log"
	"os/signal"
	"streamingbot/internal/platform/config"
	"streamingbot/internal/platform/logger"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	lg := logger.New(cfg.LogLevel)
	lg.Info("streamingbot bootstrap started")
	<-ctx.Done()
	lg.Info("streamingbot shutdown complete")
}
