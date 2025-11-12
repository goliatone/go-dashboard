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
