package model

import "time"

// GaugeRow represents a single gauge data point for ClickHouse insertion.
type GaugeRow struct {
	ResourceAttributes    map[string]string
	ResourceSchemaUrl     string
	ScopeName             string
	ScopeVersion          string
	ScopeAttributes       map[string]string
	ScopeDroppedAttrCount uint32
	ScopeSchemaUrl        string
	ServiceName           string
	MetricName            string
	MetricDescription     string
	MetricUnit            string
	Attributes            map[string]string
	StartTimeUnix         time.Time
	TimeUnix              time.Time
	Value                 float64
	Flags                 uint32
}

// SumRow represents a single sum data point for ClickHouse insertion.
type SumRow struct {
	GaugeRow
	AggregationTemporality int32
	IsMonotonic            bool
}
