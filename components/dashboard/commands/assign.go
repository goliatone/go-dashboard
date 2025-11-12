package commands

import (
	"context"
	"errors"

	gocommand "github.com/goliatone/go-command"
	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

// AssignWidgetCommand wraps Service.AddWidget so transports can invoke widget
// assignments without linking directly against the service.
type assignService interface {
	AddWidget(ctx context.Context, req dashboard.AddWidgetRequest) error
}

// AssignWidgetCommand translates incoming requests into service calls and emits
// telemetry so operators can observe widget assignment activity.
type AssignWidgetCommand struct {
	service   assignService
	telemetry Telemetry
}

// NewAssignWidgetCommand creates a command instance.
func NewAssignWidgetCommand(service assignService, telemetry Telemetry) *AssignWidgetCommand {
	return &AssignWidgetCommand{service: service, telemetry: normalizeTelemetry(telemetry)}
}

var _ gocommand.Commander[dashboard.AddWidgetRequest] = (*AssignWidgetCommand)(nil)

// Execute delegates to the dashboard service.
func (c *AssignWidgetCommand) Execute(ctx context.Context, msg dashboard.AddWidgetRequest) error {
	if c.service == nil {
		return errors.New("assign command requires service")
	}
	if err := c.service.AddWidget(ctx, msg); err != nil {
		return err
	}
	c.telemetry.Record(ctx, "dashboard.widget.assign", map[string]any{
		"definition_id": msg.DefinitionID,
		"area_code":     msg.AreaCode,
	})
	return nil
}
