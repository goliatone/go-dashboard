package commands

import (
	"context"
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
	req := dashboard.AddWidgetRequest{DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main"}
	if err := cmd.Execute(context.Background(), req); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if service.addCalls != 1 {
		t.Fatalf("expected add call")
	}
}

func TestRemoveWidgetCommand(t *testing.T) {
	service := &stubService{}
	cmd := NewRemoveWidgetCommand(service, nil)
	if err := cmd.Execute(context.Background(), RemoveWidgetInput{WidgetID: "widget-1"}); err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if service.removeCalls != 1 {
		t.Fatalf("expected remove call")
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

type stubService struct {
	addCalls     int
	removeCalls  int
	reorderCalls int
	refreshCalls int
}

func (s *stubService) AddWidget(context.Context, dashboard.AddWidgetRequest) error {
	s.addCalls++
	return nil
}

func (s *stubService) RemoveWidget(context.Context, string) error {
	s.removeCalls++
	return nil
}

func (s *stubService) ReorderWidgets(context.Context, string, []string) error {
	s.reorderCalls++
	return nil
}

func (s *stubService) NotifyWidgetUpdated(context.Context, dashboard.WidgetEvent) error {
	s.refreshCalls++
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
	return dashboard.WidgetInstance{ID: input.DefinitionID + "-instance", DefinitionID: input.DefinitionID}, nil
}

func (s *stubStore) DeleteInstance(context.Context, string) error { return nil }

func (s *stubStore) AssignInstance(context.Context, dashboard.AssignWidgetInput) error {
	s.assignCalls++
	return nil
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
