package clickhouse

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"dash0.com/otlp-log-processor-backend/internal/model"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	xxhash "github.com/cespare/xxhash/v2"
)

// MetricStore defines the interface for storing metrics in ClickHouse.
type MetricStore interface {
	CreateTables(ctx context.Context) error
	InsertGauge(ctx context.Context, rows []model.GaugeRow) error
	InsertSum(ctx context.Context, rows []model.SumRow) error
	Close() error
}

// metricStore implements MetricsStore using a ClickHouse connection.
type metricStore struct {
	conn driver.Conn
}

// NewMetric creates a new ClickHouseMetricsStore connected to the given address.
func NewMetric(
	ctx context.Context,
	addr string,
	database string,
	username string,
	password string,
) (MetricStore, error) {
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{addr},
		Auth: clickhouse.Auth{
			Database: database,
			Username: username,
			Password: password,
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		DialTimeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("opening clickhouse connection: %w", err)
	}
	if err := conn.Ping(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("pinging clickhouse: %w", err)
	}
	return &metricStore{conn: conn}, nil
}

// CreateTables executes DDL for meta and metric tables.
func (s *metricStore) CreateTables(ctx context.Context) error {
	ddls := []string{
		createGaugeTableSQL,
		createSumTableSQL,
		createHistogramTableSQL,
		createExponentialHistogramTableSQL,
		createSummaryTableSQL,
	}
	for _, ddl := range ddls {
		if err := s.conn.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("creating table: %w", err)
		}
	}
	return nil
}

// InsertGauge upserts meta rows and inserts fact rows into otel_metrics_gauge.
func (s *metricStore) InsertGauge(ctx context.Context, rows []model.GaugeRow) error {
	// Upsert unique metas for this batch
	seen := make(map[uint64]struct{}, len(rows))
	metaBatch, err := s.conn.PrepareBatch(ctx, "INSERT INTO otel_metric_meta")
	if err != nil {
		return fmt.Errorf("preparing meta batch (gauge): %w", err)
	}
	for _, r := range rows {
		id := computeMetaIDGauge(r)
		if _, ok := seen[id]; ok {
			continue
		}
		if err := metaBatch.Append(
			id,
			r.ResourceAttributes,
			r.ResourceSchemaUrl,
			r.ScopeName,
			r.ScopeVersion,
			r.ScopeAttributes,
			r.ScopeDroppedAttrCount,
			r.ScopeSchemaUrl,
			r.ServiceName,
			r.MetricName,
			r.MetricDescription,
			r.MetricUnit,
			r.Attributes,
			nil, // AggregationTemporality
			nil, // IsMonotonic
		); err != nil {
			return fmt.Errorf("appending meta row (gauge): %w", err)
		}
		seen[id] = struct{}{}
	}
	if err := metaBatch.Send(); err != nil {
		return fmt.Errorf("sending meta batch (gauge): %w", err)
	}

	// Insert fact rows
	factBatch, err := s.conn.PrepareBatch(ctx, "INSERT INTO otel_metrics_gauge")
	if err != nil {
		return fmt.Errorf("preparing gauge fact batch: %w", err)
	}
	for _, r := range rows {
		id := computeMetaIDGauge(r)
		if err := factBatch.Append(
			id,
			r.TimeUnix,
			r.Value,
			r.Flags,
		); err != nil {
			return fmt.Errorf("appending gauge fact row: %w", err)
		}
	}
	return factBatch.Send()
}

