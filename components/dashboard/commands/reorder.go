package commands

import (
	"context"
	"errors"

	gocommand "github.com/goliatone/go-command"
	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

// ReorderWidgetsInput contains the reorder payload.
type ReorderWidgetsInput struct {
	AreaCode  string
	WidgetIDs []string
	ActorID   string `json:"actor_id"`
	UserID    string `json:"user_id"`
	TenantID  string `json:"tenant_id"`
}

// ReorderWidgetsCommand wraps Service.ReorderWidgets so transports only have to
// worry about parsing JSON payloads before invoking the shared logic.
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
	ctx = dashboard.ContextWithActivity(ctx, dashboard.ActivityContext{
		ActorID:  msg.ActorID,
		UserID:   msg.UserID,
		TenantID: msg.TenantID,
	})
	if err := c.service.ReorderWidgets(ctx, msg.AreaCode, msg.WidgetIDs); err != nil {
		return err
	}
	c.telemetry.Record(ctx, "dashboard.widget.reorder", map[string]any{
		"area_code": msg.AreaCode,
		"count":     len(msg.WidgetIDs),
	})
	return nil
}
