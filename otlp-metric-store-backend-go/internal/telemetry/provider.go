package telemetry

import "context"

// Provider is an interface for telemetry providers that can be set up and shut down.
type Provider interface {
	Setup(ctx context.Context) (shutdown func(context.Context) error, err error)
}
