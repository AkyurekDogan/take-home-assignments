package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"dash0.com/otlp-log-processor-backend/internal/app"
	"dash0.com/otlp-log-processor-backend/internal/config"
)

func main() {
	cfg := config.MustLoad()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application, err := app.New(ctx, cfg)
	if err != nil {
		log.Fatalf("create application: %v", err)
	}

	if err := application.Run(ctx); err != nil {
		log.Fatalf("run application: %v", err)
	}
}
