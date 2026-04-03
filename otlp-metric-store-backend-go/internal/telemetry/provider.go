package telemetry

import "context"

type Provider interface {
	Setup(ctx context.Context) (shutdown func(context.Context) error, err error)
}
