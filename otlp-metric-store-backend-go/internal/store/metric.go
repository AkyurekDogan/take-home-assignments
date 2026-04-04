package store

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"dash0.com/otlp-log-processor-backend/internal/model"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	xxhash "github.com/cespare/xxhash/v2"
)

// Metric defines the interface for storing metrics in ClickHouse.
type Metric interface {
	CreateTables(ctx context.Context) error
	InsertGauge(ctx context.Context, rows []model.GaugeRow) error
	InsertSum(ctx context.Context, rows []model.SumRow) error
	Close() error
}

// metric implements MetricsStore using a ClickHouse connection.
type metric struct {
	isDBEnabled bool
	conn        driver.Conn
}

// NewMetric creates a new ClickHouseMetricsStore connected to the given address.
func NewMetric(
	conn driver.Conn,
) Metric {
	return &metric{
		isDBEnabled: conn != nil,
		conn:        conn,
	}
}

// CreateTables executes DDL for meta and metric tables.
func (s *metric) CreateTables(ctx context.Context) error {
	if !s.isDBEnabled {
		slog.Info("Database disabled, skipping table creation")
		return nil
	}
	ddls := []string{
		createMetaTableSQL,
		createGaugeTableSQL,
		createSumTableSQL,
	}
	for _, ddl := range ddls {
		if err := s.conn.Exec(ctx, ddl); err != nil {
			return fmt.Errorf("creating table: %w", err)
		}
	}
	return nil
}

// InsertGauge upserts meta rows and inserts fact rows into otel_metrics_gauge.
func (s *metric) InsertGauge(
	ctx context.Context,
	rows []model.GaugeRow) error {
	if !s.isDBEnabled {
		slog.Info("Database disabled, skipping database insert for Gauge rows", slog.Int("rowCount", len(rows)))
		return nil
	}
	// Upsert unique meta rows (dedup within this batch) into dimension table
	if len(rows) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(rows))
	metaBatch, err := s.conn.PrepareBatch(ctx, `
        INSERT INTO otel_metric_meta (
            meta_id,
            ServiceName,
            ResourceAttributes,
            ResourceSchemaUrl,
            ScopeName,
            ScopeVersion,
            ScopeAttributes,
            ScopeDroppedAttrCount,
            ScopeSchemaUrl,
            MetricName,
            MetricDescription,
            MetricUnit,
            Attributes,
            AggregationTemporality,
            IsMonotonic
        )`)
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
			r.ServiceName,
			r.ResourceAttributes,
			r.ResourceSchemaUrl,
			r.ScopeName,
			r.ScopeVersion,
			r.ScopeAttributes,
			r.ScopeDroppedAttrCount,
			r.ScopeSchemaUrl,
			r.MetricName,
			r.MetricDescription,
			r.MetricUnit,
			r.Attributes,
			nil, // AggregationTemporality (nullable)
			nil, // IsMonotonic (nullable)
		); err != nil {
			return fmt.Errorf("appending meta row (gauge): %w", err)
		}
		seen[id] = struct{}{}
	}
	if err := metaBatch.Send(); err != nil {
		return fmt.Errorf("sending meta batch (gauge): %w", err)
	}

	// Insert fact rows with reference to meta_id
	factBatch, err := s.conn.PrepareBatch(ctx, `
        INSERT INTO otel_metrics_gauge (
            meta_id,
            StartTimeUnix,
            TimeUnix,
            Value,
            Flags
        )`)
	if err != nil {
		return fmt.Errorf("preparing gauge fact batch: %w", err)
	}
	for _, r := range rows {
		id := computeMetaIDGauge(r)
		if err := factBatch.Append(
			id,
			r.StartTimeUnix,
			r.TimeUnix,
			r.Value,
			r.Flags,
		); err != nil {
			return fmt.Errorf("appending gauge fact row: %w", err)
		}
	}
	if len(rows) == 0 {
		return nil
	}
	return factBatch.Send()
}

