package httpapi

import (
	"context"
	"testing"

	"github.com/goliatone/go-dashboard/components/dashboard"
	"github.com/goliatone/go-dashboard/components/dashboard/commands"
	"github.com/goliatone/go-dashboard/pkg/activity"
)

type stubCommander[T any] struct {
	last  T
	calls int
	err   error
}

type stubServiceExecutor struct {
	addReq         dashboard.AddWidgetRequest
	removeID       string
	reorderArea    string
	reorderIDs     []string
	refreshEvent   dashboard.WidgetEvent
	prefsViewer    dashboard.ViewerContext
	prefsOverrides dashboard.LayoutOverrides
	addCalls       int
	removeCalls    int
	reorderCalls   int
	refreshCalls   int
	prefsCalls     int
	err            error
}

func (s *stubServiceExecutor) AddWidget(_ context.Context, req dashboard.AddWidgetRequest) error {
	s.addReq = req
	s.addCalls++
	return s.err
}

type serviceExecutorWidgetStore struct {
	instance dashboard.WidgetInstance
}

func (s *serviceExecutorWidgetStore) EnsureArea(context.Context, dashboard.WidgetAreaDefinition) (bool, error) {
	return false, nil
}

func (s *serviceExecutorWidgetStore) EnsureDefinition(context.Context, dashboard.WidgetDefinition) (bool, error) {
	return false, nil
}

func (s *serviceExecutorWidgetStore) CreateInstance(context.Context, dashboard.CreateWidgetInstanceInput) (dashboard.WidgetInstance, error) {
	return dashboard.WidgetInstance{}, nil
}

func (s *serviceExecutorWidgetStore) GetInstance(context.Context, string) (dashboard.WidgetInstance, error) {
	return s.instance, nil
}

func (s *serviceExecutorWidgetStore) DeleteInstance(context.Context, string) error { return nil }

func (s *serviceExecutorWidgetStore) AssignInstance(context.Context, dashboard.AssignWidgetInput) error {
	return nil
}

func (s *serviceExecutorWidgetStore) UpdateInstance(context.Context, dashboard.UpdateWidgetInstanceInput) (dashboard.WidgetInstance, error) {
	return dashboard.WidgetInstance{}, nil
}

func (s *serviceExecutorWidgetStore) ReorderArea(context.Context, dashboard.ReorderAreaInput) error {
	return nil
}

func (s *serviceExecutorWidgetStore) ResolveArea(context.Context, dashboard.ResolveAreaInput) (dashboard.ResolvedArea, error) {
	return dashboard.ResolvedArea{}, nil
}

func (s *stubServiceExecutor) RemoveWidget(_ context.Context, widgetID string) error {
	s.removeID = widgetID
	s.removeCalls++
	return s.err
}

func (s *stubServiceExecutor) ReorderWidgets(_ context.Context, areaCode string, widgetIDs []string) error {
	s.reorderArea = areaCode
	s.reorderIDs = append([]string{}, widgetIDs...)
	s.reorderCalls++
	return s.err
}

func (s *stubServiceExecutor) NotifyWidgetUpdated(_ context.Context, event dashboard.WidgetEvent) error {
	s.refreshEvent = event
	s.refreshCalls++
	return s.err
}

func (s *stubServiceExecutor) SavePreferences(_ context.Context, viewer dashboard.ViewerContext, overrides dashboard.LayoutOverrides) error {
	s.prefsViewer = viewer
	s.prefsOverrides = overrides
	s.prefsCalls++
	return s.err
}

func (s *stubCommander[T]) Execute(ctx context.Context, msg T) error {
	s.last = msg
	s.calls++
	return s.err
}

func TestCommandExecutorAssign(t *testing.T) {
	assign := &stubCommander[dashboard.AddWidgetRequest]{}
	exec := &CommandExecutor{AssignCommander: assign}
	req := dashboard.AddWidgetRequest{DefinitionID: "def", AreaCode: "area"}
	if err := exec.Assign(context.Background(), req); err != nil {
		t.Fatalf("Assign returned error: %v", err)
	}
	if assign.calls != 1 {
		t.Fatalf("expected assign command execution")
	}
}

