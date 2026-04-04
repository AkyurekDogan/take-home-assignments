package store

// Dimension table containing unique metric metadata rows, referenced by meta_id.
// This table is designed to be wide to capture all relevant metadata for a metric in a single row,
// which simplifies querying and reduces the need for joins. The meta_id serves as a surrogate key
// that can be efficiently referenced by the fact tables (gauge, sum, etc.) to associate metric data
// points with their corresponding metadata.
const createMetaTableSQL = `
CREATE TABLE IF NOT EXISTS otel_metric_meta (
    meta_id UInt64,
    ServiceName LowCardinality(String) CODEC(ZSTD(1)),
    ResourceAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ResourceSchemaUrl String CODEC(ZSTD(1)),
    ScopeName String CODEC(ZSTD(1)),
    ScopeVersion String CODEC(ZSTD(1)),
    ScopeAttributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    ScopeDroppedAttrCount UInt32 CODEC(ZSTD(1)),
    ScopeSchemaUrl String CODEC(ZSTD(1)),
    MetricName LowCardinality(String) CODEC(ZSTD(1)),
    MetricDescription String CODEC(ZSTD(1)),
    MetricUnit String CODEC(ZSTD(1)),
    Attributes Map(LowCardinality(String), String) CODEC(ZSTD(1)),
    AggregationTemporality Nullable(Int32) CODEC(ZSTD(1)),
    IsMonotonic Nullable(Bool) CODEC(ZSTD(1))
)
-- Using ReplacingMergeTree to allow idempotent upserts of identical meta rows.
-- The meta table is a dimension keyed by meta_id; duplicates with the same
-- meta_id (and identical content) may be inserted across parts during
-- high-throughput ingestion. ReplacingMergeTree collapses these duplicates
-- during merges without requiring explicit UPDATE/DELETE operations, whereas
-- a plain MergeTree would retain all duplicates.
ENGINE ReplacingMergeTree()
ORDER BY (meta_id)
SETTINGS index_granularity = 8192;
`

// Fact tables: only reference meta, hold timestamps and values.
const createGaugeTableSQL = `
CREATE TABLE IF NOT EXISTS otel_metrics_gauge (
    meta_id UInt64,
    StartTimeUnix DateTime64(9) CODEC(Delta(8), ZSTD(1)),
    TimeUnix DateTime64(9) CODEC(Delta(8), ZSTD(1)),
    Value Float64 CODEC(ZSTD(1)),
    Flags UInt32 CODEC(ZSTD(1))
) ENGINE MergeTree()
PARTITION BY toDate(TimeUnix)
ORDER BY (meta_id, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;
`

const createSumTableSQL = `
CREATE TABLE IF NOT EXISTS otel_metrics_sum (
    meta_id UInt64,
    StartTimeUnix DateTime64(9) CODEC(Delta(8), ZSTD(1)),
    TimeUnix DateTime64(9) CODEC(Delta(8), ZSTD(1)),
    Value Float64 CODEC(ZSTD(1)),
    Flags UInt32 CODEC(ZSTD(1))
) ENGINE MergeTree()
PARTITION BY toDate(TimeUnix)
ORDER BY (meta_id, toUnixTimestamp64Nano(TimeUnix))
SETTINGS index_granularity = 8192, ttl_only_drop_parts = 1;
`
