package httpapi

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/goliatone/go-dashboard/components/dashboard"
)

type failingTransportViewModel struct{}

func (failingTransportViewModel) Serialize() (any, error) {
	return nil, errors.New("serialize failed")
}

type transportLayoutStore struct{}

type stubPageControllerResolver struct {
	layout dashboard.Layout
	err    error
}

func (s *stubPageControllerResolver) ConfigureLayout(context.Context, dashboard.ViewerContext) (dashboard.Layout, error) {
	return s.layout, s.err
}

func (transportLayoutStore) EnsureArea(context.Context, dashboard.WidgetAreaDefinition) (bool, error) {
	return false, nil
}

func (transportLayoutStore) EnsureDefinition(context.Context, dashboard.WidgetDefinition) (bool, error) {
	return false, nil
}

func (transportLayoutStore) CreateInstance(context.Context, dashboard.CreateWidgetInstanceInput) (dashboard.WidgetInstance, error) {
	return dashboard.WidgetInstance{}, nil
}

func (transportLayoutStore) GetInstance(context.Context, string) (dashboard.WidgetInstance, error) {
	return dashboard.WidgetInstance{}, nil
}

func (transportLayoutStore) DeleteInstance(context.Context, string) error { return nil }

func (transportLayoutStore) AssignInstance(context.Context, dashboard.AssignWidgetInput) error {
	return nil
}

func (transportLayoutStore) UpdateInstance(context.Context, dashboard.UpdateWidgetInstanceInput) (dashboard.WidgetInstance, error) {
	return dashboard.WidgetInstance{}, nil
}

func (transportLayoutStore) ReorderArea(context.Context, dashboard.ReorderAreaInput) error {
	return nil
}

func (transportLayoutStore) ResolveArea(_ context.Context, input dashboard.ResolveAreaInput) (dashboard.ResolvedArea, error) {
	return dashboard.ResolvedArea{
		AreaCode: input.AreaCode,
		Widgets: []dashboard.WidgetInstance{
			{
				ID:           "broken-transport-1",
				DefinitionID: "custom.widget.fail_transport",
				AreaCode:     input.AreaCode,
			},
		},
	}, nil
}

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

func TestPreferencesInputFromMapCompatibleSupportsLegacyLayoutPayload(t *testing.T) {
	input, err := PreferencesInputFromMapCompatible(map[string]any{
		"layout": []any{
			map[string]any{
				"id":       "w2",
				"area":     "admin.dashboard.main",
				"position": 0,
				"span":     8,
			},
			map[string]any{
				"id":       "w1",
				"area":     "admin.dashboard.main",
				"position": 1,
				"span":     4,
				"hidden":   true,
				"locale":   "fr",
			},
		},
	}, dashboard.ViewerContext{UserID: "user-1"})
	if err != nil {
		t.Fatalf("PreferencesInputFromMapCompatible returned error: %v", err)
	}
	if input.Viewer.UserID != "user-1" || input.Viewer.Locale != "fr" {
		t.Fatalf("expected viewer merged with legacy locale, got %+v", input.Viewer)
	}
	if !reflect.DeepEqual(input.AreaOrder["admin.dashboard.main"], []string{"w2", "w1"}) {
		t.Fatalf("expected legacy layout converted into area order, got %+v", input.AreaOrder)
	}
	rows := input.LayoutRows["admin.dashboard.main"]
	if len(rows) != 2 || rows[0].Widgets[0].Width != 8 || rows[1].Widgets[0].Width != 4 {
		t.Fatalf("expected legacy layout converted into row widths, got %+v", input.LayoutRows)
	}
	if len(input.HiddenWidgets) != 1 || input.HiddenWidgets[0] != "w1" {
		t.Fatalf("expected hidden widgets preserved, got %+v", input.HiddenWidgets)
	}
}

func TestLegacyPreferencesInputFromMapRejectsEmptyLayoutPayload(t *testing.T) {
	if _, err := LegacyPreferencesInputFromMap(map[string]any{"layout": []any{}}, dashboard.ViewerContext{UserID: "user-1"}); err == nil {
		t.Fatalf("expected empty legacy layout payload to fail")
	}
}

func TestPageReturnsTypedDashboardPage(t *testing.T) {
	controller := dashboard.NewController(dashboard.ControllerOptions{
		Service: &stubPageControllerResolver{
			layout: dashboard.Layout{
				Areas: map[string][]dashboard.WidgetInstance{
					"admin.dashboard.main": {
						{ID: "w1", DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main"},
					},
				},
			},
		},
	})
	page, err := Page(context.Background(), controller, dashboard.ViewerContext{Locale: "es"})
	if err != nil {
		t.Fatalf("Page returned error: %v", err)
	}
	if page.Locale != "es" {
		t.Fatalf("expected locale propagated to page transport, got %+v", page)
	}
	if len(page.Areas) == 0 || page.Areas[0].Widgets[0].ID != "w1" {
		t.Fatalf("expected typed page transport, got %+v", page)
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

func TestLayoutSurfacesWidgetSerializeFailures(t *testing.T) {
	registry := dashboard.NewRegistry()
	if err := registry.RegisterDefinition(dashboard.WidgetDefinition{Code: "custom.widget.fail_transport"}); err != nil {
		t.Fatalf("RegisterDefinition returned error: %v", err)
	}
	provider := dashboard.NewWidgetProvider(dashboard.WidgetSpec[struct{}, int, failingTransportViewModel]{
		Definition: dashboard.WidgetDefinition{Code: "custom.widget.fail_transport"},
		Fetch: func(context.Context, dashboard.WidgetRequest[struct{}]) (int, error) {
			return 1, nil
		},
		BuildView: func(context.Context, int, dashboard.WidgetViewContext[struct{}]) (failingTransportViewModel, error) {
			return failingTransportViewModel{}, nil
		},
	})
	if err := registry.RegisterProvider("custom.widget.fail_transport", provider); err != nil {
		t.Fatalf("RegisterProvider returned error: %v", err)
	}

	service := dashboard.NewService(dashboard.Options{
		WidgetStore:     transportLayoutStore{},
		Providers:       registry,
		PreferenceStore: dashboard.NewInMemoryPreferenceStore(),
	})
	controller := dashboard.NewController(dashboard.ControllerOptions{Service: service})
	if _, err := Layout(context.Background(), controller, dashboard.ViewerContext{UserID: "user-1"}); err == nil {
		t.Fatalf("expected Layout helper to surface widget serialize failure")
	}
}
