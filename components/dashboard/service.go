package dashboard

import "context"

// WidgetStore describes the minimal contract the Service requires from go-cms.
// Concrete implementations will wrap the real widget service once Phase 1 lands.
type WidgetStore interface{}

// Authorizer guards widget visibility for the active viewer.
type Authorizer interface{}

// PreferenceStore provides per-user overrides for widget placement.
type PreferenceStore interface{}

// ProviderRegistry maps widget definition codes to data providers.
type ProviderRegistry interface{}

// RefreshHook notifies transports about widget updates or refresh requests.
type RefreshHook interface{}

// Options configures the dashboard Service.
type Options struct {
	WidgetStore     WidgetStore
	Authorizer      Authorizer
	PreferenceStore PreferenceStore
	Providers       ProviderRegistry
	RefreshHook     RefreshHook
}

// Service is the façade go-admin will use to orchestrate dashboard behavior.
type Service struct {
	opts Options
}

// NewService builds a Service instance. Future tasks will validate options
// and wire default implementations.
func NewService(opts Options) *Service {
	return &Service{opts: opts}
}

// AddWidgetRequest captures the data required to create widget assignments.
type AddWidgetRequest struct {
	DefinitionID  string
	AreaCode      string
	Configuration map[string]any
	Position      *int
	Roles         []string
	StartDate     *string
	EndDate       *string
	UserID        string
}

// AddWidget is a placeholder to satisfy go-admin build wiring during Phase 0.
func (s *Service) AddWidget(ctx context.Context, req AddWidgetRequest) error {
	return nil
}

// RemoveWidget removes widgets from dashboard areas. Implementation follows later.
func (s *Service) RemoveWidget(ctx context.Context, widgetID string) error {
	return nil
}

// ReorderWidgets changes widget ordering within an area.
func (s *Service) ReorderWidgets(ctx context.Context, areaCode string, widgetIDs []string) error {
	return nil
}