// InsertSum upserts meta rows and inserts fact rows into otel_metrics_sum.
func (s *metric) InsertSum(
	ctx context.Context,
	rows []model.SumRow) error {
	if !s.isDBEnabled {
		slog.Info("Database disabled, skipping database insert for Sum rows", slog.Int("rowCount", len(rows)))
		return nil
	}
	// Upsert unique meta rows (dedup within this batch) into dimension table
	if len(rows) == 0 {
		return nil
	}
	seen := make(map[uint64]struct{}, len(rows))
	metaBatch, err := s.conn.PrepareBatch(ctx, `
        INSERT INTO otel_metric_meta (
            meta_id,
            ServiceName,
            ResourceAttributes,
            ResourceSchemaUrl,
            ScopeName,
            ScopeVersion,
            ScopeAttributes,
            ScopeDroppedAttrCount,
            ScopeSchemaUrl,
            MetricName,
            MetricDescription,
            MetricUnit,
            Attributes,
            AggregationTemporality,
            IsMonotonic
        )`)
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
			r.ServiceName,
			r.ResourceAttributes,
			r.ResourceSchemaUrl,
			r.ScopeName,
			r.ScopeVersion,
			r.ScopeAttributes,
			r.ScopeDroppedAttrCount,
			r.ScopeSchemaUrl,
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

	// Insert fact rows with reference to meta_id
	factBatch, err := s.conn.PrepareBatch(ctx, `
        INSERT INTO otel_metrics_sum (
            meta_id,
            StartTimeUnix,
            TimeUnix,
            Value,
            Flags
        )`)
	if err != nil {
		return fmt.Errorf("preparing sum fact batch: %w", err)
	}
	for _, r := range rows {
		id := computeMetaIDSum(r)
		if err := factBatch.Append(
			id,
			r.StartTimeUnix,
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
func (s *metric) Close() error {
	return s.conn.Close()
}

// ---- helpers ----

// stableMapPairs takes a map and returns a slice of "key=value" strings sorted by key.
// This is used to create a consistent string representation of map fields for hashing,
// ensuring that the same map content always produces the same string regardless of the original key order.
// This is important for generating stable meta_id values for metrics, as the order of attributes should not affect the identity of the metric.
// If the map is empty, it returns nil to avoid unnecessary processing and to represent the absence of attributes clearly.
// For example, a map {"env": "prod", "version": "1.0"} would produce the slice ["env=prod", "version=1.0"] sorted by key.
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

// computeMetaIDGauge computes a hash of the identifying fields of a Gauge metric to generate a meta_id.
// This should include all fields that define the identity of the metric, excluding the value and timestamp fields.
// The same metric with different values or timestamps should yield the same meta_id, while different metrics should yield different meta_ids.
// The strings.Builder is used to create a consistent string representation of the metric's identity, which is then hashed using xxhash to produce a uint64 meta_id.
// Also prevents a lot of allocations by reusing the builder and avoiding intermediate string concatenations.
// Without labels, different values could accidentally produce ambiguous strings label structure can be dynamic and changing.

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
	return xxhash.Sum64String(b.String()) // A 64-bit integer is much cheaper and cleaner as a reference key.
}

// computeMetaIDSum computes a hash of the identifying fields of a Sum metric to generate a meta_id.
// This should include all fields that define the identity of the metric, excluding the value and timestamp fields.
// The same metric with different values or timestamps should yield the same meta_id, while different metrics should yield different meta_ids.
// The strings.Builder is used to create a consistent string representation of the metric's identity, which is then hashed using xxhash to produce a uint64 meta_id.
// Also prevents a lot of allocations by reusing the builder and avoiding intermediate string concatenations.
// Without labels, different values could accidentally produce ambiguous strings label structure can be dynamic and changing.
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
	b.WriteString(strconv.Itoa(int(r.AggregationTemporality)))
	b.WriteString("|monotonic=")
	if r.IsMonotonic {
		b.WriteString("1")
	} else {
		b.WriteString("0")
	}
	return xxhash.Sum64String(b.String()) // A 64-bit integer is much cheaper and cleaner as a reference key.
}