// InsertSum upserts meta rows and inserts fact rows into otel_metrics_sum.
func (s *metricStore) InsertSum(ctx context.Context, rows []model.SumRow) error {
	// Upsert unique metas for this batch
	seen := make(map[uint64]struct{}, len(rows))
	metaBatch, err := s.conn.PrepareBatch(ctx, "INSERT INTO otel_metric_meta")
	if err != nil {
		return fmt.Errorf("preparing meta batch (sum): %w", err)
	}
	for _, r := range rows {
		id := computeMetaIDSum(r)
		if _, ok := seen[id]; ok {
			continue
		}
		if err := metaBatch.Append(
			id,
			r.ResourceAttributes,
			r.ResourceSchemaUrl,
			r.ScopeName,
			r.ScopeVersion,
			r.ScopeAttributes,
			r.ScopeDroppedAttrCount,
			r.ScopeSchemaUrl,
			r.ServiceName,
			r.MetricName,
			r.MetricDescription,
			r.MetricUnit,
			r.Attributes,
			int32(r.AggregationTemporality),
			r.IsMonotonic,
		); err != nil {
			return fmt.Errorf("appending meta row (sum): %w", err)
		}
		seen[id] = struct{}{}
	}
	if err := metaBatch.Send(); err != nil {
		return fmt.Errorf("sending meta batch (sum): %w", err)
	}

	// Insert fact rows
	factBatch, err := s.conn.PrepareBatch(ctx, "INSERT INTO otel_metrics_sum")
	if err != nil {
		return fmt.Errorf("preparing sum fact batch: %w", err)
	}
	for _, r := range rows {
		id := computeMetaIDSum(r)
		if err := factBatch.Append(
			id,
			r.TimeUnix,
			r.Value,
			r.Flags,
		); err != nil {
			return fmt.Errorf("appending sum fact row: %w", err)
		}
	}
	return factBatch.Send()
}

// Close closes the underlying ClickHouse connection.
func (s *metricStore) Close() error {
	return s.conn.Close()
}

// --- helpers ---

func stableMapPairs(m map[string]string) []string {
	if len(m) == 0 {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(m))
	for _, k := range keys {
		out = append(out, k+"="+m[k])
	}
	return out
}

func computeMetaIDGauge(r model.GaugeRow) uint64 {
	var b strings.Builder
	b.WriteString("svc=")
	b.WriteString(r.ServiceName)
	b.WriteString("|name=")
	b.WriteString(r.MetricName)
	b.WriteString("|desc=")
	b.WriteString(r.MetricDescription)
	b.WriteString("|unit=")
	b.WriteString(r.MetricUnit)
	b.WriteString("|res=")
	b.WriteString(strings.Join(stableMapPairs(r.ResourceAttributes), ","))
	b.WriteString("|scope=")
	b.WriteString(r.ScopeName)
	b.WriteString("|scopever=")
	b.WriteString(r.ScopeVersion)
	b.WriteString("|scopeattrs=")
	b.WriteString(strings.Join(stableMapPairs(r.ScopeAttributes), ","))
	b.WriteString("|scopeschema=")
	b.WriteString(r.ScopeSchemaUrl)
	b.WriteString("|resschema=")
	b.WriteString(r.ResourceSchemaUrl)
	b.WriteString("|attrs=")
	b.WriteString(strings.Join(stableMapPairs(r.Attributes), ","))
	return xxhash.Sum64String(b.String())
}

func computeMetaIDSum(r model.SumRow) uint64 {
	var b strings.Builder
	b.WriteString("svc=")
	b.WriteString(r.ServiceName)
	b.WriteString("|name=")
	b.WriteString(r.MetricName)
	b.WriteString("|desc=")
	b.WriteString(r.MetricDescription)
	b.WriteString("|unit=")
	b.WriteString(r.MetricUnit)
	b.WriteString("|res=")
	b.WriteString(strings.Join(stableMapPairs(r.ResourceAttributes), ","))
	b.WriteString("|scope=")
	b.WriteString(r.ScopeName)
	b.WriteString("|scopever=")
	b.WriteString(r.ScopeVersion)
	b.WriteString("|scopeattrs=")
	b.WriteString(strings.Join(stableMapPairs(r.ScopeAttributes), ","))
	b.WriteString("|scopeschema=")
	b.WriteString(r.ScopeSchemaUrl)
	b.WriteString("|resschema=")
	b.WriteString(r.ResourceSchemaUrl)
	b.WriteString("|attrs=")
	b.WriteString(strings.Join(stableMapPairs(r.Attributes), ","))
	b.WriteString("|aggtemp=")
	b.WriteString(fmt.Sprintf("%d", r.AggregationTemporality))
	b.WriteString("|monotonic=")
	if r.IsMonotonic {
		b.WriteString("1")
	} else {
		b.WriteString("0")
	}
	return xxhash.Sum64String(b.String())
}
