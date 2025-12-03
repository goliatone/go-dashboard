package dashboard

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/goliatone/go-dashboard/pkg/activity"
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
	ThemeProvider   ThemeProvider
	ThemeSelector   ThemeSelectorFunc
	Areas           []string
	Translation     TranslationService
	ScriptNonce     func(context.Context) string
	ActivityHooks   activity.Hooks
	ActivityConfig  activity.Config
	ActivityFeed    ActivityFeed
}

// Service orchestrates dashboard widgets on top of go-cms.
type Service struct {
	opts Options
	act  *activity.Emitter
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
	svc := &Service{
		opts: opts,
		act:  newActivityEmitter(opts),
	}
	if opts.Providers != nil && opts.ActivityFeed != nil {
		_ = opts.Providers.RegisterProvider("admin.widget.recent_activity", newRecentActivityProvider(opts.ActivityFeed))
	}
	return svc
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
	ActorID       string `json:"actor_id,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"`
	Locale        string
}

// UpdateWidgetRequest captures mutable widget fields.
type UpdateWidgetRequest struct {
	Configuration map[string]any
	Metadata      map[string]any
	ActorID       string `json:"actor_id,omitempty"`
	UserID        string `json:"user_id,omitempty"`
	TenantID      string `json:"tenant_id,omitempty"`
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
	metadata := map[string]any{
		"user_id": req.UserID,
	}
	if req.Locale != "" {
		metadata["locale"] = req.Locale
	}
	instance, err := store.CreateInstance(ctx, CreateWidgetInstanceInput{
		DefinitionID:  req.DefinitionID,
		Configuration: req.Configuration,
		Visibility: WidgetVisibility{
			Roles:   req.Roles,
			StartAt: req.StartAt,
			EndAt:   req.EndAt,
		},
		Metadata: metadata,
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
	actCtx := resolveActivityContext(ctx, ActivityContext{
		ActorID:  req.ActorID,
		UserID:   req.UserID,
		TenantID: req.TenantID,
	})
	s.emitActivity(ctx, activity.Event{
		Verb:       "dashboard.widget.add",
		ActorID:    actCtx.ActorID,
		UserID:     actCtx.UserID,
		TenantID:   actCtx.TenantID,
		ObjectType: "widget_instance",
		ObjectID:   instance.ID,
		Metadata: map[string]any{
			"area_code":     req.AreaCode,
			"definition_id": req.DefinitionID,
			"widget_id":     instance.ID,
			"position":      req.Position,
			"roles":         req.Roles,
			"locale":        req.Locale,
			"reason":        "add",
		},
		OccurredAt: time.Now(),
	})
	return nil
}

func (s *Service) recordTelemetry(ctx context.Context, event string, payload map[string]any) {
	s.opts.Telemetry.Record(ctx, event, payload)
}

func (s *Service) emitActivity(ctx context.Context, event activity.Event) {
	if s.act == nil || !s.act.Enabled() {
		return
	}
	_ = s.act.Emit(ctx, event)
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
	instance, err := store.GetInstance(ctx, widgetID)
	if err != nil {
		return err
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
	actCtx := resolveActivityContext(ctx, ActivityContext{})
	s.emitActivity(ctx, activity.Event{
		Verb:       "dashboard.widget.remove",
		ActorID:    actCtx.ActorID,
		UserID:     actCtx.UserID,
		TenantID:   actCtx.TenantID,
		ObjectType: "widget_instance",
		ObjectID:   widgetID,
		Metadata: map[string]any{
			"area_code":     instance.AreaCode,
			"definition_id": instance.DefinitionID,
			"widget_id":     widgetID,
			"reason":        "delete",
		},
		OccurredAt: time.Now(),
	})
	return nil
}

// UpdateWidget mutates widget configuration and/or metadata.
func (s *Service) UpdateWidget(ctx context.Context, widgetID string, req UpdateWidgetRequest) error {
	store, err := s.widgetStore()
	if err != nil {
		return err
	}
	if widgetID == "" {
		return errors.New("dashboard: widget id is required")
	}
	current, err := store.GetInstance(ctx, widgetID)
	if err != nil {
		return err
	}
	updateInput := UpdateWidgetInstanceInput{InstanceID: widgetID}
	if req.Configuration != nil {
		if err := s.validateConfiguration(current.DefinitionID, req.Configuration); err != nil {
			return err
		}
		updateInput.Configuration = req.Configuration
	}
	if req.Metadata != nil {
		updateInput.Metadata = req.Metadata
	}
	updated, err := store.UpdateInstance(ctx, updateInput)
	if err != nil {
		return err
	}
	if updated.ID == "" {
		updated = current
		if req.Configuration != nil {
			updated.Configuration = req.Configuration
		}
		if req.Metadata != nil {
			updated.Metadata = req.Metadata
		}
	}
	if updated.AreaCode == "" {
		updated.AreaCode = current.AreaCode
	}
	event := WidgetEvent{AreaCode: updated.AreaCode, Instance: updated, Reason: "update"}
	if err := s.opts.RefreshHook.WidgetUpdated(ctx, event); err != nil {
		return err
	}
	s.recordTelemetry(ctx, "dashboard.widget.update", map[string]any{
		"widget_id":     widgetID,
		"definition_id": current.DefinitionID,
	})
	actCtx := resolveActivityContext(ctx, ActivityContext{
		ActorID:  req.ActorID,
		UserID:   req.UserID,
		TenantID: req.TenantID,
	})
	s.emitActivity(ctx, activity.Event{
		Verb:       "dashboard.widget.update",
		ActorID:    actCtx.ActorID,
		UserID:     actCtx.UserID,
		TenantID:   actCtx.TenantID,
		ObjectType: "widget_instance",
		ObjectID:   widgetID,
		Metadata: map[string]any{
			"definition_id": current.DefinitionID,
			"area_code":     updated.AreaCode,
			"widget_id":     widgetID,
			"reason":        "update",
		},
		OccurredAt: time.Now(),
	})
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
	actCtx := resolveActivityContext(ctx, ActivityContext{})
	s.emitActivity(ctx, activity.Event{
		Verb:       "dashboard.widget.reorder",
		ActorID:    actCtx.ActorID,
		UserID:     actCtx.UserID,
		TenantID:   actCtx.TenantID,
		ObjectType: "widget_instance",
		ObjectID:   areaCode,
		Metadata: map[string]any{
			"area_code": areaCode,
			"count":     len(widgetIDs),
			"reason":    "reorder",
		},
		OccurredAt: time.Now(),
	})
	return nil
}

// ConfigureLayout resolves widgets for each dashboard area respecting preferences + auth.
func (s *Service) ConfigureLayout(ctx context.Context, viewer ViewerContext) (Layout, error) {
	store, err := s.widgetStore()
	if err != nil {
		return Layout{}, err
	}
	theme := s.resolveTheme(ctx, viewer)
	overrides, err := s.opts.PreferenceStore.LayoutOverrides(ctx, viewer)
	if err != nil {
		return Layout{}, err
	}
	layout := Layout{
		Areas: make(map[string][]WidgetInstance),
		Theme: theme,
	}
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
		filtered := s.filterAuthorized(ctx, viewer, theme, resolved.Widgets)
		ordered := applyOrderOverride(filtered, overrides.AreaOrder[area])
		withLayout := applyRowMetadata(ordered, overrides.AreaRows[area])
		layout.Areas[area] = applyHiddenFilter(withLayout, overrides.HiddenWidgets)
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
	theme := s.resolveTheme(ctx, viewer)
	resolved, err := store.ResolveArea(ctx, ResolveAreaInput{
		AreaCode: areaCode,
		Audience: viewer.Roles,
		Locale:   viewer.Locale,
	})
	if err != nil {
		return ResolvedArea{}, err
	}
	resolved.Widgets = s.filterAuthorized(ctx, viewer, theme, resolved.Widgets)
	overrides, err := s.opts.PreferenceStore.LayoutOverrides(ctx, viewer)
	if err == nil {
		ordered := applyOrderOverride(resolved.Widgets, overrides.AreaOrder[areaCode])
		resolved.Widgets = applyRowMetadata(ordered, overrides.AreaRows[areaCode])
		resolved.Widgets = applyHiddenFilter(resolved.Widgets, overrides.HiddenWidgets)
	}
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

func (s *Service) resolveTheme(ctx context.Context, viewer ViewerContext) *ThemeSelection {
	if s.opts.ThemeProvider == nil {
		return nil
	}
	selector := ThemeSelector{}
	if s.opts.ThemeSelector != nil {
		selector = s.opts.ThemeSelector(ctx, viewer)
	}
	theme, err := s.opts.ThemeProvider.SelectTheme(ctx, selector)
	if err != nil {
		s.recordTelemetry(ctx, "dashboard.theme.resolve_error", map[string]any{
			"theme":   selector.Name,
			"variant": selector.Variant,
			"error":   err.Error(),
		})
		return nil
	}
	return cloneThemeSelection(theme)
}

func (s *Service) filterAuthorized(ctx context.Context, viewer ViewerContext, theme *ThemeSelection, widgets []WidgetInstance) []WidgetInstance {
	if len(widgets) == 0 {
		return widgets
	}
	var filtered []WidgetInstance
	for _, w := range widgets {
		if s.opts.Authorizer.CanViewWidget(ctx, viewer, w) {
			filtered = append(filtered, w)
		}
	}
	return s.attachProviderData(ctx, viewer, theme, filtered)
}

func (s *Service) attachProviderData(ctx context.Context, viewer ViewerContext, theme *ThemeSelection, widgets []WidgetInstance) []WidgetInstance {
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
		var options map[string]any
		if s.opts.ScriptNonce != nil {
			if nonce := strings.TrimSpace(s.opts.ScriptNonce(ctx)); nonce != "" {
				options = map[string]any{
					scriptNonceOptionKey: nonce,
				}
			}
		}
		data, err := provider.Fetch(ctx, WidgetContext{
			Instance:   inst,
			Viewer:     viewer,
			Translator: s.opts.Translation,
			Options:    options,
			Theme:      theme,
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
	actCtx := resolveActivityContext(ctx, ActivityContext{})
	s.emitActivity(ctx, activity.Event{
		Verb:       "dashboard.widget.event",
		ActorID:    actCtx.ActorID,
		UserID:     actCtx.UserID,
		TenantID:   actCtx.TenantID,
		ObjectType: "widget_instance",
		ObjectID:   event.Instance.ID,
		Metadata: map[string]any{
			"area_code": event.AreaCode,
			"widget_id": event.Instance.ID,
			"reason":    event.Reason,
		},
		OccurredAt: time.Now(),
	})
	return nil
}

// SavePreferences persists per-viewer layout overrides.
func (s *Service) SavePreferences(ctx context.Context, viewer ViewerContext, overrides LayoutOverrides) error {
	if viewer.UserID == "" {
		return errors.New("dashboard: viewer context missing user id")
	}
	if overrides.Locale == "" {
		overrides.Locale = viewer.Locale
	}
	s.normalizeOverrides(&overrides)
	if err := s.opts.PreferenceStore.SaveLayoutOverrides(ctx, viewer, overrides); err != nil {
		return err
	}
	actCtx := resolveActivityContext(ctx, ActivityContext{
		ActorID: viewer.UserID,
		UserID:  viewer.UserID,
	})
	s.emitActivity(ctx, activity.Event{
		Verb:       "dashboard.preferences.save",
		ActorID:    actCtx.ActorID,
		UserID:     actCtx.UserID,
		TenantID:   actCtx.TenantID,
		ObjectType: "dashboard_preferences",
		ObjectID:   viewer.UserID,
		Metadata: map[string]any{
			"locale":        overrides.Locale,
			"areas":         len(overrides.AreaOrder),
			"hidden_count":  len(overrides.HiddenWidgets),
			"viewer_roles":  viewer.Roles,
			"viewer_locale": viewer.Locale,
		},
		OccurredAt: time.Now(),
	})
	return nil
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

func newActivityEmitter(opts Options) *activity.Emitter {
	cfg := opts.ActivityConfig
	return activity.NewEmitter(cloneActivityHooks(opts.ActivityHooks), cfg)
}

func cloneActivityHooks(hooks activity.Hooks) activity.Hooks {
	if len(hooks) == 0 {
		return nil
	}
	normalized := make([]activity.ActivityHook, 0, len(hooks))
	for _, hook := range hooks {
		if hook == nil {
			continue
		}
		normalized = append(normalized, hook)
	}
	if len(normalized) == 0 {
		return nil
	}
	return activity.Hooks(normalized)
}

func resolveActivityContext(ctx context.Context, fallback ActivityContext) ActivityContext {
	meta := activityContextFrom(ctx)
	if meta.ActorID == "" {
		meta.ActorID = fallback.ActorID
	}
	if meta.UserID == "" {
		meta.UserID = fallback.UserID
	}
	if meta.TenantID == "" {
		meta.TenantID = fallback.TenantID
	}
	return meta
}
