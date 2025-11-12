package dashboard

import (
	"context"
	"time"
)

// WidgetStore encapsulates the persistence + orchestration hooks provided by go-cms.
// Implementations ensure thread safety and idempotency.
type WidgetStore interface {
	EnsureArea(ctx context.Context, def WidgetAreaDefinition) (bool, error)
	EnsureDefinition(ctx context.Context, def WidgetDefinition) (bool, error)
	CreateInstance(ctx context.Context, input CreateWidgetInstanceInput) (WidgetInstance, error)
	DeleteInstance(ctx context.Context, instanceID string) error
	AssignInstance(ctx context.Context, input AssignWidgetInput) error
	ReorderArea(ctx context.Context, input ReorderAreaInput) error
	ResolveArea(ctx context.Context, input ResolveAreaInput) (ResolvedArea, error)
}

// Authorizer determines if a viewer can see a widget instance.
type Authorizer interface {
	CanViewWidget(ctx context.Context, viewer ViewerContext, instance WidgetInstance) bool
}

// PreferenceStore returns layout overrides per viewer.
type PreferenceStore interface {
	LayoutOverrides(ctx context.Context, viewer ViewerContext) (LayoutOverrides, error)
	SaveLayoutOverrides(ctx context.Context, viewer ViewerContext, overrides LayoutOverrides) error
}

// ProviderRegistry stores widget definitions/providers discoverable via hooks or manifests.
type ProviderRegistry interface {
	RegisterDefinition(def WidgetDefinition) error
	RegisterProvider(code string, provider Provider) error
	Definition(code string) (WidgetDefinition, bool)
	Provider(code string) (Provider, bool)
	Definitions() []WidgetDefinition
}

// RefreshHook notifies transports (REST/WebSocket) about widget changes.
type RefreshHook interface {
	WidgetUpdated(ctx context.Context, event WidgetEvent) error
}

// WidgetAreaDefinition models a dashboard widget area (main/sidebar/footer).
type WidgetAreaDefinition struct {
	Code        string
	Name        string
	Description string
}

// WidgetDefinition describes a widget schema stored within go-cms.
type WidgetDefinition struct {
	Code        string
	Name        string
	Description string
	Schema      map[string]any
	Category    string
}

// WidgetInstance represents a widget instance stored in go-cms.
type WidgetInstance struct {
	ID            string
	DefinitionID  string
	AreaCode      string
	Configuration map[string]any
	Metadata      map[string]any
}

// CreateWidgetInstanceInput configures new instances.
type CreateWidgetInstanceInput struct {
	DefinitionID  string
	Configuration map[string]any
	Visibility    WidgetVisibility
	Metadata      map[string]any
}

// WidgetVisibility defines runtime visibility constraints.
type WidgetVisibility struct {
	Roles    []string
	StartAt  *time.Time
	EndAt    *time.Time
	Audience []string
}

// AssignWidgetInput associates a widget instance with an area.
type AssignWidgetInput struct {
	AreaCode   string
	InstanceID string
	Position   *int
}

// ReorderAreaInput represents a new ordering for widgets within an area.
type ReorderAreaInput struct {
	AreaCode  string
	WidgetIDs []string
}

// ResolveAreaInput requests widget instances for a given area and audience.
type ResolveAreaInput struct {
	AreaCode string
	Audience []string
	Locale   string
}

// ResolvedArea is a container for widgets returned by the store.
type ResolvedArea struct {
	AreaCode string
	Widgets  []WidgetInstance
}

// LayoutOverrides captures per-user adjustments.
type LayoutOverrides struct {
	AreaOrder map[string][]string
}

// ViewerContext captures the active user/locale information needed to render dashboards.
type ViewerContext struct {
	UserID string
	Roles  []string
	Locale string
}

// Layout describes the resolved widget instances per dashboard area.
type Layout struct {
	Areas map[string][]WidgetInstance
}

// WidgetEvent describes changes that transports might care about.
type WidgetEvent struct {
	AreaCode string
	Instance WidgetInstance
	Reason   string
}