func TestCommandExecutorRemove(t *testing.T) {
	remove := &stubCommander[commands.RemoveWidgetInput]{}
	exec := &CommandExecutor{RemoveCommander: remove}
	input := commands.RemoveWidgetInput{WidgetID: "widget-1"}
	if err := exec.Remove(context.Background(), input); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if remove.last.WidgetID != "widget-1" {
		t.Fatalf("expected widget id propagation")
	}
}

func TestCommandExecutorReorder(t *testing.T) {
	reorder := &stubCommander[commands.ReorderWidgetsInput]{}
	exec := &CommandExecutor{ReorderCommander: reorder}
	input := commands.ReorderWidgetsInput{AreaCode: "area", WidgetIDs: []string{"w1", "w2"}}
	if err := exec.Reorder(context.Background(), input); err != nil {
		t.Fatalf("Reorder returned error: %v", err)
	}
	if reorder.calls != 1 {
		t.Fatalf("expected reorder execution")
	}
}

func TestCommandExecutorRefresh(t *testing.T) {
	refresh := &stubCommander[commands.RefreshWidgetInput]{}
	exec := &CommandExecutor{RefreshCommander: refresh}
	input := commands.RefreshWidgetInput{Event: dashboard.WidgetEvent{AreaCode: "area"}}
	if err := exec.Refresh(context.Background(), input); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if refresh.calls != 1 {
		t.Fatalf("expected refresh execution")
	}
}

func TestCommandExecutorMissingCommand(t *testing.T) {
	exec := &CommandExecutor{}
	if err := exec.Assign(context.Background(), dashboard.AddWidgetRequest{}); err == nil {
		t.Fatalf("expected error when assign command missing")
	}
}

func TestCommandExecutorPreferences(t *testing.T) {
	prefs := &stubCommander[commands.SaveLayoutPreferencesInput]{}
	exec := &CommandExecutor{PrefsCommander: prefs}
	input := commands.SaveLayoutPreferencesInput{Viewer: dashboard.ViewerContext{UserID: "user"}}
	if err := exec.Preferences(context.Background(), input); err != nil {
		t.Fatalf("Preferences returned error: %v", err)
	}
	if prefs.calls != 1 {
		t.Fatalf("expected preferences execution")
	}
}

func TestServiceExecutorAssign(t *testing.T) {
	svc := &stubServiceExecutor{}
	exec := NewServiceExecutor(svc)
	req := dashboard.AddWidgetRequest{DefinitionID: "def", AreaCode: "area"}
	if err := exec.Assign(context.Background(), req); err != nil {
		t.Fatalf("Assign returned error: %v", err)
	}
	if svc.addCalls != 1 || svc.addReq.DefinitionID != "def" {
		t.Fatalf("expected assign request forwarded, got %+v", svc.addReq)
	}
}

func TestServiceExecutorRemove(t *testing.T) {
	svc := &stubServiceExecutor{}
	exec := NewServiceExecutor(svc)
	input := commands.RemoveWidgetInput{WidgetID: "widget-1", ActorID: "actor-1", UserID: "user-1", TenantID: "tenant-1"}
	if err := exec.Remove(context.Background(), input); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if svc.removeCalls != 1 || svc.removeID != "widget-1" {
		t.Fatalf("expected remove request forwarded, got id=%q calls=%d", svc.removeID, svc.removeCalls)
	}
}

