package commands

import (
	"context"
	"errors"

	gocommand "github.com/goliatone/go-command"
	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

// RefreshWidgetInput emits refresh notifications for a widget instance.
type RefreshWidgetInput struct {
	Event dashboard.WidgetEvent
}

// RefreshWidgetCommand triggers refresh hooks without forcing transports.
type refreshNotifier interface {
	NotifyWidgetUpdated(ctx context.Context, event dashboard.WidgetEvent) error
}

// RefreshWidgetCommand triggers refresh hooks without forcing transports.
type RefreshWidgetCommand struct {
	service   refreshNotifier
	telemetry Telemetry
}

// NewRefreshWidgetCommand creates the command.
func NewRefreshWidgetCommand(service refreshNotifier, telemetry Telemetry) *RefreshWidgetCommand {
	return &RefreshWidgetCommand{service: service, telemetry: normalizeTelemetry(telemetry)}
}

var _ gocommand.Commander[RefreshWidgetInput] = (*RefreshWidgetCommand)(nil)

// Execute notifies the dashboard service's refresh hooks.
func (c *RefreshWidgetCommand) Execute(ctx context.Context, msg RefreshWidgetInput) error {
	if c.service == nil {
		return errors.New("refresh command requires service")
	}
	if err := c.service.NotifyWidgetUpdated(ctx, msg.Event); err != nil {
		return err
	}
	c.telemetry.Record(ctx, "dashboard.widget.refresh", map[string]any{
		"area_code": msg.Event.AreaCode,
		"widget_id": msg.Event.Instance.ID,
	})
	return nil
}
