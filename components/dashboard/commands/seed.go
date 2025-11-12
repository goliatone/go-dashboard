package commands

import (
	"context"
	"errors"

	gocommand "github.com/goliatone/go-command"
	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

// SeedDashboardInput controls bootstrap behavior.
type SeedDashboardInput struct {
	SeedLayout bool
}

// SeedDashboardCommand registers areas/definitions and optionally seeds layout.
type SeedDashboardCommand struct {
	store     dashboard.WidgetStore
	registry  dashboard.ProviderRegistry
	service   *dashboard.Service
	telemetry Telemetry
}

// NewSeedDashboardCommand wires dependencies.
func NewSeedDashboardCommand(store dashboard.WidgetStore, registry dashboard.ProviderRegistry, service *dashboard.Service, telemetry Telemetry) *SeedDashboardCommand {
	return &SeedDashboardCommand{
		store:     store,
		registry:  registry,
		service:   service,
		telemetry: normalizeTelemetry(telemetry),
	}
}

var _ gocommand.Commander[SeedDashboardInput] = (*SeedDashboardCommand)(nil)

// Execute runs the bootstrap pipeline.
func (c *SeedDashboardCommand) Execute(ctx context.Context, msg SeedDashboardInput) error {
	if c.store == nil {
		return errors.New("seed command requires widget store")
	}
	if err := dashboard.RegisterAreas(ctx, c.store); err != nil {
		return err
	}
	if err := dashboard.RegisterDefinitions(ctx, c.store, c.registry); err != nil {
		return err
	}
	if msg.SeedLayout && c.service != nil {
		if err := dashboard.SeedLayout(ctx, c.service); err != nil {
			return err
		}
	}
	c.telemetry.Record(ctx, "dashboard.seed", map[string]any{"seed_layout": msg.SeedLayout})
	return nil
}
