package store

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

var ErrClickHouseDisabled = fmt.Errorf("clickhouse storage is disabled")

// ClickHouseOptions encapsulates connection settings and bootstrap behavior.
type ClickHouseOptions struct {
	IsEnabled bool
	Addr      string
	Database  string
	Username  string
	Password  string
}

// NewClickHouse opens a ClickHouse connection, verifies it, wraps it in
// the Metric store abstraction, and optionally creates required tables.
func NewClickhouse(ctx context.Context, opts ClickHouseOptions) (driver.Conn, error) {
	if !opts.IsEnabled || opts.Addr == "" {
		return nil, ErrClickHouseDisabled
	}
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{opts.Addr},
		Auth: clickhouse.Auth{
			Database: opts.Database,
			Username: opts.Username,
			Password: opts.Password,
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
	return conn, nil
}
