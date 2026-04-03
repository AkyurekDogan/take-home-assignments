package store

// Dimension table containing unique metric metadata rows, referenced by meta_id.
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
) ENGINE ReplacingMergeTree()
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
