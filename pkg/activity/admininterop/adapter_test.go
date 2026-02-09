package admininterop

import (
	"context"
	"testing"
	"time"

	dashboardactivity "github.com/goliatone/go-dashboard/pkg/activity"
)

func TestAdapterRecordMapsFields(t *testing.T) {
	capture := &dashboardactivity.CaptureHook{}
	adapter := NewAdapter(
		dashboardactivity.Hooks{capture},
		dashboardactivity.Config{Enabled: true, Channel: "dashboard"},
	)

	now := time.Date(2026, 1, 1, 15, 0, 0, 0, time.UTC)
	record := Record{
		Actor:   "user-123",
		Action:  "create",
		Object:  "page:42",
		Channel: "audit",
		Metadata: map[string]any{
			"locale": "en",
		},
		OccurredAt: now,
	}

	if err := adapter.Record(context.Background(), record); err != nil {
		t.Fatalf("record: %v", err)
	}
	if len(capture.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(capture.Events))
	}

	event := capture.Events[0]
	if event.ActorID != "user-123" || event.Verb != "create" {
		t.Fatalf("unexpected actor/action mapping: %+v", event)
	}
	if event.ObjectType != "page" || event.ObjectID != "42" {
		t.Fatalf("unexpected object mapping: %+v", event)
	}
	if event.Channel != "audit" {
		t.Fatalf("expected channel audit, got %q", event.Channel)
	}
	if !event.OccurredAt.Equal(now) {
		t.Fatalf("expected occurred_at %v, got %v", now, event.OccurredAt)
	}
	if event.Metadata["locale"] != "en" {
		t.Fatalf("expected metadata passthrough, got %+v", event.Metadata)
	}
}

func TestAdapterRecordDefaultsChannelToAdmin(t *testing.T) {
	capture := &dashboardactivity.CaptureHook{}
	adapter := NewAdapter(
		dashboardactivity.Hooks{capture},
		dashboardactivity.Config{Enabled: true, Channel: "dashboard"},
	)

	if err := adapter.Record(context.Background(), Record{
		Actor:  "user-1",
		Action: "update",
		Object: "settings:7",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	if len(capture.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(capture.Events))
	}
	if capture.Events[0].Channel != "admin" {
		t.Fatalf("expected channel admin, got %q", capture.Events[0].Channel)
	}
}

func TestAdapterRecordFallsBackToEmitterChannel(t *testing.T) {
	capture := &dashboardactivity.CaptureHook{}
	adapter := NewAdapter(
		dashboardactivity.Hooks{capture},
		dashboardactivity.Config{Enabled: true, Channel: "dashboard"},
		WithDefaultChannel(""),
	)

	if err := adapter.Record(context.Background(), Record{
		Actor:  "user-1",
		Action: "update",
		Object: "settings:7",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	if len(capture.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(capture.Events))
	}
	if capture.Events[0].Channel != "dashboard" {
		t.Fatalf("expected channel dashboard, got %q", capture.Events[0].Channel)
	}
}

func TestAdapterRecordDefaultsOccurredAtWhenMissing(t *testing.T) {
	capture := &dashboardactivity.CaptureHook{}
	adapter := NewAdapter(
		dashboardactivity.Hooks{capture},
		dashboardactivity.Config{Enabled: true},
	)

	before := time.Now()
	if err := adapter.Record(context.Background(), Record{
		Actor:  "user-1",
		Action: "delete",
		Object: "page:5",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}
	after := time.Now()

	if len(capture.Events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(capture.Events))
	}
	occurredAt := capture.Events[0].OccurredAt
	if occurredAt.IsZero() || occurredAt.Before(before) || occurredAt.After(after.Add(2*time.Second)) {
		t.Fatalf("unexpected occurred_at value: %v", occurredAt)
	}
}

func TestAdapterRecordSkipsInvalidObject(t *testing.T) {
	capture := &dashboardactivity.CaptureHook{}
	adapter := NewAdapter(
		dashboardactivity.Hooks{capture},
		dashboardactivity.Config{Enabled: true},
	)

	if err := adapter.Record(context.Background(), Record{
		Actor:  "user-1",
		Action: "update",
		Object: "page",
	}); err != nil {
		t.Fatalf("record: %v", err)
	}

	if len(capture.Events) != 0 {
		t.Fatalf("expected no events for invalid object, got %d", len(capture.Events))
	}
}
