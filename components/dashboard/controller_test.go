package dashboard

import (
	"bytes"
	"context"
	"io"
	"testing"
)

type stubLayoutResolver struct {
	layout Layout
	err    error
}

func (s *stubLayoutResolver) ConfigureLayout(ctx context.Context, viewer ViewerContext) (Layout, error) {
	return s.layout, s.err
}

type stubRenderer struct {
	lastTemplate string
	lastPayload  map[string]any
	err          error
}

func (r *stubRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	r.lastTemplate = name
	if payload, ok := data.(map[string]any); ok {
		r.lastPayload = payload
	}
	if len(out) > 0 && out[0] != nil {
		out[0].Write([]byte("<html></html>"))
	}
	return "<html></html>", r.err
}

func TestControllerRenderTemplate(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{ID: "w1", DefinitionID: "admin.widget.user_stats", Metadata: map[string]any{"data": WidgetData{"value": 42}}},
				},
			},
		},
	}
	renderer := &stubRenderer{}
	controller := NewController(ControllerOptions{
		Service:  service,
		Renderer: renderer,
		Template: "dashboard.html",
	})

	var buf bytes.Buffer
	if err := controller.RenderTemplate(context.Background(), ViewerContext{UserID: "user"}, &buf); err != nil {
		t.Fatalf("RenderTemplate returned error: %v", err)
	}
	if renderer.lastTemplate != "dashboard.html" {
		t.Fatalf("expected dashboard template to render, got %s", renderer.lastTemplate)
	}
	if buf.Len() == 0 {
		t.Fatalf("expected rendered output")
	}
}

func TestLayoutPayloadUsesSnakeCaseKeys(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{ID: "w1", DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main", Configuration: map[string]any{"metric": "total"}},
				},
			},
		},
	}
	controller := NewController(ControllerOptions{Service: service})

	payload, err := controller.LayoutPayload(context.Background(), ViewerContext{})
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	if locale, ok := payload["locale"].(string); !ok || locale != "" {
		t.Fatalf("expected empty locale for anonymous viewer, got %#v", payload["locale"])
	}
	areas, ok := payload["areas"].(map[string]any)
	if !ok {
		t.Fatalf("expected areas map, got %T", payload["areas"])
	}
	mainArea, ok := areas["main"].(map[string]any)
	if !ok {
		t.Fatalf("expected main area map, got %T", areas["main"])
	}
	if _, ok := mainArea["code"]; !ok {
		t.Fatalf("expected snake_case code key")
	}
	widgets, ok := mainArea["widgets"].([]map[string]any)
	if !ok {
		t.Fatalf("expected widgets slice, got %T", mainArea["widgets"])
	}
	if len(widgets) == 0 {
		t.Fatalf("expected at least one widget")
	}
	if _, ok := widgets[0]["area_code"]; !ok {
		t.Fatalf("expected area_code key on widget payload")
	}

	ordered, ok := payload["ordered_areas"].([]map[string]any)
	if !ok {
		raw, rok := payload["ordered_areas"].([]any)
		if !rok {
			t.Fatalf("expected ordered_areas slice, got %T", payload["ordered_areas"])
		}
		ordered = make([]map[string]any, 0, len(raw))
		for _, item := range raw {
			if m, ok := item.(map[string]any); ok {
				ordered = append(ordered, m)
			}
		}
	}
	if len(ordered) != 3 {
		t.Fatalf("expected 3 ordered areas, got %d", len(ordered))
	}
	if ordered[0]["slot"] != "main" || ordered[1]["slot"] != "sidebar" || ordered[2]["slot"] != "footer" {
		t.Fatalf("unexpected ordered area slots: %+v", ordered)
	}
}

func TestLayoutPayloadIncludesLocale(t *testing.T) {
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{},
		},
	}
	controller := NewController(ControllerOptions{Service: service})
	viewer := ViewerContext{Locale: "es"}
	payload, err := controller.LayoutPayload(context.Background(), viewer)
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	if payload["locale"] != "es" {
		t.Fatalf("expected locale propagated to payload, got %#v", payload["locale"])
	}
}

func TestLayoutPayloadIncludesTheme(t *testing.T) {
	theme := &ThemeSelection{
		Name:    "demo",
		Variant: "dark",
		Tokens: map[string]string{
			"--color-primary": "#fff",
		},
		Assets: ThemeAssets{
			Values: map[string]string{
				"logo": "img/logo.svg",
			},
			Prefix: "https://cdn.example.com",
		},
	}
	service := &stubLayoutResolver{
		layout: Layout{
			Areas: map[string][]WidgetInstance{
				"admin.dashboard.main": {
					{ID: "w1", DefinitionID: "admin.widget.user_stats"},
				},
			},
			Theme: theme,
		},
	}
	controller := NewController(ControllerOptions{Service: service})
	payload, err := controller.LayoutPayload(context.Background(), ViewerContext{})
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	rawTheme, ok := payload["theme"].(map[string]any)
	if !ok {
		t.Fatalf("expected theme payload included in response")
	}
	if rawTheme["variant"] != "dark" {
		t.Fatalf("expected theme variant propagated, got %#v", rawTheme["variant"])
	}
	assets, ok := rawTheme["assets"].(map[string]string)
	if !ok {
		t.Fatalf("expected resolved assets map, got %T", rawTheme["assets"])
	}
	if assets["logo"] != "https://cdn.example.com/img/logo.svg" {
		t.Fatalf("expected asset URL resolved with prefix, got %s", assets["logo"])
	}
	widgets := payload["areas"].(map[string]any)["main"].(map[string]any)["widgets"].([]map[string]any)
	if widgets[0]["theme"] == nil {
		t.Fatalf("expected widget payload to include theme reference")
	}
}

func TestTemplatePathForDefinition(t *testing.T) {
	tests := map[string]string{
		"admin.widget.user_stats":       "widgets/user_stats.html",
		"admin.widget.analytics_funnel": "widgets/analytics_funnel.html",
		"user_stats":                    "widgets/user_stats.html",
		"":                              "widgets/unknown.html",
	}
	for def, expect := range tests {
		if got := templatePathFor(def); got != expect {
			t.Fatalf("templatePathFor(%q) = %q, want %q", def, got, expect)
		}
	}
}

func TestControllerSupportsCustomAreas(t *testing.T) {
	layout := Layout{
		Areas: map[string][]WidgetInstance{
			"custom.hero":   {{ID: "hero-1"}},
			"custom.bottom": {{ID: "bottom-1"}},
		},
	}
	service := &stubLayoutResolver{layout: layout}
	controller := NewController(ControllerOptions{
		Service: service,
		Areas: []AreaSlot{
			{Slot: "hero", Code: "custom.hero"},
			{Slot: "bottom", Code: "custom.bottom"},
		},
	})

	payload, err := controller.LayoutPayload(context.Background(), ViewerContext{})
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	areas, ok := payload["areas"].(map[string]any)
	if !ok {
		t.Fatalf("expected areas map, got %T", payload["areas"])
	}
	if _, ok := areas["hero"]; !ok {
		t.Fatalf("expected hero slot in areas map")
	}
	if _, ok := areas["bottom"]; !ok {
		t.Fatalf("expected bottom slot in areas map")
	}
	ordered := payload["ordered_areas"].([]map[string]any)
	if ordered[0]["slot"] != "hero" || ordered[1]["slot"] != "bottom" {
		t.Fatalf("unexpected ordered slot sequence: %+v", ordered)
	}
}
