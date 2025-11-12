package commands

import (
	"context"
	"errors"

	gocommand "github.com/goliatone/go-command"
	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

// SaveLayoutPreferencesInput captures viewer overrides for layout customization.
type SaveLayoutPreferencesInput struct {
	Viewer        dashboard.ViewerContext `json:"viewer"`
	AreaOrder     map[string][]string     `json:"area_order"`
	HiddenWidgets []string                `json:"hidden_widget_ids"`
}

type preferenceService interface {
	SavePreferences(ctx context.Context, viewer dashboard.ViewerContext, overrides dashboard.LayoutOverrides) error
}

// SaveLayoutPreferencesCommand persists per-user layout overrides.
type SaveLayoutPreferencesCommand struct {
	service   preferenceService
	telemetry Telemetry
}

// NewSaveLayoutPreferencesCommand creates the command.
func NewSaveLayoutPreferencesCommand(service preferenceService, telemetry Telemetry) *SaveLayoutPreferencesCommand {
	return &SaveLayoutPreferencesCommand{service: service, telemetry: normalizeTelemetry(telemetry)}
}

var _ gocommand.Commander[SaveLayoutPreferencesInput] = (*SaveLayoutPreferencesCommand)(nil)

// Execute stores the provided overrides for the viewer.
func (c *SaveLayoutPreferencesCommand) Execute(ctx context.Context, msg SaveLayoutPreferencesInput) error {
	if c.service == nil {
		return errors.New("preferences command requires service")
	}
	if msg.Viewer.UserID == "" {
		return errors.New("preferences command requires viewer user id")
	}
	overrides := dashboard.LayoutOverrides{
		AreaOrder:     msg.AreaOrder,
		HiddenWidgets: make(map[string]bool, len(msg.HiddenWidgets)),
	}
	for _, id := range msg.HiddenWidgets {
		overrides.HiddenWidgets[id] = true
	}
	if err := c.service.SavePreferences(ctx, msg.Viewer, overrides); err != nil {
		return err
	}
	c.telemetry.Record(ctx, "dashboard.preferences.save", map[string]any{
		"user_id":    msg.Viewer.UserID,
		"areas":      len(msg.AreaOrder),
		"hidden_cnt": len(msg.HiddenWidgets),
	})
	return nil
}
