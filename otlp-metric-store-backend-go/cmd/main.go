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
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

func main() {
	// Read configuration from YAML file (default: config.yml). Use -config to override.
	var configPath string
	flag.StringVar(&configPath, "config", "../config.yml", "Path to YAML config file")
	flag.Parse()
	// Load configuration from the specified file. The application will panic if the config cannot be loaded.
	cfg := config.MustLoadFile(configPath)
	// Set up signal handling to gracefully shut down the application on SIGINT or SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	// Add telemetry provider to the application.
	// The provider will be responsible for setting up the OpenTelemetry SDK.
	var res = resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(cfg.Telemetry.ServiceName),
		semconv.ServiceNamespaceKey.String(cfg.Telemetry.ServiceNamespace),
		semconv.ServiceVersionKey.String(cfg.Telemetry.ServiceVersion),
	)
	telemetryProvider := telemetry.NewOTELProvider(res)
	// grpc server
	grpcSrv := grpc.NewGRPCServer(
		grpc.ServerOptions{
			MaxReceiveMessageSize: cfg.GRPC.MaxReceiveMessageSize,
		})
	// Create store and initialize tables if ClickHouse is enabled and address is provided.
	// If ClickHouse is disabled, the store will log insert operations instead of writing to a database.
	// I chose to initialize the store and pass it to the application regardless of whether ClickHouse is enabled or not,
	// so that the application can use the same interface for storing metrics without needing to know about the database configuration.
	// If ClickHouse is disabled, NewClickhouse will return a nil connection and a specific error which we check for.
	// But beside the error whithout the database connection, we can still create a Metric store that will log operations instead of writing to a database.
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
