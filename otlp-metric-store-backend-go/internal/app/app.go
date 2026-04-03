package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"

	"dash0.com/otlp-log-processor-backend/internal/config"
	grpcserver "dash0.com/otlp-log-processor-backend/internal/grpc"
	"dash0.com/otlp-log-processor-backend/internal/telemetry"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type App struct {
	cfg        config.Config
	grpcServer *grpc.Server
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

	a.grpcServer = grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.MaxRecvMsgSize(a.cfg.GRPC.MaxReceiveMessageSize),
		grpc.Creds(insecure.NewCredentials()),
	)

	colmetricspb.RegisterMetricsServiceServer(
		a.grpcServer,
		grpcserver.NewServer(a.cfg.GRPC.ListenAddr, nil),
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
