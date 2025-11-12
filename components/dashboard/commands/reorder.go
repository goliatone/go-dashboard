package commands

import (
	"context"
	"errors"

	gocommand "github.com/goliatone/go-command"
)

// ReorderWidgetsInput contains the reorder payload.
type ReorderWidgetsInput struct {
	AreaCode  string
	WidgetIDs []string
}

// ReorderWidgetsCommand wraps Service.ReorderWidgets.
type reorderService interface {
	ReorderWidgets(ctx context.Context, areaCode string, widgetIDs []string) error
}

// ReorderWidgetsCommand wraps Service.ReorderWidgets.
type ReorderWidgetsCommand struct {
	service   reorderService
	telemetry Telemetry
}

// NewReorderWidgetsCommand builds the command.
func NewReorderWidgetsCommand(service reorderService, telemetry Telemetry) *ReorderWidgetsCommand {
	return &ReorderWidgetsCommand{service: service, telemetry: normalizeTelemetry(telemetry)}
}

var _ gocommand.Commander[ReorderWidgetsInput] = (*ReorderWidgetsCommand)(nil)

// Execute applies the new ordering.
func (c *ReorderWidgetsCommand) Execute(ctx context.Context, msg ReorderWidgetsInput) error {
	if c.service == nil {
		return errors.New("reorder command requires service")
	}
	if err := c.service.ReorderWidgets(ctx, msg.AreaCode, msg.WidgetIDs); err != nil {
		return err
	}
	c.telemetry.Record(ctx, "dashboard.widget.reorder", map[string]any{
		"area_code": msg.AreaCode,
		"count":     len(msg.WidgetIDs),
	})
	return nil
}
