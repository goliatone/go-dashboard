package dashboard

import (
	"context"
	"errors"
	"time"
)

var defaultAreas = []string{
	"admin.dashboard.main",
	"admin.dashboard.sidebar",
	"admin.dashboard.footer",
}

var (
	errMissingWidgetStore = errors.New("dashboard: widget store not configured")
	errInvalidArea        = errors.New("dashboard: area code is required")
	errInvalidDefinition  = errors.New("dashboard: definition id is required")
)

// Options configures the dashboard Service. Every collaborator is provided via
// interface so applications can swap implementations without importing internal
// go-dashboard packages.
type Options struct {
	WidgetStore     WidgetStore
	Authorizer      Authorizer
	PreferenceStore PreferenceStore
	Providers       ProviderRegistry
	ConfigValidator ConfigValidator
	RefreshHook     RefreshHook
	Telemetry       Telemetry
	Areas           []string
}

// Service orchestrates dashboard widgets on top of go-cms.
type Service struct {
	opts Options
}

// NewService builds a Service instance with safe defaults.
func NewService(opts Options) *Service {
	if opts.Authorizer == nil {
		opts.Authorizer = allowAllAuthorizer{}
	}
	if opts.RefreshHook == nil {
		opts.RefreshHook = noopRefreshHook{}
	}
	if opts.Providers == nil {
		opts.Providers = NewRegistry()
	}
	if opts.ConfigValidator == nil {
		opts.ConfigValidator = NewJSONSchemaValidator()
	}
	opts.Telemetry = normalizeTelemetry(opts.Telemetry)
	if opts.PreferenceStore == nil {
		opts.PreferenceStore = NewInMemoryPreferenceStore()
	}
	return &Service{opts: opts}
}

// AddWidgetRequest captures the data required to create widget assignments.
type AddWidgetRequest struct {
	DefinitionID  string
	AreaCode      string
	Configuration map[string]any
	Position      *int
	Roles         []string
	StartAt       *time.Time
	EndAt         *time.Time
	UserID        string
}

// AddWidget creates a widget instance and assigns it to an area.
func (s *Service) AddWidget(ctx context.Context, req AddWidgetRequest) error {
	store, err := s.widgetStore()
	if err != nil {
		return err
	}
	if req.AreaCode == "" {
		return errInvalidArea
	}
	if req.DefinitionID == "" {
		return errInvalidDefinition
	}
	if err := s.validateConfiguration(req.DefinitionID, req.Configuration); err != nil {
		return err
	}
	instance, err := store.CreateInstance(ctx, CreateWidgetInstanceInput{
		DefinitionID:  req.DefinitionID,
		Configuration: req.Configuration,
		Visibility: WidgetVisibility{
			Roles:   req.Roles,
			StartAt: req.StartAt,
			EndAt:   req.EndAt,
		},
		Metadata: map[string]any{
			"user_id": req.UserID,
		},
	})
	if err != nil {
		return err
	}
	if err := store.AssignInstance(ctx, AssignWidgetInput{
		AreaCode:   req.AreaCode,
		InstanceID: instance.ID,
		Position:   req.Position,
	}); err != nil {
		return err
	}
	event := WidgetEvent{
		AreaCode: req.AreaCode,
		Instance: instance,
		Reason:   "add",
	}
	if err := s.opts.RefreshHook.WidgetUpdated(ctx, event); err != nil {
		return err
	}
	s.recordTelemetry(ctx, "dashboard.widget.add", map[string]any{
		"area_code":     req.AreaCode,
		"definition_id": req.DefinitionID,
	})
	return nil
}

func (s *Service) recordTelemetry(ctx context.Context, event string, payload map[string]any) {
	s.opts.Telemetry.Record(ctx, event, payload)
}

// RemoveWidget deletes the widget instance.
func (s *Service) RemoveWidget(ctx context.Context, widgetID string) error {
	store, err := s.widgetStore()
	if err != nil {
		return err
	}
	if widgetID == "" {
		return errors.New("dashboard: widget id is required")
	}
	if err := store.DeleteInstance(ctx, widgetID); err != nil {
		return err
	}
	if err := s.opts.RefreshHook.WidgetUpdated(ctx, WidgetEvent{
		Instance: WidgetInstance{ID: widgetID},
		Reason:   "delete",
	}); err != nil {
		return err
	}
	s.recordTelemetry(ctx, "dashboard.widget.remove", map[string]any{"widget_id": widgetID})
	return nil
}

// ReorderWidgets changes widget ordering within an area.
func (s *Service) ReorderWidgets(ctx context.Context, areaCode string, widgetIDs []string) error {
	store, err := s.widgetStore()
	if err != nil {
		return err
	}
	if areaCode == "" {
		return errInvalidArea
	}
	if err := store.ReorderArea(ctx, ReorderAreaInput{
		AreaCode:  areaCode,
		WidgetIDs: widgetIDs,
	}); err != nil {
		return err
	}
	if err := s.opts.RefreshHook.WidgetUpdated(ctx, WidgetEvent{
		AreaCode: areaCode,
		Reason:   "reorder",
	}); err != nil {
		return err
	}
	s.recordTelemetry(ctx, "dashboard.widget.reorder", map[string]any{
		"area_code": areaCode,
		"count":     len(widgetIDs),
	})
	return nil
}

