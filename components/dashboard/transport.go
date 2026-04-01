package dashboard

import (
	"context"
	"errors"
)

// RemoveWidgetInput identifies the widget instance to remove.
type RemoveWidgetInput struct {
	WidgetID string `json:"widget_id"`
	ActorID  string `json:"actor_id"`
	UserID   string `json:"user_id"`
	TenantID string `json:"tenant_id"`
}

// ReorderWidgetsInput contains the reorder payload.
type ReorderWidgetsInput struct {
	AreaCode  string   `json:"area_code"`
	WidgetIDs []string `json:"widget_ids"`
	ActorID   string   `json:"actor_id"`
	UserID    string   `json:"user_id"`
	TenantID  string   `json:"tenant_id"`
}

// RefreshWidgetInput emits refresh notifications for a widget instance.
type RefreshWidgetInput struct {
	Event WidgetEvent `json:"event"`
}

// SaveLayoutPreferencesInput captures viewer overrides for layout customization.
type SaveLayoutPreferencesInput struct {
	Viewer        ViewerContext               `json:"viewer"`
	AreaOrder     map[string][]string         `json:"area_order"`
	LayoutRows    map[string][]LayoutRowInput `json:"layout_rows"`
	HiddenWidgets []string                    `json:"hidden_widget_ids"`
}

// LayoutRowInput represents widgets that share the same row in a transport payload.
type LayoutRowInput struct {
	Widgets []LayoutWidgetInput `json:"widgets"`
}

// LayoutWidgetInput describes a widget placement + width in a transport payload.
type LayoutWidgetInput struct {
	ID    string `json:"id"`
	Width int    `json:"width"`
}

// Executor defines a router-agnostic command surface that transports can call.
type Executor interface {
	Assign(ctx context.Context, req AddWidgetRequest) error
	Remove(ctx context.Context, input RemoveWidgetInput) error
	Reorder(ctx context.Context, input ReorderWidgetsInput) error
	Refresh(ctx context.Context, input RefreshWidgetInput) error
	Preferences(ctx context.Context, input SaveLayoutPreferencesInput) error
}

// ServiceExecutorService captures the shared dashboard service surface used by the default executor.
type ServiceExecutorService interface {
	AddWidget(ctx context.Context, req AddWidgetRequest) error
	RemoveWidget(ctx context.Context, widgetID string) error
	ReorderWidgets(ctx context.Context, areaCode string, widgetIDs []string) error
	NotifyWidgetUpdated(ctx context.Context, event WidgetEvent) error
	SavePreferences(ctx context.Context, viewer ViewerContext, overrides LayoutOverrides) error
}

// ServiceExecutor adapts a dashboard service directly to the transport executor surface.
type ServiceExecutor struct {
	Service ServiceExecutorService
}

var _ Executor = (*ServiceExecutor)(nil)

// NewServiceExecutor creates an executor backed directly by the shared dashboard service.
func NewServiceExecutor(service ServiceExecutorService) *ServiceExecutor {
	return &ServiceExecutor{Service: service}
}

// Assign delegates widget creation directly to the configured service.
func (e *ServiceExecutor) Assign(ctx context.Context, req AddWidgetRequest) error {
	if e == nil || e.Service == nil {
		return errors.New("dashboard: service executor not configured")
	}
	return e.Service.AddWidget(ctx, req)
}

// Remove delegates widget removal directly to the configured service.
func (e *ServiceExecutor) Remove(ctx context.Context, input RemoveWidgetInput) error {
	if e == nil || e.Service == nil {
		return errors.New("dashboard: service executor not configured")
	}
	ctx = ContextWithActivity(ctx, ActivityContext{
		ActorID:  input.ActorID,
		UserID:   input.UserID,
		TenantID: input.TenantID,
	})
	return e.Service.RemoveWidget(ctx, input.WidgetID)
}

// Reorder delegates area ordering changes directly to the configured service.
func (e *ServiceExecutor) Reorder(ctx context.Context, input ReorderWidgetsInput) error {
	if e == nil || e.Service == nil {
		return errors.New("dashboard: service executor not configured")
	}
	return e.Service.ReorderWidgets(ctx, input.AreaCode, input.WidgetIDs)
}

// Refresh delegates widget update notifications directly to the configured service.
func (e *ServiceExecutor) Refresh(ctx context.Context, input RefreshWidgetInput) error {
	if e == nil || e.Service == nil {
		return errors.New("dashboard: service executor not configured")
	}
	return e.Service.NotifyWidgetUpdated(ctx, input.Event)
}

// Preferences converts the transport input into layout overrides and persists them via the configured service.
func (e *ServiceExecutor) Preferences(ctx context.Context, input SaveLayoutPreferencesInput) error {
	if e == nil || e.Service == nil {
		return errors.New("dashboard: service executor not configured")
	}
	overrides := LayoutOverrides{
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

func convertLayoutRowsInput(input map[string][]LayoutRowInput) map[string][]LayoutRow {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string][]LayoutRow, len(input))
	for area, rows := range input {
		mapped := make([]LayoutRow, 0, len(rows))
		for _, row := range rows {
			if len(row.Widgets) == 0 {
				continue
			}
			slots := make([]WidgetSlot, 0, len(row.Widgets))
			for _, widget := range row.Widgets {
				if widget.ID == "" {
					continue
				}
				slots = append(slots, WidgetSlot{
					ID:    widget.ID,
					Width: widget.Width,
				})
			}
			if len(slots) == 0 {
				continue
			}
			mapped = append(mapped, LayoutRow{Widgets: slots})
		}
		if len(mapped) > 0 {
			output[area] = mapped
		}
	}
	return output
}
