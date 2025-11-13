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
