package dashboard

import (
	"context"
	"errors"
	"sort"
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

// SaveLayoutPreferencesInput is the canonical typed transport contract for
// persisting dashboard layout overrides.
type SaveLayoutPreferencesInput struct {
	Viewer        ViewerContext               `json:"viewer"`
	AreaOrder     map[string][]string         `json:"area_order"`
	LayoutRows    map[string][]LayoutRowInput `json:"layout_rows"`
	HiddenWidgets []string                    `json:"hidden_widget_ids"`
}

// LegacyLayoutPreferencesInput is a temporary migration adapter for historical
// layout-array preference payloads. Remove once callers stop sending
// `{"layout":[...]}` bodies.
type LegacyLayoutPreferencesInput struct {
	Layout []LegacyLayoutWidgetInput `json:"layout"`
}

// LegacyLayoutWidgetInput captures the legacy layout-only transport shape.
type LegacyLayoutWidgetInput struct {
	ID       string `json:"id"`
	Area     string `json:"area,omitempty"`
	AreaCode string `json:"area_code,omitempty"`
	Position int    `json:"position,omitempty"`
	Span     int    `json:"span,omitempty"`
	Width    int    `json:"width,omitempty"`
	Hidden   bool   `json:"hidden,omitempty"`
	Locale   string `json:"locale,omitempty"`
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

// ToSaveLayoutPreferencesInput converts the legacy layout-array transport shape
// into the canonical typed preference input.
func (input LegacyLayoutPreferencesInput) ToSaveLayoutPreferencesInput(viewer ViewerContext) SaveLayoutPreferencesInput {
	type placedWidget struct {
		index  int
		id     string
		width  int
		hidden bool
		pos    int
	}
	grouped := map[string][]placedWidget{}
	hiddenIDs := make([]string, 0, len(input.Layout))
	for idx, widget := range input.Layout {
		areaCode := widget.AreaCode
		if areaCode == "" {
			areaCode = widget.Area
		}
		if widget.ID == "" || areaCode == "" {
			continue
		}
		width := widget.Width
		if width <= 0 {
			width = widget.Span
		}
		if width <= 0 {
			width = 12
		}
		if viewer.Locale == "" && widget.Locale != "" {
			viewer.Locale = widget.Locale
		}
		grouped[areaCode] = append(grouped[areaCode], placedWidget{
			index:  idx,
			id:     widget.ID,
			width:  width,
			hidden: widget.Hidden,
			pos:    widget.Position,
		})
		if widget.Hidden {
			hiddenIDs = append(hiddenIDs, widget.ID)
		}
	}

	areaOrder := make(map[string][]string, len(grouped))
	layoutRows := make(map[string][]LayoutRowInput, len(grouped))
	for areaCode, widgets := range grouped {
		sort.SliceStable(widgets, func(i, j int) bool {
			if widgets[i].pos == widgets[j].pos {
				return widgets[i].index < widgets[j].index
			}
			return widgets[i].pos < widgets[j].pos
		})
		orderedIDs := make([]string, 0, len(widgets))
		rows := make([]LayoutRowInput, 0, len(widgets))
		for _, widget := range widgets {
			orderedIDs = append(orderedIDs, widget.id)
			rows = append(rows, LayoutRowInput{
				Widgets: []LayoutWidgetInput{{
					ID:    widget.id,
					Width: widget.width,
				}},
			})
		}
		areaOrder[areaCode] = orderedIDs
		layoutRows[areaCode] = rows
	}

	return SaveLayoutPreferencesInput{
		Viewer:        viewer,
		AreaOrder:     areaOrder,
		LayoutRows:    layoutRows,
		HiddenWidgets: hiddenIDs,
	}
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
