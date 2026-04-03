package grpc

import (
	"log/slog"
	"net"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const name = "dash0.com/otlp-log-processor-backend"

var (
	meter                  = otel.Meter(name)
	logger                 = otelslog.NewLogger(name)
	metricsReceivedCounter metric.Int64Counter
)

func init() {
	var err error
	metricsReceivedCounter, err = meter.Int64Counter("com.dash0.homeexercise.metrics.received",
		metric.WithDescription("The number of metrics received by otlp-metrics-processor-backend"),
		metric.WithUnit("{metric}"))
	if err != nil {
		panic(err)
	}
	// Set otelslog as default slog logger for this package usages.
	slog.SetDefault(logger)
}

// Server is the gRPC server interface exposed for the application layer.
// It embeds grpc.ServiceRegistrar to remain compatible with protobuf Register* helpers.
type Server interface {
	grpc.ServiceRegistrar
	Serve(lis net.Listener) error
	GracefulStop()
}

// ServerOptions configures NewGRPCServer.
type ServerOptions struct {
	MaxReceiveMessageSize int
}

// NewGRPCServer constructs a *grpc.Server with OpenTelemetry instrumentation
// and insecure transport credentials for local/edge scenarios.
func NewGRPCServer(opts ServerOptions) Server {
	return grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
		grpc.MaxRecvMsgSize(opts.MaxReceiveMessageSize),
		grpc.Creds(insecure.NewCredentials()),
	)
}
