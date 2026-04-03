package grpc

import (
	"context"
	"log/slog"

	"dash0.com/otlp-log-processor-backend/internal/clickhouse"
	colmetricspb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
)

type dash0MetricsServiceServer struct {
	store clickhouse.MetricStore

	colmetricspb.UnimplementedMetricsServiceServer
}

// NewMetricServer constructs a MetricsServiceServer.
// The first parameter (addr) is accepted for backward compatibility with call sites
// but is not used by the implementation.
func NewMetricServer(_ string, store clickhouse.MetricStore) colmetricspb.MetricsServiceServer {
	return &dash0MetricsServiceServer{store: store}
}

func (m *dash0MetricsServiceServer) Export(ctx context.Context, request *colmetricspb.ExportMetricsServiceRequest) (*colmetricspb.ExportMetricsServiceResponse, error) {
	slog.DebugContext(ctx, "Received ExportMetricsServiceRequest")
	metricsReceivedCounter.Add(ctx, 1)

	if m.store != nil {
		rm := request.GetResourceMetrics()

		if gaugeRows := MapGaugeRows(rm); len(gaugeRows) > 0 {
			if err := m.store.InsertGauge(ctx, gaugeRows); err != nil {
				return nil, err
			}
		}
		if sumRows := MapSumRows(rm); len(sumRows) > 0 {
			if err := m.store.InsertSum(ctx, sumRows); err != nil {
				return nil, err
			}
		}
	}

	return &colmetricspb.ExportMetricsServiceResponse{}, nil
}
