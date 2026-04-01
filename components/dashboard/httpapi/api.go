package httpapi

import (
	"context"
	"errors"

	gocommand "github.com/goliatone/go-command"
	"github.com/goliatone/go-dashboard/components/dashboard"
	"github.com/goliatone/go-dashboard/components/dashboard/commands"
)

type serviceBridge interface {
	AddWidget(ctx context.Context, req dashboard.AddWidgetRequest) error
	RemoveWidget(ctx context.Context, widgetID string) error
	ReorderWidgets(ctx context.Context, areaCode string, widgetIDs []string) error
	NotifyWidgetUpdated(ctx context.Context, event dashboard.WidgetEvent) error
	SavePreferences(ctx context.Context, viewer dashboard.ViewerContext, overrides dashboard.LayoutOverrides) error
}

// Executor defines a router-agnostic command surface that transports can call.
// Any HTTP, CLI, or background adapter can translate inbound payloads into the
// strongly typed inputs defined here and delegate to the shared commands.
type Executor interface {
	Assign(ctx context.Context, req dashboard.AddWidgetRequest) error
	Remove(ctx context.Context, input commands.RemoveWidgetInput) error
	Reorder(ctx context.Context, input commands.ReorderWidgetsInput) error
	Refresh(ctx context.Context, input commands.RefreshWidgetInput) error
	Preferences(ctx context.Context, input commands.SaveLayoutPreferencesInput) error
}

// CommandExecutor wires go-command.Commander instances into the Executor contract.
type CommandExecutor struct {
	AssignCommander  gocommand.Commander[dashboard.AddWidgetRequest]
	RemoveCommander  gocommand.Commander[commands.RemoveWidgetInput]
	ReorderCommander gocommand.Commander[commands.ReorderWidgetsInput]
	RefreshCommander gocommand.Commander[commands.RefreshWidgetInput]
	PrefsCommander   gocommand.Commander[commands.SaveLayoutPreferencesInput]
}

var _ Executor = (*CommandExecutor)(nil)

// ServiceExecutor adapts a dashboard service directly to the transport executor surface.
type ServiceExecutor struct {
	Service serviceBridge
}

var _ Executor = (*ServiceExecutor)(nil)

// NewServiceExecutor creates an executor backed directly by the shared dashboard service.
func NewServiceExecutor(service serviceBridge) *ServiceExecutor {
	return &ServiceExecutor{Service: service}
}

// Assign delegates widget creation to the configured command.
func (e *CommandExecutor) Assign(ctx context.Context, req dashboard.AddWidgetRequest) error {
	if e == nil || e.AssignCommander == nil {
		return errors.New("dashboard: assign command not configured")
	}
	return e.AssignCommander.Execute(ctx, req)
}

// Remove delegates widget removal to the configured command.
func (e *CommandExecutor) Remove(ctx context.Context, input commands.RemoveWidgetInput) error {
	if e == nil || e.RemoveCommander == nil {
		return errors.New("dashboard: remove command not configured")
	}
	return e.RemoveCommander.Execute(ctx, input)
}

// Reorder delegates ordering changes to the configured command.
func (e *CommandExecutor) Reorder(ctx context.Context, input commands.ReorderWidgetsInput) error {
	if e == nil || e.ReorderCommander == nil {
		return errors.New("dashboard: reorder command not configured")
	}
	return e.ReorderCommander.Execute(ctx, input)
}

// Refresh notifies refresh subscribers using the configured command.
func (e *CommandExecutor) Refresh(ctx context.Context, input commands.RefreshWidgetInput) error {
	if e == nil || e.RefreshCommander == nil {
		return errors.New("dashboard: refresh command not configured")
	}
	return e.RefreshCommander.Execute(ctx, input)
}

// Preferences saves layout overrides for the viewer.
func (e *CommandExecutor) Preferences(ctx context.Context, input commands.SaveLayoutPreferencesInput) error {
	if e == nil || e.PrefsCommander == nil {
		return errors.New("dashboard: preferences command not configured")
	}
	return e.PrefsCommander.Execute(ctx, input)
}

// Assign delegates widget creation directly to the configured service.
func (e *ServiceExecutor) Assign(ctx context.Context, req dashboard.AddWidgetRequest) error {
	if e == nil || e.Service == nil {
		return errors.New("dashboard: service executor not configured")
	}
	return e.Service.AddWidget(ctx, req)
}

// Remove delegates widget removal directly to the configured service.
func (e *ServiceExecutor) Remove(ctx context.Context, input commands.RemoveWidgetInput) error {
	if e == nil || e.Service == nil {
		return errors.New("dashboard: service executor not configured")
	}
	ctx = dashboard.ContextWithActivity(ctx, dashboard.ActivityContext{
		ActorID:  input.ActorID,
		UserID:   input.UserID,
		TenantID: input.TenantID,
	})
	return e.Service.RemoveWidget(ctx, input.WidgetID)
}

// Reorder delegates area ordering changes directly to the configured service.
func (e *ServiceExecutor) Reorder(ctx context.Context, input commands.ReorderWidgetsInput) error {
	if e == nil || e.Service == nil {
		return errors.New("dashboard: service executor not configured")
	}
	return e.Service.ReorderWidgets(ctx, input.AreaCode, input.WidgetIDs)
}

// Refresh delegates widget update notifications directly to the configured service.
func (e *ServiceExecutor) Refresh(ctx context.Context, input commands.RefreshWidgetInput) error {
	if e == nil || e.Service == nil {
		return errors.New("dashboard: service executor not configured")
	}
	return e.Service.NotifyWidgetUpdated(ctx, input.Event)
}

// Preferences converts the transport input into layout overrides and persists them via the configured service.
func (e *ServiceExecutor) Preferences(ctx context.Context, input commands.SaveLayoutPreferencesInput) error {
	if e == nil || e.Service == nil {
		return errors.New("dashboard: service executor not configured")
	}
	overrides := dashboard.LayoutOverrides{
		AreaOrder:     input.AreaOrder,
		AreaRows:      convertLayoutRowsInput(input.LayoutRows),
		HiddenWidgets: make(map[string]bool, len(input.HiddenWidgets)),
		Locale:        input.Viewer.Locale,
	}
	for _, id := range input.HiddenWidgets {
		overrides.HiddenWidgets[id] = true
	}
	return e.Service.SavePreferences(ctx, input.Viewer, overrides)
}

func convertLayoutRowsInput(input map[string][]commands.LayoutRowInput) map[string][]dashboard.LayoutRow {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string][]dashboard.LayoutRow, len(input))
	for area, rows := range input {
		mapped := make([]dashboard.LayoutRow, 0, len(rows))
		for _, row := range rows {
			if len(row.Widgets) == 0 {
				continue
			}
			slots := make([]dashboard.WidgetSlot, 0, len(row.Widgets))
			for _, widget := range row.Widgets {
				if widget.ID == "" {
					continue
				}
				slots = append(slots, dashboard.WidgetSlot{
					ID:    widget.ID,
					Width: widget.Width,
				})
			}
			if len(slots) == 0 {
				continue
			}
			mapped = append(mapped, dashboard.LayoutRow{Widgets: slots})
		}
		if len(mapped) > 0 {
			output[area] = mapped
		}
	}
	return output
}
