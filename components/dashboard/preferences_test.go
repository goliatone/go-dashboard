package dashboard

import (
	"context"
	"testing"
)

func TestInMemoryPreferenceStore(t *testing.T) {
	store := NewInMemoryPreferenceStore()
	viewer := ViewerContext{UserID: "user-1", Locale: "en"}
	overrides := LayoutOverrides{AreaOrder: map[string][]string{
		"admin.dashboard.main": {"w2", "w1"},
	}, HiddenWidgets: map[string]bool{"w3": true}}
	if err := store.SaveLayoutOverrides(context.Background(), viewer, overrides); err != nil {
		t.Fatalf("SaveLayoutOverrides returned error: %v", err)
	}
	out, err := store.LayoutOverrides(context.Background(), viewer)
	if err != nil {
		t.Fatalf("LayoutOverrides returned error: %v", err)
	}
	if out.Locale != "en" {
		t.Fatalf("expected locale metadata persisted, got %q", out.Locale)
	}
	if order := out.AreaOrder["admin.dashboard.main"]; len(order) != 2 || order[0] != "w2" {
		t.Fatalf("expected override order, got %v", order)
	}
	if hidden := out.HiddenWidgets["w3"]; !hidden {
		t.Fatalf("expected hidden widget persisted")
	}
}
