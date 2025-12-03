package commands

import (
	"context"
	"errors"

	gocommand "github.com/goliatone/go-command"
	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

// UpdateWidgetInput captures widget update payloads.
type UpdateWidgetInput struct {
	WidgetID      string         `json:"widget_id"`
	Configuration map[string]any `json:"configuration"`
	Metadata      map[string]any `json:"metadata"`
	ActorID       string         `json:"actor_id"`
	UserID        string         `json:"user_id"`
	TenantID      string         `json:"tenant_id"`
}

type updateService interface {
	UpdateWidget(ctx context.Context, widgetID string, req dashboard.UpdateWidgetRequest) error
}

// UpdateWidgetCommand wraps Service.UpdateWidget.
type UpdateWidgetCommand struct {
	service   updateService
	telemetry Telemetry
}

// NewUpdateWidgetCommand creates the command.
func NewUpdateWidgetCommand(service updateService, telemetry Telemetry) *UpdateWidgetCommand {
	return &UpdateWidgetCommand{service: service, telemetry: normalizeTelemetry(telemetry)}
}

var _ gocommand.Commander[UpdateWidgetInput] = (*UpdateWidgetCommand)(nil)

// Execute updates widget configuration/metadata.
func (c *UpdateWidgetCommand) Execute(ctx context.Context, msg UpdateWidgetInput) error {
	if c.service == nil {
		return errors.New("update command requires service")
	}
	if msg.WidgetID == "" {
		return errors.New("update command requires widget id")
	}
	ctx = dashboard.ContextWithActivity(ctx, dashboard.ActivityContext{
		ActorID:  msg.ActorID,
		UserID:   msg.UserID,
		TenantID: msg.TenantID,
	})
	req := dashboard.UpdateWidgetRequest{
		Configuration: msg.Configuration,
		Metadata:      msg.Metadata,
		ActorID:       msg.ActorID,
		UserID:        msg.UserID,
		TenantID:      msg.TenantID,
	}
	if err := c.service.UpdateWidget(ctx, msg.WidgetID, req); err != nil {
		return err
	}
	c.telemetry.Record(ctx, "dashboard.widget.update", map[string]any{
		"widget_id": msg.WidgetID,
	})
	return nil
}
