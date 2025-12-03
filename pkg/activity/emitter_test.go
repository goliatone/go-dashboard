package activity

import (
	"context"
	"testing"
)

type recordingHook struct {
	events []Event
}

func (h *recordingHook) Notify(_ context.Context, evt Event) error {
	h.events = append(h.events, evt)
	return nil
}

func TestEmitterDefaultsChannelAndEmits(t *testing.T) {
	hook := &recordingHook{}
	em := NewEmitter(Hooks{hook}, Config{Enabled: true})
	if !em.Enabled() {
		t.Fatalf("expected emitter enabled")
	}
	err := em.Emit(context.Background(), Event{
		Verb:       "verb",
		ObjectType: "object",
		ObjectID:   "id",
	})
	if err != nil {
		t.Fatalf("emit returned error: %v", err)
	}
	if len(hook.events) != 1 {
		t.Fatalf("expected event emitted, got %d", len(hook.events))
	}
	if hook.events[0].Channel != "dashboard" {
		t.Fatalf("expected default channel dashboard, got %q", hook.events[0].Channel)
	}
}

func TestEmitterDisabledWithoutHooks(t *testing.T) {
	em := NewEmitter(nil, Config{Enabled: true})
	if em.Enabled() {
		t.Fatalf("expected emitter disabled without hooks")
	}
}
