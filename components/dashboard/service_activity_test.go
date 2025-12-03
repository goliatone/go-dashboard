package dashboard

import (
	"context"
	"testing"

	"github.com/goliatone/go-dashboard/pkg/activity"
)

func TestAddWidgetEmitsActivity(t *testing.T) {
	store := &fakeWidgetStore{}
	capture := &activity.CaptureHook{}
	service := NewService(Options{
		WidgetStore: store,
		ActivityHooks: activity.Hooks{
			capture,
		},
		ActivityConfig: activity.Config{Enabled: true, Channel: "dashboard"},
	})

	req := AddWidgetRequest{
		DefinitionID: "admin.widget.user_stats",
		AreaCode:     "admin.dashboard.main",
		ActorID:      "actor-1",
		UserID:       "user-1",
		TenantID:     "tenant-1",
	}
	if err := service.AddWidget(context.Background(), req); err != nil {
		t.Fatalf("AddWidget returned error: %v", err)
	}
	if len(capture.Events) != 1 {
		t.Fatalf("expected 1 activity event, got %d", len(capture.Events))
	}
	event := capture.Events[0]
	if event.Verb != "dashboard.widget.add" || event.ObjectType != "widget_instance" {
		t.Fatalf("unexpected event payload: %+v", event)
	}
	if event.ActorID != "actor-1" || event.UserID != "user-1" || event.TenantID != "tenant-1" {
		t.Fatalf("unexpected actor context: %+v", event)
	}
	if event.Metadata["area_code"] != "admin.dashboard.main" {
		t.Fatalf("expected area_code metadata, got %+v", event.Metadata)
	}
}

func TestRemoveWidgetEmitsActivity(t *testing.T) {
	store := &fakeWidgetStore{
		instances: map[string]WidgetInstance{
			"w-1": {ID: "w-1", DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main"},
		},
	}
	capture := &activity.CaptureHook{}
	service := NewService(Options{
		WidgetStore: store,
		ActivityHooks: activity.Hooks{
			capture,
		},
		ActivityConfig: activity.Config{Enabled: true},
	})

	if err := service.RemoveWidget(context.Background(), "w-1"); err != nil {
		t.Fatalf("RemoveWidget returned error: %v", err)
	}
	if len(capture.Events) != 1 {
		t.Fatalf("expected 1 activity event, got %d", len(capture.Events))
	}
	event := capture.Events[0]
	if event.Verb != "dashboard.widget.remove" || event.ObjectID != "w-1" {
		t.Fatalf("unexpected event payload: %+v", event)
	}
	if event.Metadata["definition_id"] != "admin.widget.user_stats" {
		t.Fatalf("expected definition_id metadata, got %+v", event.Metadata)
	}
}

func TestReorderWidgetsEmitsActivity(t *testing.T) {
	store := &fakeWidgetStore{}
	capture := &activity.CaptureHook{}
	service := NewService(Options{
		WidgetStore: store,
		ActivityHooks: activity.Hooks{
			capture,
		},
		ActivityConfig: activity.Config{Enabled: true},
	})
	err := service.ReorderWidgets(context.Background(), "admin.dashboard.main", []string{"w1", "w2"})
	if err != nil {
		t.Fatalf("ReorderWidgets returned error: %v", err)
	}
	if len(capture.Events) != 1 {
		t.Fatalf("expected 1 activity event, got %d", len(capture.Events))
	}
	if capture.Events[0].Verb != "dashboard.widget.reorder" {
		t.Fatalf("unexpected verb %q", capture.Events[0].Verb)
	}
	if capture.Events[0].Metadata["count"] != 2 {
		t.Fatalf("expected reorder count metadata, got %+v", capture.Events[0].Metadata)
	}
}
