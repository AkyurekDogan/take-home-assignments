# OTLP Metric Storage (Go)

## Introduction
This take-home assignment is designed to give you an opportunity to demonstrate your skills and experience in
building a small backend application. We expect you to spend 3-4 hours on this assignment (using AI coding agents).
If you find yourself spending more time than that, please stop and submit what you have. We are not looking for a
complete solution, but rather a demonstration of your skills and experience.

To submit your solution, please create a public GitHub repository and send us the link. Please include a `README.md` file
with instructions on how to run your application.

## Overview
The goal of this assignment is to build a simple backend application that receives [metric datapoints](https://opentelemetry.io/docs/concepts/signals/metrics/)
on a gRPC endpoint and processes them, before storing in ClickHouse.
Current state is that we have a gRPC endpoint for receiving metrics, and Gauge and Sum type get correctly converted to
records and inserted into ClickHouse. This is tested with both unit- and integration-tests.

What we're looking for is to extract meta-data about the metrics into a separate table, which will then act as a 'lookup'
table, and that actual data-points just get stored as value + timestamp and with a reference to the lookup table.

Think about and keep in mind the following things:
- How to do the reference between tables?
- How to efficiently store the meta-data in ClickHouse?
- All data should be stored in such a way that full table scans are never needed, under the assumption data always gets queried for a specific time-frame
- Other than time-frame, there are no other mandatory filters for querying
- While you can assume cardinality of the metrics is 'low', e.g. Resources (Attributes) are likely to change over time 

Your solution should take into account high throughput, both in number of messages and the number of metrics / data-points per message.

Feel free to use the existing scaffoling in this folder. Of course, you can also change anything else as you see fit.

## Technology Constraints
- Your Go program should compile using standard Go SDK, and be compatible with Go 1.26.
- Use any additional libraries you want and need.

## Notes
- As this assignment is for the role of a Staff / Senior Product Engineer, we expect you to pay some attention to maintainability and operability of the solution. For example:
  - Consistent terminology usage
  - Validation of the behaviour
  - Include signals / events to help in debugging
- Assume that this application will be deployed to production. Build it accordingly.

## Usage

Build the application:
```shell
go build ./...
```

Run the application:
```shell
go run ./...
```

Run tests
```shell
go test ./...
```

## References

- [OpenTelemetry Metrics](https://opentelemetry.io/docs/concepts/signals/metrics/)
- [OpenTelemetry Protocol (OTLP)](https://github.com/open-telemetry/opentelemetry-proto)

## Solution Details

This implementation normalizes metric metadata into a dedicated lookup table and stores metric datapoints as compact fact rows that reference the metadata by key. The design targets high throughput ingestion and efficient time-window queries without full table scans.

Key artifacts (click to open):
- Schema DDLs: [internal/store/schema.go](otlp-metric-store-backend-go/internal/store/schema.go)
- ClickHouse store (ingestion logic): [internal/store/metric.go](otlp-metric-store-backend-go/internal/store/metric.go)
- gRPC service (OTLP ingestion): [internal/grpc/metrics_server.go](otlp-metric-store-backend-go/internal/grpc/metrics_server.go)
- gRPC server bootstrap: [internal/grpc/server.go](otlp-metric-store-backend-go/internal/grpc/server.go)
- OTLP-to-internal mappers: [internal/grpc/mapper.go](otlp-metric-store-backend-go/internal/grpc/mapper.go)
- Telemetry provider: [internal/telemetry/otel.go](otlp-metric-store-backend-go/internal/telemetry/otel.go)
- YAML configuration: [internal/config/config.go](otlp-metric-store-backend-go/internal/config/config.go) and example [config.yml](otlp-metric-store-backend-go/config.yml)
- Application wiring: [cmd/main.go](otlp-metric-store-backend-go/cmd/main.go)

### Normalized Data Model

Metadata (dimension) table (lookup):
- Table: `otel_metric_meta`
- Columns include service and resource identity, scope (instrumentation library) info, metric identity (name/description/unit), data-point attributes (tags), and for sums, temporality/monotonicity.
- Engine: ReplacingMergeTree (see inline comment in [internal/store/schema.go](otlp-metric-store-backend-go/internal/store/schema.go)) to allow idempotent upserts and deduplication of identical metadata rows during merges.

Fact tables (time-series):
- `otel_metrics_gauge(meta_id, StartTimeUnix, TimeUnix, Value, Flags)`
- `otel_metrics_sum(meta_id, StartTimeUnix, TimeUnix, Value, Flags)`
- Both are MergeTree, partitioned by `toDate(TimeUnix)`, ordered by `(meta_id, toUnixTimestamp64Nano(TimeUnix))` for efficient time-windowed scans and segment pruning.

Why this model:
- Referencing by `meta_id` keeps fact rows lean and ingestion fast.
- Partitioning by date plus ordering by `(meta_id, time)` ensures time-bounded queries avoid full scans.
- Changing attributes/resources naturally produce new `meta_id` rows while preserving historical queryability.

### Ingestion Flow

1) gRPC receives OTLP metrics at the MetricsService `Export` endpoint: [internal/grpc/metrics_server.go](otlp-metric-store-backend-go/internal/grpc/metrics_server.go)
2) OTLP data is mapped into internal rows using mappers in [internal/grpc/mapper.go](otlp-metric-store-backend-go/internal/grpc/mapper.go)
3) The ClickHouse store writes using a two-phase batch per metric type: [internal/store/metric.go](otlp-metric-store-backend-go/internal/store/metric.go)
   - Upsert unique metadata rows into `otel_metric_meta` (explicit column lists to avoid order mistakes)
   - Insert fact rows referencing `meta_id` with timestamps and values

