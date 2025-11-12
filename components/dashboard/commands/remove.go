package commands

import (
	"context"
	"errors"

	gocommand "github.com/goliatone/go-command"
)

// RemoveWidgetInput identifies the widget instance to remove.
type RemoveWidgetInput struct {
	WidgetID string
}

// RemoveWidgetCommand wraps Service.RemoveWidget.
type removeService interface {
	RemoveWidget(ctx context.Context, widgetID string) error
}

// RemoveWidgetCommand wraps Service.RemoveWidget.
type RemoveWidgetCommand struct {
	service   removeService
	telemetry Telemetry
}

// NewRemoveWidgetCommand builds a command instance.
func NewRemoveWidgetCommand(service removeService, telemetry Telemetry) *RemoveWidgetCommand {
	return &RemoveWidgetCommand{service: service, telemetry: normalizeTelemetry(telemetry)}
}

var _ gocommand.Commander[RemoveWidgetInput] = (*RemoveWidgetCommand)(nil)

// Execute removes the widget.
func (c *RemoveWidgetCommand) Execute(ctx context.Context, msg RemoveWidgetInput) error {
	if c.service == nil {
		return errors.New("remove command requires service")
	}
	if err := c.service.RemoveWidget(ctx, msg.WidgetID); err != nil {
		return err
	}
	c.telemetry.Record(ctx, "dashboard.widget.remove", map[string]any{"widget_id": msg.WidgetID})
	return nil
}
