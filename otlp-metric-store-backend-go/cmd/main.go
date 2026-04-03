package main

import (
	"context"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"dash0.com/otlp-log-processor-backend/internal/app"
	"dash0.com/otlp-log-processor-backend/internal/config"
)

func main() {
	// Read configuration from YAML file (default: config.yml). Use -config to override.
	var configPath string
	flag.StringVar(&configPath, "config", "../config.yml", "Path to YAML config file")
	flag.Parse()

	cfg := config.MustLoadFile(configPath)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	application := app.New(cfg)

	if err := application.Run(ctx); err != nil {
		log.Fatalf("run application: %v", err)
	}
}
