package httpapi

import (
	"context"
	"testing"

	"github.com/goliatone/go-dashboard/components/dashboard"
)

func TestPreferencesInputFromMapInjectsResolvedViewer(t *testing.T) {
	input, err := PreferencesInputFromMap(map[string]any{
		"viewer": map[string]any{"user_id": "ignored"},
		"area_order": map[string]any{
			"admin.dashboard.main": []any{"w1"},
		},
		"layout_rows": map[string]any{
			"admin.dashboard.main": []any{
				map[string]any{
					"widgets": []any{
						map[string]any{"id": "w1", "width": 8},
					},
				},
			},
		},
		"hidden_widget_ids": []any{"w2"},
	}, dashboard.ViewerContext{UserID: "user-1", Locale: "es"})
	if err != nil {
		t.Fatalf("PreferencesInputFromMap returned error: %v", err)
	}
	if input.Viewer.UserID != "user-1" || input.Viewer.Locale != "es" {
		t.Fatalf("expected resolved viewer to override payload viewer, got %+v", input.Viewer)
	}
	if input.LayoutRows["admin.dashboard.main"][0].Widgets[0].Width != 8 {
		t.Fatalf("expected layout rows to decode, got %+v", input.LayoutRows)
	}
}

func TestPreferencesHelperUsesSharedExecutor(t *testing.T) {
	svc := &stubServiceExecutor{}
	reply, err := Preferences(context.Background(), dashboard.NewServiceExecutor(svc), dashboard.SaveLayoutPreferencesInput{
		Viewer: dashboard.ViewerContext{UserID: "user-1", Locale: "en"},
		AreaOrder: map[string][]string{
			"admin.dashboard.main": {"w1"},
		},
	})
	if err != nil {
		t.Fatalf("Preferences returned error: %v", err)
	}
	if reply.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", reply.StatusCode)
	}
	payload, ok := reply.Payload.(map[string]string)
	if !ok || payload["status"] != "saved" {
		t.Fatalf("expected saved payload, got %+v", reply.Payload)
	}
	if svc.prefsCalls != 1 {
		t.Fatalf("expected preferences helper to call executor, got %d", svc.prefsCalls)
	}
}
