package config

import "flag"

const (
	defaultListenAddr            = "localhost:4317"
	defaultMaxReceiveMessageSize = 16 * 1024 * 1024
)

type Config struct {
	GRPC       GRPCConfig
	ClickHouse ClickHouseConfig
}

type GRPCConfig struct {
	ListenAddr            string
	MaxReceiveMessageSize int
}

type ClickHouseConfig struct {
	Addr     string
	Database string
	Username string
	Password string
	Enabled  bool
}

func MustLoad() Config {
	var cfg Config

	flag.StringVar(
		&cfg.GRPC.ListenAddr,
		"listenAddr",
		defaultListenAddr,
		"The listen address",
	)

	flag.IntVar(
		&cfg.GRPC.MaxReceiveMessageSize,
		"maxReceiveMessageSize",
		defaultMaxReceiveMessageSize,
		"The max message size in bytes the server can receive",
	)

	flag.StringVar(
		&cfg.ClickHouse.Addr,
		"clickhouseAddr",
		"",
		"ClickHouse address in host:port format (optional)",
	)

	flag.StringVar(
		&cfg.ClickHouse.Database,
		"clickhouseDatabase",
		"default",
		"ClickHouse database name",
	)

	flag.StringVar(
		&cfg.ClickHouse.Username,
		"clickhouseUsername",
		"default",
		"ClickHouse username",
	)

	flag.StringVar(
		&cfg.ClickHouse.Password,
		"clickhousePassword",
		"",
		"ClickHouse password",
	)

	flag.BoolVar(
		&cfg.ClickHouse.Enabled,
		"clickhouseEnabled",
		false,
		"Enable ClickHouse storage",
	)

	flag.Parse()
	return cfg
}
