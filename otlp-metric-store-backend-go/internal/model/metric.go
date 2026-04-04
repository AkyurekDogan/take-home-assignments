package model

import "time"

// GaugeRow represents a single gauge data point for ClickHouse insertion.
// Each field corresponds to OTLP Resource/Scope/Metric/DataPoint attributes that
// together describe the identity and value of the time series point.
type GaugeRow struct {
	// ResourceAttributes are the OTLP resource key/value pairs (e.g., service.name,
	// host.name, k8s.*). They describe the entity that produced the metric and are
	// used as part of the series identity (meta data) in the lookup table.
	ResourceAttributes map[string]string

	// ResourceSchemaUrl is the schema URL for the resource that indicates the
	// semantic conventions version used to encode resource attributes.
	ResourceSchemaUrl string

	// ScopeName is the instrumentation scope (library) name that produced the metric
	// (formerly InstrumentationLibraryName).
	ScopeName string

	// ScopeVersion is the version of the instrumentation scope (library).
	ScopeVersion string

	// ScopeAttributes are attributes attached to the instrumentation scope.
	ScopeAttributes map[string]string

	// ScopeDroppedAttrCount is the number of scope attributes that were dropped by
	// the SDK/exporter prior to reaching this backend (diagnostic information).
	ScopeDroppedAttrCount uint32

	// ScopeSchemaUrl is the schema URL for the scope that indicates the semantic
	// conventions version for scope attributes.
	ScopeSchemaUrl string

	// ServiceName is a convenience extraction of resource["service.name"]. It is
	// frequently used for partitioning and querying.
	ServiceName string

	// MetricName is the OTLP metric name (e.g., http.server.duration).
	MetricName string

	// MetricDescription is the human‑readable description of the metric.
	MetricDescription string

	// MetricUnit is the measurement unit (e.g., s, ms, By, {request}).
	MetricUnit string

	// Attributes are the data point (label/tag) key/value pairs that specialize a
	// metric (e.g., method=GET, status=200). They are part of the series identity.
	Attributes map[string]string

	// StartTimeUnix is the data point start time (for delta/cumulative contexts).
	StartTimeUnix time.Time

	// TimeUnix is the data point timestamp (wall‑clock event time at nanosecond precision).
	TimeUnix time.Time

	// Value is the numeric sample value for the gauge at TimeUnix.
	Value float64

	// Flags carries OTLP data point flags (e.g., NO_RECORDED_VALUE).
	Flags uint32
}

// SumRow represents a single sum data point for ClickHouse insertion.
// It embeds GaugeRow fields for shared resource/scope/metric/data point metadata and
// adds Sum‑specific properties.
type SumRow struct {
	GaugeRow
	// AggregationTemporality indicates DELTA or CUMULATIVE semantics as defined by OTLP.
	AggregationTemporality int32
	// IsMonotonic specifies whether the counter is strictly increasing over time.
	IsMonotonic bool
}
