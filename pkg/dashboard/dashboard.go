package dashboard

import (
	core "github.com/goliatone/go-dashboard/components/dashboard"
)

// Service exposes the underlying components/dashboard.Service type.
type Service = core.Service

// Options re-export for convenience.
type Options = core.Options

// NewService proxies to the internal constructor.
func NewService(opts Options) *Service {
	return core.NewService(opts)
}
