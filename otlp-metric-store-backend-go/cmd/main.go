package main

import (
	"context"
	"flag"
	"log"
	"log/slog"
	"os/signal"
	"syscall"
	"time"

	"dash0.com/otlp-log-processor-backend/internal/app"
	"dash0.com/otlp-log-processor-backend/internal/config"
	"dash0.com/otlp-log-processor-backend/internal/grpc"
	"dash0.com/otlp-log-processor-backend/internal/store"
	"dash0.com/otlp-log-processor-backend/internal/telemetry"

	"github.com/ClickHouse/clickhouse-go/v2"
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
	var metricStore store.Metric
	if cfg.ClickHouse.Enabled && cfg.ClickHouse.Addr != "" {
		// Create ClickHouse connection explicitly, then wrap it with our MetricStore
		conn, err := clickhouse.Open(&clickhouse.Options{
			Addr: []string{cfg.ClickHouse.Addr},
			Auth: clickhouse.Auth{
				Database: cfg.ClickHouse.Database,
				Username: cfg.ClickHouse.Username,
				Password: cfg.ClickHouse.Password,
			},
			Settings: clickhouse.Settings{
				"max_execution_time": 60,
			},
			DialTimeout: 5 * time.Second,
		})
		if err != nil {
			log.Fatalf("opening clickhouse connection: %v", err)
		}
		if err := conn.Ping(ctx); err != nil {
			_ = conn.Close()
			log.Fatalf("pinging clickhouse: %v", err)
		}
		metricStore = store.NewMetric(conn)

		defer func() {
			if err := metricStore.Close(); err != nil {
				log.Printf("failed to close clickhouse metric store: %v", err)
			}
		}()
		slog.Info("ClickHouse storage enabled", slog.String("addr", cfg.ClickHouse.Addr))
		//Create the required tables.
		if err := metricStore.CreateTables(ctx); err != nil {
			log.Fatalf("failed to create clickhouse tables: %v", err)
		}
		slog.Info("Required tables are created", slog.String("addr", cfg.ClickHouse.Addr))
	} else {
		slog.Info("ClickHouse storage disabled or address empty; running without persistence")
	}
	// Initialize and run the application
	metricServer := grpc.NewMetricServer(metricStore)
	application := app.New(cfg, telemetryProvider, grpcSrv, metricServer)

	if err := application.Run(ctx); err != nil {
		log.Fatalf("run application: %v", err)
	}
}
