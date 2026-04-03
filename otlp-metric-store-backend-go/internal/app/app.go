package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"dash0.com/otlp-log-processor-backend/internal/config"
	grpcserver "dash0.com/otlp-log-processor-backend/internal/grpc"
	"dash0.com/otlp-log-processor-backend/internal/telemetry"

	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

type App struct {
	cfg               config.Config
	grpcServer        grpcserver.Server
	grpcMetricServer  colmetricspb.MetricsServiceServer
	telemetryProvider telemetry.Provider
}

func New(
	cfg config.Config,
	telemetryProvider telemetry.Provider,
	grpcServer grpcserver.Server,
	grpcMetricServer colmetricspb.MetricsServiceServer,
) *App {
	return &App{
		cfg:               cfg,
		telemetryProvider: telemetryProvider,
		grpcServer:        grpcServer,
		grpcMetricServer:  grpcMetricServer,
	}
}

func (a *App) Run(ctx context.Context) error {
	otelShutdown, err := a.telemetryProvider.Setup(ctx)
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

	colmetricspb.RegisterMetricsServiceServer(
		a.grpcServer,
		a.grpcMetricServer,
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
