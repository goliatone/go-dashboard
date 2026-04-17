package dashboard

import (
	"context"
	"testing"
)

func TestServiceDiagnosticsExposeResolvedState(t *testing.T) {
	store := &fakeWidgetStore{
		resolved: map[string][]WidgetInstance{
			"admin.dashboard.main": {
				{ID: "w1", DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main", Configuration: map[string]any{"metric": "total"}},
				{ID: "w2", DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main", Configuration: map[string]any{"metric": "active"}},
			},
			"admin.dashboard.sidebar": {
				{ID: "w3", DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.sidebar"},
			},
		},
	}
	prefs := NewInMemoryPreferenceStore()
	viewer := ViewerContext{UserID: "user-1", Locale: "es"}
	if err := prefs.SaveLayoutOverrides(context.Background(), viewer, LayoutOverrides{
		AreaOrder: map[string][]string{
			"admin.dashboard.main": {"w2", "w1"},
		},
		HiddenWidgets: map[string]bool{
			"w3": true,
		},
	}); err != nil {
		t.Fatalf("SaveLayoutOverrides returned error: %v", err)
	}
	service := NewService(Options{
		WidgetStore:     store,
		PreferenceStore: prefs,
		ThemeProvider: &stubThemeProvider{
			selection: &ThemeSelection{Name: "admin", Variant: "dark"},
		},
	})

	diagnostics, err := service.Diagnostics(context.Background(), viewer)
	if err != nil {
		t.Fatalf("Diagnostics returned error: %v", err)
	}
	if diagnostics.Viewer.UserID != "user-1" || diagnostics.Preferences.Locale != "es" {
		t.Fatalf("expected viewer and preference locale preserved, got %+v", diagnostics)
	}
	if diagnostics.Theme == nil || diagnostics.Theme.Variant != "dark" {
		t.Fatalf("expected theme diagnostics, got %+v", diagnostics.Theme)
	}
	if len(diagnostics.Layout.Areas) != 3 {
		t.Fatalf("expected stable diagnostics area set, got %+v", diagnostics.Layout.Areas)
	}
	if diagnostics.Layout.Areas[0].Code != "admin.dashboard.main" || diagnostics.Layout.Areas[1].Code != "admin.dashboard.sidebar" {
		t.Fatalf("expected area diagnostics in service order, got %+v", diagnostics.Layout.Areas)
	}
	main := diagnostics.Layout.Areas[0]
	if len(main.Widgets) != 2 || main.Widgets[0].ID != "w2" || main.Widgets[1].ID != "w1" {
		t.Fatalf("expected ordered widgets in diagnostics, got %+v", main.Widgets)
	}
	if len(diagnostics.Layout.Areas[1].Widgets) != 0 {
		t.Fatalf("expected hidden widgets removed from diagnostics, got %+v", diagnostics.Layout.Areas[1].Widgets)
	}

	diagnostics.Preferences.HiddenWidgets["w1"] = true
	diagnostics.Layout.Areas[0].Widgets[0].Configuration["metric"] = "mutated"

	refreshed, err := service.Diagnostics(context.Background(), viewer)
	if err != nil {
		t.Fatalf("Diagnostics second call returned error: %v", err)
	}
	if refreshed.Preferences.HiddenWidgets["w1"] {
		t.Fatalf("expected diagnostics preferences to be cloned, got %+v", refreshed.Preferences.HiddenWidgets)
	}
	if refreshed.Layout.Areas[0].Widgets[0].Configuration["metric"] != "active" {
		t.Fatalf("expected widget configuration snapshot to be cloned, got %+v", refreshed.Layout.Areas[0].Widgets[0].Configuration)
	}
}

func TestControllerDiagnosticsIncludeTypedPage(t *testing.T) {
	store := &fakeWidgetStore{
		resolved: map[string][]WidgetInstance{
			"admin.dashboard.main": {
				{ID: "w1", DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main"},
			},
		},
	}
	service := NewService(Options{
		WidgetStore:     store,
		PreferenceStore: NewInMemoryPreferenceStore(),
	})
	controller := NewController(ControllerOptions{Service: service})

	diagnostics, err := controller.Diagnostics(context.Background(), ViewerContext{UserID: "user-1", Locale: "en"})
	if err != nil {
		t.Fatalf("Diagnostics returned error: %v", err)
	}
	if diagnostics.Page == nil {
		t.Fatalf("expected controller diagnostics to include typed page")
	}
	if len(diagnostics.Page.Areas) == 0 || diagnostics.Page.Areas[0].Widgets[0].ID != "w1" {
		t.Fatalf("expected typed page diagnostics, got %+v", diagnostics.Page)
	}
	if len(diagnostics.Layout.Areas) == 0 || diagnostics.Layout.Areas[0].Code != "admin.dashboard.main" {
		t.Fatalf("expected layout diagnostics carried through controller, got %+v", diagnostics.Layout.Areas)
	}

	diagnostics.Page.Areas[0].Widgets[0].ID = "mutated"
	refreshed, err := controller.Diagnostics(context.Background(), ViewerContext{UserID: "user-1", Locale: "en"})
	if err != nil {
		t.Fatalf("Diagnostics second call returned error: %v", err)
	}
	if refreshed.Page.Areas[0].Widgets[0].ID != "w1" {
		t.Fatalf("expected controller diagnostics page snapshot to be cloned, got %+v", refreshed.Page.Areas[0].Widgets)
	}
}
