package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os/signal"
	"syscall"

	"dash0.com/otlp-log-processor-backend/internal/app"
	"dash0.com/otlp-log-processor-backend/internal/clickhouse"
	"dash0.com/otlp-log-processor-backend/internal/config"
	"dash0.com/otlp-log-processor-backend/internal/grpc"
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
	var metricStore clickhouse.MetricStore
	var err error
	if cfg.ClickHouse.Enabled && cfg.ClickHouse.Addr != "" {
		metricStore, err = clickhouse.NewMetric(
			context.Background(),
			cfg.ClickHouse.Addr,
			cfg.ClickHouse.Database,
			cfg.ClickHouse.Username,
			cfg.ClickHouse.Password,
		)
		if err != nil {
			log.Fatalf("failed to initialize clickhouse metric store: %v", err)
		}
		defer func() {
			if err := metricStore.Close(); err != nil {
				log.Printf("failed to close clickhouse metric store: %v", err)
			}
		}()
		slog.Info("ClickHouse storage enabled", slog.String("addr", cfg.ClickHouse.Addr))
	} else {
		slog.Info("ClickHouse storage disabled or address empty; running without persistence")
	}
	//TODO Actiavte it later.
	/*
		if err := metricStore.CreateTables(ctx); err != nil {
			log.Fatalf("failed to create clickhouse tables: %v", err)
		}
	*/
	// Initialize and run the application
	metricServer := grpc.NewMetricServer(cfg.GRPC.ListenAddr, metricStore)
	application := app.New(cfg, telemetryProvider, grpcSrv, metricServer)

	if err := application.Run(ctx); err != nil {
		log.Fatalf("run application: %v", err)
	}
}
