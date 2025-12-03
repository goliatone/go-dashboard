package commands

import (
	"context"
	"fmt"
	"testing"

	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

func TestSeedDashboardCommand(t *testing.T) {
	store := newStubStore()
	reg := &stubRegistry{}
	service := dashboard.NewService(dashboard.Options{WidgetStore: store})
	telemetry := &stubTelemetry{}
	cmd := NewSeedDashboardCommand(store, reg, service, telemetry)
	if err := cmd.Execute(context.Background(), SeedDashboardInput{SeedLayout: true}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if store.ensureAreaCalls != len(dashboard.DefaultAreaDefinitions()) {
		t.Fatalf("expected %d areas, got %d", len(dashboard.DefaultAreaDefinitions()), store.ensureAreaCalls)
	}
	if reg.count != len(dashboard.DefaultWidgetDefinitions()) {
		t.Fatalf("expected registry count %d, got %d", len(dashboard.DefaultWidgetDefinitions()), reg.count)
	}
	if store.assignCalls != len(dashboard.DefaultSeedWidgets()) {
		t.Fatalf("expected %d assign calls, got %d", len(dashboard.DefaultSeedWidgets()), store.assignCalls)
	}
	if telemetry.calls == 0 {
		t.Fatalf("expected telemetry to record events")
	}
}

func TestAssignWidgetCommand(t *testing.T) {
	service := &stubService{}
	cmd := NewAssignWidgetCommand(service, nil)
	req := dashboard.AddWidgetRequest{DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main", ActorID: "actor-1", UserID: "user-1", TenantID: "tenant-1"}
	if err := cmd.Execute(context.Background(), req); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if service.addCalls != 1 {
		t.Fatalf("expected add call")
	}
	if service.lastAddReq.ActorID != "actor-1" || service.lastAddReq.UserID != "user-1" || service.lastAddReq.TenantID != "tenant-1" {
		t.Fatalf("expected actor context forwarded, got %+v", service.lastAddReq)
	}
}

func TestRemoveWidgetCommand(t *testing.T) {
	service := &stubService{}
	cmd := NewRemoveWidgetCommand(service, nil)
	input := RemoveWidgetInput{WidgetID: "widget-1", ActorID: "actor-1", UserID: "user-1", TenantID: "tenant-1"}
	if err := cmd.Execute(context.Background(), input); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if service.removeCalls != 1 {
		t.Fatalf("expected remove call")
	}
	if ctx := service.lastCtx; ctx == nil {
		t.Fatalf("expected context to be set")
	} else {
		meta := dashboard.ViewerContext{}
		if v := ctx.Value(dashboard.ViewerContext{}); v != nil {
			if parsed, ok := v.(dashboard.ViewerContext); ok {
				meta = parsed
			}
		}
		_ = meta
	}
}

func TestReorderWidgetsCommand(t *testing.T) {
	service := &stubService{}
	cmd := NewReorderWidgetsCommand(service, nil)
	if err := cmd.Execute(context.Background(), ReorderWidgetsInput{
		AreaCode:  "admin.dashboard.main",
		WidgetIDs: []string{"w1", "w2"},
	}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if service.reorderCalls != 1 {
		t.Fatalf("expected reorder call")
	}
}

func TestRefreshWidgetCommand(t *testing.T) {
	service := &stubService{}
	cmd := NewRefreshWidgetCommand(service, nil)
	event := dashboard.WidgetEvent{AreaCode: "admin.dashboard.main"}
	if err := cmd.Execute(context.Background(), RefreshWidgetInput{Event: event}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if service.refreshCalls != 1 {
		t.Fatalf("expected refresh call")
	}
}

func TestUpdateWidgetCommand(t *testing.T) {
	service := &stubService{}
	cmd := NewUpdateWidgetCommand(service, nil)
	input := UpdateWidgetInput{WidgetID: "widget-1", Configuration: map[string]any{"title": "Updated"}, ActorID: "actor-1", UserID: "user-1", TenantID: "tenant-1"}
	if err := cmd.Execute(context.Background(), input); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if service.updateCalls != 1 {
		t.Fatalf("expected update call")
	}
	if service.lastUpdateReq.ActorID != "actor-1" || service.lastUpdateReq.UserID != "user-1" || service.lastUpdateReq.TenantID != "tenant-1" {
		t.Fatalf("expected actor context forwarded, got %+v", service.lastUpdateReq)
	}
}

func TestSaveLayoutPreferencesCommand(t *testing.T) {
	service := &stubService{}
	cmd := NewSaveLayoutPreferencesCommand(service, nil)
	input := SaveLayoutPreferencesInput{
		Viewer: dashboard.ViewerContext{UserID: "user-1"},
		AreaOrder: map[string][]string{
			"admin.dashboard.main": {"w2", "w1"},
		},
		LayoutRows: map[string][]LayoutRowInput{
			"admin.dashboard.main": {
				{Widgets: []LayoutWidgetInput{
					{ID: "w2", Width: 6},
					{ID: "w1", Width: 6},
				}},
			},
		},
		HiddenWidgets: []string{"w3"},
	}
	if err := cmd.Execute(context.Background(), input); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if service.savePrefCalls != 1 {
		t.Fatalf("expected preferences save")
	}
	rows := service.lastOverrides.AreaRows["admin.dashboard.main"]
	if len(rows) == 0 || len(rows[0].Widgets) != 2 || rows[0].Widgets[0].Width != 6 {
		t.Fatalf("expected layout rows persisted, got %#v", rows)
	}
}

type stubService struct {
	addCalls      int
	removeCalls   int
	reorderCalls  int
	refreshCalls  int
	updateCalls   int
	savePrefCalls int
	lastOverrides dashboard.LayoutOverrides
	lastCtx       context.Context
	lastAddReq    dashboard.AddWidgetRequest
	lastUpdateReq dashboard.UpdateWidgetRequest
}

func (s *stubService) AddWidget(_ context.Context, req dashboard.AddWidgetRequest) error {
	s.addCalls++
	s.lastAddReq = req
	return nil
}

func (s *stubService) RemoveWidget(ctx context.Context, _ string) error {
	s.removeCalls++
	s.lastCtx = ctx
	return nil
}

func (s *stubService) ReorderWidgets(ctx context.Context, _ string, _ []string) error {
	s.reorderCalls++
	s.lastCtx = ctx
	return nil
}

func (s *stubService) NotifyWidgetUpdated(context.Context, dashboard.WidgetEvent) error {
	s.refreshCalls++
	return nil
}

func (s *stubService) SavePreferences(ctx context.Context, viewer dashboard.ViewerContext, overrides dashboard.LayoutOverrides) error {
	s.savePrefCalls++
	s.lastOverrides = overrides
	s.lastCtx = ctx
	return nil
}

func (s *stubService) UpdateWidget(ctx context.Context, widgetID string, req dashboard.UpdateWidgetRequest) error {
	s.updateCalls++
	s.lastCtx = ctx
	s.lastUpdateReq = req
	return nil
}

type stubRegistry struct {
	count int
}

func (s *stubRegistry) RegisterDefinition(def dashboard.WidgetDefinition) error {
	s.count++
	return nil
}

func (s *stubRegistry) RegisterProvider(string, dashboard.Provider) error { return nil }
func (s *stubRegistry) Definition(string) (dashboard.WidgetDefinition, bool) {
	return dashboard.WidgetDefinition{}, false
}
func (s *stubRegistry) Provider(string) (dashboard.Provider, bool) { return nil, false }
func (s *stubRegistry) Definitions() []dashboard.WidgetDefinition  { return nil }

type stubStore struct {
	ensureAreaCalls int
	assignCalls     int
	instances       map[string]dashboard.WidgetInstance
}

func newStubStore() *stubStore { return &stubStore{} }

func (s *stubStore) EnsureArea(context.Context, dashboard.WidgetAreaDefinition) (bool, error) {
	s.ensureAreaCalls++
	return true, nil
}

func (s *stubStore) EnsureDefinition(context.Context, dashboard.WidgetDefinition) (bool, error) {
	return true, nil
}

func (s *stubStore) CreateInstance(ctx context.Context, input dashboard.CreateWidgetInstanceInput) (dashboard.WidgetInstance, error) {
	inst := dashboard.WidgetInstance{ID: input.DefinitionID + "-instance", DefinitionID: input.DefinitionID}
	if s.instances == nil {
		s.instances = map[string]dashboard.WidgetInstance{}
	}
	s.instances[inst.ID] = inst
	return inst, nil
}

func (s *stubStore) GetInstance(ctx context.Context, id string) (dashboard.WidgetInstance, error) {
	if inst, ok := s.instances[id]; ok {
		return inst, nil
	}
	return dashboard.WidgetInstance{}, fmt.Errorf("instance %s not found", id)
}

func (s *stubStore) DeleteInstance(context.Context, string) error { return nil }

func (s *stubStore) AssignInstance(context.Context, dashboard.AssignWidgetInput) error {
	s.assignCalls++
	return nil
}

func (s *stubStore) UpdateInstance(ctx context.Context, input dashboard.UpdateWidgetInstanceInput) (dashboard.WidgetInstance, error) {
	inst, ok := s.instances[input.InstanceID]
	if !ok {
		return dashboard.WidgetInstance{}, fmt.Errorf("instance %s not found", input.InstanceID)
	}
	if input.Configuration != nil {
		inst.Configuration = input.Configuration
	}
	if input.Metadata != nil {
		inst.Metadata = input.Metadata
	}
	s.instances[input.InstanceID] = inst
	return inst, nil
}

func (s *stubStore) ReorderArea(context.Context, dashboard.ReorderAreaInput) error { return nil }

func (s *stubStore) ResolveArea(context.Context, dashboard.ResolveAreaInput) (dashboard.ResolvedArea, error) {
	return dashboard.ResolvedArea{}, nil
}

type stubTelemetry struct {
	calls int
}

func (s *stubTelemetry) Record(context.Context, string, map[string]any) {
	s.calls++
}
