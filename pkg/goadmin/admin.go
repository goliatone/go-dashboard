package goadmin

import (
	"context"
	"errors"

	activitypkg "github.com/goliatone/go-dashboard/pkg/activity"
	dashboardpkg "github.com/goliatone/go-dashboard/pkg/dashboard"
)

// MenuBuilder ensures dashboard entries exist within the admin navigation.
type MenuBuilder interface {
	EnsureMenuItem(ctx context.Context, menuCode string, item MenuItem) error
}

// MenuItem captures dashboard link metadata.
type MenuItem struct {
	Label    string
	Route    string
	Icon     string
	Position int
}

// Config wires dashboard service + feature flags into an admin shell.
type Config struct {
	EnableDashboard bool
	MenuCode        string
	MenuBuilder     MenuBuilder
	Service         *dashboardpkg.Service
	DefaultMenuItem MenuItem
	ActivityHooks   activitypkg.Hooks
	ActivityConfig  activitypkg.Config
}

// Admin exposes helpers for go-admin style applications.
type Admin struct {
	cfg Config
}

// New creates an Admin helper that can seed dashboard menus.
func New(cfg Config) (*Admin, error) {
	if cfg.EnableDashboard && cfg.Service == nil {
		return nil, errors.New("goadmin: dashboard service is required when enabled")
	}
	if cfg.MenuCode == "" {
		cfg.MenuCode = "admin.main"
	}
	if cfg.DefaultMenuItem.Label == "" {
		cfg.DefaultMenuItem.Label = "Dashboard"
	}
	if cfg.DefaultMenuItem.Route == "" {
		cfg.DefaultMenuItem.Route = "admin.dashboard"
	}
	if cfg.DefaultMenuItem.Icon == "" {
		cfg.DefaultMenuItem.Icon = "home"
	}
	return &Admin{cfg: cfg}, nil
}

// Dashboard exposes the configured dashboard service when enabled.
func (a *Admin) Dashboard() *dashboardpkg.Service {
	if !a.cfg.EnableDashboard {
		return nil
	}
	return a.cfg.Service
}

// Bootstrap seeds menu entries when dashboard support is enabled.
func (a *Admin) Bootstrap(ctx context.Context) error {
	if !a.cfg.EnableDashboard || a.cfg.MenuBuilder == nil {
		return nil
	}
	return a.cfg.MenuBuilder.EnsureMenuItem(ctx, a.cfg.MenuCode, a.cfg.DefaultMenuItem)
}