High-throughput notes:
- Deduplication of meta per batch avoids repeated inserts in a single batch.
- ReplacingMergeTree collapses duplicate meta rows across parts during background merges.
- Fact rows are minimal (key + time + value + flags), improving throughput and storage.

### Querying Patterns

Always include a time filter (PREWHERE) to leverage partition pruning:

```sql
-- Gauge time series over last hour for a given metric and service
SELECT f.TimeUnix, f.Value
FROM otel_metrics_gauge AS f
INNER JOIN otel_metric_meta AS m USING (meta_id)
PREWHERE f.TimeUnix >= now() - INTERVAL 1 HOUR
WHERE m.MetricName = 'cpu.utilization' AND m.ServiceName = 'api'
ORDER BY f.TimeUnix;
```

```sql
-- Sum aggregation per minute with an attribute filter in a day range
SELECT toStartOfMinute(f.TimeUnix) AS ts, sum(f.Value) AS total
FROM otel_metrics_sum AS f
INNER JOIN otel_metric_meta AS m USING (meta_id)
PREWHERE f.TimeUnix >= toDateTime('2026-04-03 00:00:00')
  AND f.TimeUnix < toDateTime('2026-04-04 00:00:00')
WHERE m.MetricName = 'http.requests.total' AND m.Attributes['method'] = 'GET'
GROUP BY ts
ORDER BY ts;
```

### Configuration

Application reads configuration from YAML (default path: `config.yml`):
- Example: [config.yml](otlp-metric-store-backend-go/config.yml)
- Schema: [internal/config/config.go](otlp-metric-store-backend-go/internal/config/config.go)

Blocks:
- `grpc`: listen address and max message size
- `clickhouse`: connectivity and enable flag; when enabled, the app ensures tables on startup
- `telemetry`: serviceName, serviceNamespace, serviceVersion for OpenTelemetry Resource

### Telemetry

The app initializes OpenTelemetry via a provider abstraction: [internal/telemetry/otel.go](otlp-metric-store-backend-go/internal/telemetry/otel.go)
- Current exporters are stdout for traces/metrics/logs for quick local inspection.
- The Resource is built from YAML config in [cmd/main.go](otlp-metric-store-backend-go/cmd/main.go) so service identity can be changed without code edits.
- To send to a dashboard (Jaeger, Grafana, etc.), replace the stdout exporters with OTLP exporters and run an OpenTelemetry Collector.

### gRPC Server and Service Wiring

- The gRPC server is created via a factory and exposes `grpc.ServiceRegistrar`: [internal/grpc/server.go](otlp-metric-store-backend-go/internal/grpc/server.go)
- The OTLP MetricsService implementation is registered against the server: [internal/grpc/metrics_server.go](otlp-metric-store-backend-go/internal/grpc/metrics_server.go)
- Application composition and lifecycle management live in [cmd/main.go](otlp-metric-store-backend-go/cmd/main.go)

### Running Locally

Build:
```shell
cd otlp-metric-store-backend-go
go build ./...
```

Run (uses config.yml by default):
```shell
cd otlp-metric-store-backend-go
go run ./cmd -config=config.yml
```

Notes:
- To enable ClickHouse persistence, set `clickhouse.enabled: true` and provide `addr/credentials` in `config.yml`. Tables are created automatically on startup.
- With ClickHouse disabled, the service still accepts metrics but does not persist them.

### Design Rationale (Summary)

- Reference key (`meta_id`) keeps facts lean and joins fast; metadata changes create new keys for historical accuracy.
- ReplacingMergeTree for metadata tolerates duplicate upserts and compacts storage automatically.
- Time partitioning and ordering by `(meta_id, time)` prevent full table scans under time-bounded queries.
- Explicit column lists in INSERT batches prevent schema/order drift issues during ingestion.
