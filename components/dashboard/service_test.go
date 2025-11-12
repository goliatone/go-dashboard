package dashboard

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestConfigureLayoutFiltersByAuthorizer(t *testing.T) {
	store := &fakeWidgetStore{
		resolved: map[string][]WidgetInstance{
			"admin.dashboard.main": {
				{ID: "w1", DefinitionID: "admin.widget.user_stats"},
				{ID: "w2", DefinitionID: "admin.widget.user_stats"},
			},
		},
	}
	auth := allowListAuthorizer{allowed: map[string]bool{"w2": true}}
	service := NewService(Options{
		WidgetStore:     store,
		Authorizer:      auth,
		PreferenceStore: NewInMemoryPreferenceStore(),
	})
	layout, err := service.ConfigureLayout(context.Background(), ViewerContext{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ConfigureLayout returned error: %v", err)
	}
	if len(layout.Areas["admin.dashboard.main"]) != 1 || layout.Areas["admin.dashboard.main"][0].ID != "w2" {
		t.Fatalf("expected filtered widget, got %#v", layout.Areas["admin.dashboard.main"])
	}
}

func TestConfigureLayoutAppliesHiddenOverrides(t *testing.T) {
	store := &fakeWidgetStore{
		resolved: map[string][]WidgetInstance{
			"admin.dashboard.main": {
				{ID: "w1", DefinitionID: "admin.widget.user_stats"},
				{ID: "w2", DefinitionID: "admin.widget.user_stats"},
			},
		},
	}
	prefs := NewInMemoryPreferenceStore()
	viewer := ViewerContext{UserID: "user-3"}
	_ = prefs.SaveLayoutOverrides(context.Background(), viewer, LayoutOverrides{
		AreaOrder:     map[string][]string{"admin.dashboard.main": {"w1", "w2"}},
		HiddenWidgets: map[string]bool{"w2": true},
	})
	service := NewService(Options{
		WidgetStore:     store,
		PreferenceStore: prefs,
	})
	layout, err := service.ConfigureLayout(context.Background(), viewer)
	if err != nil {
		t.Fatalf("ConfigureLayout returned error: %v", err)
	}
	widgets := layout.Areas["admin.dashboard.main"]
	if len(widgets) != 1 || widgets[0].ID != "w1" {
		t.Fatalf("expected hidden widget filtered, got %#v", widgets)
	}
}

func TestConfigureLayoutAppliesPreferenceOverrides(t *testing.T) {
	store := &fakeWidgetStore{
		resolved: map[string][]WidgetInstance{
			"admin.dashboard.main": {
				{ID: "w1", DefinitionID: "admin.widget.user_stats"},
				{ID: "w2", DefinitionID: "admin.widget.user_stats"},
			},
		},
	}
	prefs := NewInMemoryPreferenceStore()
	viewer := ViewerContext{UserID: "user-2"}
	_ = prefs.SaveLayoutOverrides(context.Background(), viewer, LayoutOverrides{
		AreaOrder: map[string][]string{"admin.dashboard.main": {"w2", "w1"}},
	})
	service := NewService(Options{
		WidgetStore:     store,
		PreferenceStore: prefs,
	})
	layout, err := service.ConfigureLayout(context.Background(), viewer)
	if err != nil {
		t.Fatalf("ConfigureLayout returned error: %v", err)
	}
	order := layout.Areas["admin.dashboard.main"]
	if len(order) != 2 || order[0].ID != "w2" {
		t.Fatalf("expected preference order applied, got %#v", order)
	}
}

func TestAddWidgetEmitsRefreshHook(t *testing.T) {
	store := &fakeWidgetStore{
		createInstanceFn: func(input CreateWidgetInstanceInput) (WidgetInstance, error) {
			return WidgetInstance{ID: "instance-1", DefinitionID: input.DefinitionID}, nil
		},
	}
	hook := &collectingHook{}
	service := NewService(Options{
		WidgetStore:     store,
		PreferenceStore: NewInMemoryPreferenceStore(),
		RefreshHook:     hook,
	})
	req := AddWidgetRequest{
		DefinitionID: "admin.widget.user_stats",
		AreaCode:     "admin.dashboard.main",
		Configuration: map[string]any{
			"metric": "total",
		},
		Roles: []string{"admin"},
		StartAt: func() *time.Time {
			now := time.Now().UTC()
			return &now
		}(),
	}
	if err := service.AddWidget(context.Background(), req); err != nil {
		t.Fatalf("AddWidget returned error: %v", err)
	}
	if hook.events != 1 {
		t.Fatalf("expected hook to be invoked, got %d", hook.events)
	}
}

type fakeWidgetStore struct {
	ensureAreaFn      func(def WidgetAreaDefinition) error
	ensureDefinition  func(def WidgetDefinition) error
	createInstanceFn  func(input CreateWidgetInstanceInput) (WidgetInstance, error)
	assignInstanceFn  func(input AssignWidgetInput) error
	reorderAreaFn     func(input ReorderAreaInput) error
	resolveAreaFn     func(input ResolveAreaInput) (ResolvedArea, error)
	resolved          map[string][]WidgetInstance
	assignCalls       []AssignWidgetInput
	reorderCalls      []ReorderAreaInput
	createdDefinition []string
}

func (f *fakeWidgetStore) EnsureArea(ctx context.Context, def WidgetAreaDefinition) (bool, error) {
	if f.ensureAreaFn != nil {
		return true, f.ensureAreaFn(def)
	}
	return true, nil
}

func (f *fakeWidgetStore) EnsureDefinition(ctx context.Context, def WidgetDefinition) (bool, error) {
	if f.ensureDefinition != nil {
		return true, f.ensureDefinition(def)
	}
	f.createdDefinition = append(f.createdDefinition, def.Code)
	return true, nil
}

func (f *fakeWidgetStore) CreateInstance(ctx context.Context, input CreateWidgetInstanceInput) (WidgetInstance, error) {
	if f.createInstanceFn != nil {
		return f.createInstanceFn(input)
	}
	return WidgetInstance{ID: input.DefinitionID + "-instance", DefinitionID: input.DefinitionID}, nil
}

func (f *fakeWidgetStore) DeleteInstance(context.Context, string) error { return nil }

func (f *fakeWidgetStore) AssignInstance(ctx context.Context, input AssignWidgetInput) error {
	f.assignCalls = append(f.assignCalls, input)
	if f.assignInstanceFn != nil {
		return f.assignInstanceFn(input)
	}
	return nil
}

func (f *fakeWidgetStore) ReorderArea(ctx context.Context, input ReorderAreaInput) error {
	f.reorderCalls = append(f.reorderCalls, input)
	if f.reorderAreaFn != nil {
		return f.reorderAreaFn(input)
	}
	return nil
}

func (f *fakeWidgetStore) ResolveArea(ctx context.Context, input ResolveAreaInput) (ResolvedArea, error) {
	if f.resolveAreaFn != nil {
		return f.resolveAreaFn(input)
	}
	if widgets, ok := f.resolved[input.AreaCode]; ok {
		return ResolvedArea{AreaCode: input.AreaCode, Widgets: widgets}, nil
	}
	return ResolvedArea{AreaCode: input.AreaCode, Widgets: []WidgetInstance{}}, nil
}

type allowListAuthorizer struct {
	allowed map[string]bool
}

func (a allowListAuthorizer) CanViewWidget(_ context.Context, _ ViewerContext, instance WidgetInstance) bool {
	return a.allowed[instance.ID]
}

type collectingHook struct {
	events int
}

func (h *collectingHook) WidgetUpdated(context.Context, WidgetEvent) error {
	h.events++
	return nil
}

var _ RefreshHook = (*collectingHook)(nil)

func TestPreferenceStoreRequiresUserID(t *testing.T) {
	store := NewInMemoryPreferenceStore()
	err := store.SaveLayoutOverrides(context.Background(), ViewerContext{}, LayoutOverrides{})
	if err == nil {
		t.Fatalf("expected error when user id missing")
	}
}

func TestPreferenceStoreDefaultOverrides(t *testing.T) {
	store := NewInMemoryPreferenceStore()
	overrides, err := store.LayoutOverrides(context.Background(), ViewerContext{})
	if err != nil {
		t.Fatalf("LayoutOverrides returned error: %v", err)
	}
	if overrides.AreaOrder == nil {
		t.Fatalf("expected default map")
	}
}

func TestNotifyWidgetUpdatedTelemetry(t *testing.T) {
	hook := &collectingHook{}
	telemetry := &testTelemetry{}
	service := NewService(Options{
		WidgetStore: NewInMemoryWidgetStoreStub(),
		RefreshHook: hook,
		Telemetry:   telemetry,
	})
	event := WidgetEvent{AreaCode: "admin.dashboard.main", Instance: WidgetInstance{ID: "w1"}, Reason: "custom"}
	if err := service.NotifyWidgetUpdated(context.Background(), event); err != nil {
		t.Fatalf("NotifyWidgetUpdated returned error: %v", err)
	}
	if telemetry.calls != 1 {
		t.Fatalf("expected telemetry recorded event")
	}
}

// NewInMemoryWidgetStoreStub returns a store that supports Notify tests.
func NewInMemoryWidgetStoreStub() WidgetStore {
	return &fakeWidgetStore{
		createInstanceFn: func(input CreateWidgetInstanceInput) (WidgetInstance, error) {
			return WidgetInstance{ID: input.DefinitionID}, nil
		},
		assignInstanceFn: func(AssignWidgetInput) error { return nil },
		reorderAreaFn:    func(ReorderAreaInput) error { return nil },
		resolveAreaFn: func(input ResolveAreaInput) (ResolvedArea, error) {
			return ResolvedArea{AreaCode: input.AreaCode, Widgets: []WidgetInstance{}}, nil
		},
	}
}

type testTelemetry struct {
	calls int
}

func (t *testTelemetry) Record(context.Context, string, map[string]any) {
	t.calls++
}

func TestAddWidgetValidatesInputs(t *testing.T) {
	service := NewService(Options{WidgetStore: NewInMemoryWidgetStoreStub()})
	err := service.AddWidget(context.Background(), AddWidgetRequest{})
	if !errors.Is(err, errInvalidArea) && err == nil {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestSavePreferencesRequiresUser(t *testing.T) {
	service := NewService(Options{})
	err := service.SavePreferences(context.Background(), ViewerContext{}, LayoutOverrides{})
	if err == nil {
		t.Fatalf("expected error when user missing")
	}
}

func TestSavePreferencesStoresOverrides(t *testing.T) {
	prefs := NewInMemoryPreferenceStore()
	service := NewService(Options{PreferenceStore: prefs})
	viewer := ViewerContext{UserID: "user-4"}
	overrides := LayoutOverrides{
		AreaOrder:     map[string][]string{"admin.dashboard.main": {"w2", "w1"}},
		HiddenWidgets: map[string]bool{"w3": true},
	}
	if err := service.SavePreferences(context.Background(), viewer, overrides); err != nil {
		t.Fatalf("SavePreferences returned error: %v", err)
	}
	stored, err := prefs.LayoutOverrides(context.Background(), viewer)
	if err != nil {
		t.Fatalf("LayoutOverrides returned error: %v", err)
	}
	if !stored.HiddenWidgets["w3"] {
		t.Fatalf("expected hidden widget persisted")
	}
}
