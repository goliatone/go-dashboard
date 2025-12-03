package activity

import (
	"context"
	"testing"
	"time"
)

func TestHooksNotifyNormalizesAndSkipsInvalid(t *testing.T) {
	var called int
	hooks := Hooks{
		HookFunc(func(ctx context.Context, evt Event) error {
			called++
			if evt.Verb != "update" {
				t.Fatalf("unexpected verb %q", evt.Verb)
			}
			if evt.ObjectType != "widget" || evt.ObjectID != "123" {
				t.Fatalf("unexpected object %s %s", evt.ObjectType, evt.ObjectID)
			}
			return nil
		}),
	}

	// Missing verb: should skip.
	_ = hooks.Notify(context.Background(), Event{})
	if called != 0 {
		t.Fatalf("expected no calls for invalid event")
	}

	// Valid event should trigger hook once.
	_ = hooks.Notify(context.Background(), Event{
		Verb:       " update ",
		ObjectType: " widget ",
		ObjectID:   " 123 ",
	})
	if called != 1 {
		t.Fatalf("expected hook to be called once, got %d", called)
	}
}

func TestNormalizeEventClones(t *testing.T) {
	meta := map[string]any{"k": "v"}
	recipients := []string{"a@example.com"}
	now := time.Now()

	evt := Event{
		Verb:       "verb",
		ObjectType: "obj",
		ObjectID:   "id",
		Metadata:   meta,
		Recipients: recipients,
		OccurredAt: now,
	}
	n := NormalizeEvent(evt)

	if &n.Metadata == &evt.Metadata {
		t.Fatalf("metadata map should be cloned")
	}
	n.Metadata["k"] = "changed"
	if evt.Metadata["k"] != "v" {
		t.Fatalf("original metadata mutated")
	}

	if len(n.Recipients) == 0 || &n.Recipients[0] == &evt.Recipients[0] {
		t.Fatalf("recipients slice should be cloned")
	}
	n.Recipients[0] = "b@example.com"
	if recipients[0] != "a@example.com" {
		t.Fatalf("original recipients mutated")
	}
	if n.OccurredAt.IsZero() || !n.OccurredAt.Equal(now) {
		t.Fatalf("occurred_at should be preserved when set")
	}
}