// ConfigureLayout resolves widgets for each dashboard area respecting preferences + auth.
func (s *Service) ConfigureLayout(ctx context.Context, viewer ViewerContext) (Layout, error) {
	store, err := s.widgetStore()
	if err != nil {
		return Layout{}, err
	}
	overrides, err := s.opts.PreferenceStore.LayoutOverrides(ctx, viewer)
	if err != nil {
		return Layout{}, err
	}
	layout := Layout{Areas: make(map[string][]WidgetInstance)}
	for _, area := range s.areaList() {
		resolved, err := store.ResolveArea(ctx, ResolveAreaInput{
			AreaCode: area,
			Audience: viewer.Roles,
			Locale:   viewer.Locale,
		})
		if err != nil {
			return Layout{}, err
		}
		for i := range resolved.Widgets {
			resolved.Widgets[i].AreaCode = area
		}
		filtered := s.filterAuthorized(ctx, viewer, resolved.Widgets)
		ordered := applyOrderOverride(filtered, overrides.AreaOrder[area])
		layout.Areas[area] = applyHiddenFilter(ordered, overrides.HiddenWidgets)
	}
	s.recordTelemetry(ctx, "dashboard.layout.resolve", map[string]any{
		"viewer": viewer.UserID,
	})
	return layout, nil
}

// ResolveArea retrieves a single area layout for the viewer.
func (s *Service) ResolveArea(ctx context.Context, viewer ViewerContext, areaCode string) (ResolvedArea, error) {
	store, err := s.widgetStore()
	if err != nil {
		return ResolvedArea{}, err
	}
	resolved, err := store.ResolveArea(ctx, ResolveAreaInput{
		AreaCode: areaCode,
		Audience: viewer.Roles,
		Locale:   viewer.Locale,
	})
	if err != nil {
		return ResolvedArea{}, err
	}
	resolved.Widgets = s.filterAuthorized(ctx, viewer, resolved.Widgets)
	s.recordTelemetry(ctx, "dashboard.area.resolve", map[string]any{
		"viewer":   viewer.UserID,
		"areaCode": areaCode,
	})
	return resolved, nil
}

func (s *Service) widgetStore() (WidgetStore, error) {
	if s.opts.WidgetStore == nil {
		return nil, errMissingWidgetStore
	}
	return s.opts.WidgetStore, nil
}

func (s *Service) validateConfiguration(definitionID string, config map[string]any) error {
	if s.opts.ConfigValidator == nil || s.opts.Providers == nil {
		return nil
	}
	def, ok := s.opts.Providers.Definition(definitionID)
	if !ok {
		return nil
	}
	return s.opts.ConfigValidator.Validate(def, config)
}

func (s *Service) areaList() []string {
	if len(s.opts.Areas) > 0 {
		return s.opts.Areas
	}
	return defaultAreas
}

func (s *Service) filterAuthorized(ctx context.Context, viewer ViewerContext, widgets []WidgetInstance) []WidgetInstance {
	if len(widgets) == 0 {
		return widgets
	}
	var filtered []WidgetInstance
	for _, w := range widgets {
		if s.opts.Authorizer.CanViewWidget(ctx, viewer, w) {
			filtered = append(filtered, w)
		}
	}
	return s.attachProviderData(ctx, viewer, filtered)
}

func (s *Service) attachProviderData(ctx context.Context, viewer ViewerContext, widgets []WidgetInstance) []WidgetInstance {
	if len(widgets) == 0 || s.opts.Providers == nil {
		return widgets
	}
	enriched := make([]WidgetInstance, len(widgets))
	copy(enriched, widgets)
	for i, inst := range enriched {
		provider, ok := s.opts.Providers.Provider(inst.DefinitionID)
		if !ok || provider == nil {
			continue
		}
		data, err := provider.Fetch(ctx, WidgetContext{
			Instance: inst,
			Viewer:   viewer,
		})
		if err != nil {
			s.recordTelemetry(ctx, "dashboard.widget.provider_error", map[string]any{
				"definition_id": inst.DefinitionID,
				"error":         err.Error(),
			})
			continue
		}
		if enriched[i].Metadata == nil {
			enriched[i].Metadata = map[string]any{}
		}
		enriched[i].Metadata["data"] = data
	}
	return enriched
}

// NotifyWidgetUpdated exposes refresh hook invocation for commands/transports.
func (s *Service) NotifyWidgetUpdated(ctx context.Context, event WidgetEvent) error {
	if err := s.opts.RefreshHook.WidgetUpdated(ctx, event); err != nil {
		return err
	}
	s.recordTelemetry(ctx, "dashboard.widget.event", map[string]any{
		"area_code": event.AreaCode,
		"widget_id": event.Instance.ID,
		"reason":    event.Reason,
	})
	return nil
}

// SavePreferences persists per-viewer layout overrides.
func (s *Service) SavePreferences(ctx context.Context, viewer ViewerContext, overrides LayoutOverrides) error {
	if viewer.UserID == "" {
		return errors.New("dashboard: viewer context missing user id")
	}
	s.normalizeOverrides(&overrides)
	return s.opts.PreferenceStore.SaveLayoutOverrides(ctx, viewer, overrides)
}

func (s *Service) normalizeOverrides(overrides *LayoutOverrides) {
	if overrides.AreaOrder == nil {
		overrides.AreaOrder = map[string][]string{}
	}
	if overrides.HiddenWidgets == nil {
		overrides.HiddenWidgets = map[string]bool{}
	}
}

type allowAllAuthorizer struct{}

func (allowAllAuthorizer) CanViewWidget(context.Context, ViewerContext, WidgetInstance) bool {
	return true
}

type noopRefreshHook struct{}

func (noopRefreshHook) WidgetUpdated(context.Context, WidgetEvent) error {
	return nil
}
