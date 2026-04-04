package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	defaultListenAddr            = "localhost:4317"
	defaultMaxReceiveMessageSize = 16 * 1024 * 1024
	defaultConfigPath            = "config.yml"
)

type Config struct {
	GRPC       GRPCConfig       `yaml:"grpc"`
	ClickHouse ClickHouseConfig `yaml:"clickhouse"`
	Telemetry  TelemetryConfig  `yaml:"telemetry"`
}

type GRPCConfig struct {
	ListenAddr            string `yaml:"listenAddr"`
	MaxReceiveMessageSize int    `yaml:"maxReceiveMessageSize"`
}

type ClickHouseConfig struct {
	Addr     string `yaml:"addr"`
	Database string `yaml:"database"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Enabled  bool   `yaml:"enabled"`
}

// TelemetryConfig configures OpenTelemetry resource attributes used by the app.
type TelemetryConfig struct {
	ServiceName      string `yaml:"serviceName"`
	ServiceNamespace string `yaml:"serviceNamespace"`
	ServiceVersion   string `yaml:"serviceVersion"`
}

// MustLoad reads configuration from config.yml in the module root (or current working
// directory when running) and applies safe defaults for missing fields.
// It panics on read or parse errors to match previous fatal semantics.
func MustLoad() Config {
	return MustLoadFile(defaultConfigPath)
}

// MustLoadFile reads configuration from the provided YAML file path.
func MustLoadFile(path string) Config {
	// Start with defaults
	cfg := Config{
		GRPC: GRPCConfig{
			ListenAddr:            defaultListenAddr,
			MaxReceiveMessageSize: defaultMaxReceiveMessageSize,
		},
		ClickHouse: ClickHouseConfig{
			Database: "default",
			Username: "default",
			Enabled:  false,
		},
		Telemetry: TelemetryConfig{
			ServiceName:      "otlp-metrics-processor-backend",
			ServiceNamespace: "dash0-exercise",
			ServiceVersion:   "1.0.0",
		},
	}

	data, err := os.ReadFile(path)
	if err != nil {
		panic(fmt.Errorf("reading config file %s: %w", path, err))
	}
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		panic(fmt.Errorf("parsing config file %s: %w", path, err))
	}
	return cfg
}