func TestServiceExecutorRemovePropagatesActivityContext(t *testing.T) {
	store := &serviceExecutorWidgetStore{
		instance: dashboard.WidgetInstance{
			ID:           "widget-1",
			DefinitionID: "admin.widget.stats",
			AreaCode:     "admin.dashboard.main",
		},
	}
	capture := &activity.CaptureHook{}
	svc := dashboard.NewService(dashboard.Options{
		WidgetStore: store,
		ActivityHooks: activity.Hooks{
			capture,
		},
		ActivityConfig: activity.Config{Enabled: true},
	})
	exec := NewServiceExecutor(svc)
	input := commands.RemoveWidgetInput{
		WidgetID: "widget-1",
		ActorID:  "actor-1",
		UserID:   "user-1",
		TenantID: "tenant-1",
	}
	if err := exec.Remove(context.Background(), input); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if len(capture.Events) != 1 {
		t.Fatalf("expected 1 activity event, got %d", len(capture.Events))
	}
	event := capture.Events[0]
	if event.ActorID != "actor-1" || event.UserID != "user-1" || event.TenantID != "tenant-1" {
		t.Fatalf("expected activity context propagation, got %+v", event)
	}
	if event.Verb != "dashboard.widget.remove" || event.ObjectID != "widget-1" {
		t.Fatalf("unexpected activity event %+v", event)
	}
}

func TestServiceExecutorReorder(t *testing.T) {
	svc := &stubServiceExecutor{}
	exec := NewServiceExecutor(svc)
	input := commands.ReorderWidgetsInput{AreaCode: "area", WidgetIDs: []string{"w1", "w2"}}
	if err := exec.Reorder(context.Background(), input); err != nil {
		t.Fatalf("Reorder returned error: %v", err)
	}
	if svc.reorderCalls != 1 || svc.reorderArea != "area" {
		t.Fatalf("expected reorder request forwarded, got area=%q calls=%d", svc.reorderArea, svc.reorderCalls)
	}
}

func TestServiceExecutorRefresh(t *testing.T) {
	svc := &stubServiceExecutor{}
	exec := NewServiceExecutor(svc)
	input := commands.RefreshWidgetInput{Event: dashboard.WidgetEvent{AreaCode: "area"}}
	if err := exec.Refresh(context.Background(), input); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if svc.refreshCalls != 1 || svc.refreshEvent.AreaCode != "area" {
		t.Fatalf("expected refresh event forwarded, got %+v", svc.refreshEvent)
	}
}

func TestServiceExecutorPreferences(t *testing.T) {
	svc := &stubServiceExecutor{}
	exec := NewServiceExecutor(svc)
	input := commands.SaveLayoutPreferencesInput{
		Viewer:     dashboard.ViewerContext{UserID: "user-1", Locale: "es"},
		AreaOrder:  map[string][]string{"admin.dashboard.main": {"w1"}},
		LayoutRows: map[string][]commands.LayoutRowInput{"admin.dashboard.main": {{Widgets: []commands.LayoutWidgetInput{{ID: "w1", Width: 6}}}}},
		HiddenWidgets: []string{
			"w2",
		},
	}
	if err := exec.Preferences(context.Background(), input); err != nil {
		t.Fatalf("Preferences returned error: %v", err)
	}
	if svc.prefsCalls != 1 {
		t.Fatalf("expected preferences call, got %d", svc.prefsCalls)
	}
	if svc.prefsViewer.UserID != "user-1" || svc.prefsOverrides.Locale != "es" {
		t.Fatalf("expected viewer/locale forwarded, got viewer=%+v overrides=%+v", svc.prefsViewer, svc.prefsOverrides)
	}
	if !svc.prefsOverrides.HiddenWidgets["w2"] {
		t.Fatalf("expected hidden widget id to be converted into override map")
	}
	rows := svc.prefsOverrides.AreaRows["admin.dashboard.main"]
	if len(rows) != 1 || len(rows[0].Widgets) != 1 || rows[0].Widgets[0].Width != 6 {
		t.Fatalf("expected layout rows converted, got %+v", svc.prefsOverrides.AreaRows)
	}
}

func TestServiceExecutorMissingService(t *testing.T) {
	exec := &ServiceExecutor{}
	if err := exec.Assign(context.Background(), dashboard.AddWidgetRequest{}); err == nil {
		t.Fatalf("expected error when service missing")
	}
}
