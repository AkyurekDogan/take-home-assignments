package grpc

import (
	"context"
	"log/slog"

	"dash0.com/otlp-log-processor-backend/internal/clickhouse"
	"dash0.com/otlp-log-processor-backend/internal/metric"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

type dash0MetricsServiceServer struct {
	store clickhouse.Metric

	colmetricspb.UnimplementedMetricsServiceServer
}

// NewServer constructs a MetricsServiceServer.
// The first parameter (addr) is accepted for backward compatibility with call sites
// but is not used by the implementation.
func NewServer(_ string, store clickhouse.Metric) colmetricspb.MetricsServiceServer {
	return &dash0MetricsServiceServer{store: store}
}

func (m *dash0MetricsServiceServer) Export(ctx context.Context, request *colmetricspb.ExportMetricsServiceRequest) (*colmetricspb.ExportMetricsServiceResponse, error) {
	slog.DebugContext(ctx, "Received ExportMetricsServiceRequest")
	metricsReceivedCounter.Add(ctx, 1)

	if m.store != nil {
		rm := request.GetResourceMetrics()

		if gaugeRows := metric.MapGaugeRows(rm); len(gaugeRows) > 0 {
			if err := m.store.InsertGauge(ctx, gaugeRows); err != nil {
				return nil, err
			}
		}
		if sumRows := metric.MapSumRows(rm); len(sumRows) > 0 {
			if err := m.store.InsertSum(ctx, sumRows); err != nil {
				return nil, err
			}
		}
	}

	return &colmetricspb.ExportMetricsServiceResponse{}, nil
}
