package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"dash0.com/otlp-log-processor-backend/internal/clickhouse"
	"dash0.com/otlp-log-processor-backend/internal/config"
	grpcserver "dash0.com/otlp-log-processor-backend/internal/grpc"
	"dash0.com/otlp-log-processor-backend/internal/telemetry"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

type App struct {
	cfg        config.Config
	grpcServer grpcserver.Server
}

func New(cfg config.Config) *App {
	return &App{
		cfg: cfg,
	}
}

func (a *App) Run(ctx context.Context) error {
	otelShutdown, err := telemetry.SetupOTelSDK(ctx)
	if err != nil {
		return fmt.Errorf("setup otel sdk: %w", err)
	}
	defer func() {
		if shutdownErr := otelShutdown(context.Background()); shutdownErr != nil {
			slog.Error("failed to shutdown otel", slog.Any("error", shutdownErr))
		}
	}()

	listener, err := net.Listen("tcp", a.cfg.GRPC.ListenAddr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", a.cfg.GRPC.ListenAddr, err)
	}

	a.grpcServer = grpcserver.NewGRPCServer(grpcserver.ServerOptions{
		MaxReceiveMessageSize: a.cfg.GRPC.MaxReceiveMessageSize,
	})

	// Optionally initialize ClickHouse storage when enabled via config.
	var store clickhouse.Metric
	if a.cfg.ClickHouse.Enabled && a.cfg.ClickHouse.Addr != "" {
		s, err := clickhouse.NewMetric(
			ctx,
			a.cfg.ClickHouse.Addr,
			a.cfg.ClickHouse.Database,
			a.cfg.ClickHouse.Username,
			a.cfg.ClickHouse.Password,
		)
		if err != nil {
			return fmt.Errorf("new clickhouse metric: %w", err)
		}
		if err := s.CreateTables(ctx); err != nil {
			return fmt.Errorf("create clickhouse tables: %w", err)
		}
		store = s
		defer func() {
			if err := s.Close(); err != nil {
				slog.Warn("closing clickhouse", slog.Any("error", err))
			}
		}()
		slog.Info("ClickHouse storage enabled", slog.String("addr", a.cfg.ClickHouse.Addr))
	} else {
		slog.Info("ClickHouse storage disabled or address empty; running without persistence")
	}

	colmetricspb.RegisterMetricsServiceServer(
		a.grpcServer,
		grpcserver.NewServer(a.cfg.GRPC.ListenAddr, store),
	)

	slog.Info("starting gRPC server", slog.String("listen_addr", a.cfg.GRPC.ListenAddr))

	go func() {
		<-ctx.Done()
		slog.Info("shutting down gRPC server")
		a.grpcServer.GracefulStop()
	}()

	if err := a.grpcServer.Serve(listener); err != nil {
		return fmt.Errorf("serve gRPC: %w", err)
	}

	return nil
}
