package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os/signal"
	"syscall"

	"dash0.com/otlp-log-processor-backend/internal/app"
	"dash0.com/otlp-log-processor-backend/internal/config"
	"dash0.com/otlp-log-processor-backend/internal/grpc"
	"dash0.com/otlp-log-processor-backend/internal/store"
	"dash0.com/otlp-log-processor-backend/internal/telemetry"
)

func main() {
	// Read configuration from YAML file (default: config.yml). Use -config to override.
	var configPath string
	flag.StringVar(&configPath, "config", "../config.yml", "Path to YAML config file")
	flag.Parse()

	cfg := config.MustLoadFile(configPath)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Add telemetry provider to the application.
	// The provider will be responsible for setting up the OpenTelemetry SDK.
	telemetryProvider := telemetry.NewOTELProvider()
	// grpc server
	grpcSrv := grpc.NewGRPCServer(
		grpc.ServerOptions{
			MaxReceiveMessageSize: cfg.GRPC.MaxReceiveMessageSize,
		})

	// Create store and initialize tables if ClickHouse is enabled and address is provided.
	conn, err := store.NewClickhouse(ctx, store.ClickHouseOptions{
		IsEnabled: cfg.ClickHouse.Enabled,
		Addr:      cfg.ClickHouse.Addr,
		Database:  cfg.ClickHouse.Database,
		Username:  cfg.ClickHouse.Username,
		Password:  cfg.ClickHouse.Password,
	})
	if err != nil {
		if errors.Is(err, store.ErrClickHouseDisabled) {
			log.Printf("clickhouse storage is disabled, proceeding without database connection")
		} else {
			log.Fatalf("error initializing clickhouse service: %v", err)
		}
	}
	// initialize the metric store with the database connection (which may be nil if ClickHouse is disabled)
	// if database is disabled, the store will log insert operations instead of writing to a database
	metricStore := store.NewMetric(conn)
	// Initialize and run the application
	metricServer := grpc.NewMetricServer(metricStore)
	// initialize the application with the configured components. The application will manage their lifecycles.
	application := app.New(cfg, telemetryProvider, grpcSrv, metricServer)
	// Run the application. This will block until the application is stopped (e.g. via SIGINT/SIGTERM).
	if err := application.Run(ctx); err != nil {
		log.Fatalf("run application: %v", err)
	}
}
