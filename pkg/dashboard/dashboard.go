package dashboard

import (
	core "github.com/goliatone/go-dashboard/components/dashboard"
)

// Service exposes the underlying components/dashboard.Service type.
type Service = core.Service

// Options re-export for convenience.
type Options = core.Options

// ThemeProvider re-export for convenience.
type ThemeProvider = core.ThemeProvider

// ThemeSelector re-export for convenience.
type ThemeSelector = core.ThemeSelector

// ThemeSelectorFunc re-export for convenience.
type ThemeSelectorFunc = core.ThemeSelectorFunc

// ThemeSelection re-export for convenience.
type ThemeSelection = core.ThemeSelection

// ThemeAssets re-export for convenience.
type ThemeAssets = core.ThemeAssets

// NewService proxies to the internal constructor.
func NewService(opts Options) *Service {
	return core.NewService(opts)
}
