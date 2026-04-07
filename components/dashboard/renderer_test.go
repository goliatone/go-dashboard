package dashboard

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

type captureLegacyRenderer struct {
	lastTemplate string
	lastData     any
}

func (renderer *captureLegacyRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	renderer.lastTemplate = name
	renderer.lastData = data
	if len(out) > 0 && out[0] != nil {
		_, _ = out[0].Write([]byte("ok"))
	}
	return "ok", nil
}

func TestAdaptLegacyRendererUsesTypedPageBoundary(t *testing.T) {
	page := Page{
		Title: "Dashboard",
		Areas: []PageArea{
			{
				Slot: "main",
				Code: "admin.dashboard.main",
				Widgets: []WidgetFrame{
					{ID: "w1", Definition: "admin.widget.user_stats"},
				},
			},
		},
	}
	legacy := &captureLegacyRenderer{}
	renderer := AdaptLegacyRenderer(legacy)

	var buf bytes.Buffer
	if _, err := renderer.RenderPage("dashboard.html", page, &buf); err != nil {
		t.Fatalf("RenderPage returned error: %v", err)
	}
	if legacy.lastTemplate != "dashboard.html" {
		t.Fatalf("expected template name forwarded, got %q", legacy.lastTemplate)
	}
	payload, ok := legacy.lastData.(map[string]any)
	if !ok {
		t.Fatalf("expected legacy adapter to receive map payload, got %T", legacy.lastData)
	}
	if payload["title"] != "Dashboard" {
		t.Fatalf("expected page payload derived from typed page, got %+v", payload)
	}
}

func TestNewTemplateRendererUsesEmbeddedTemplatesOutsidePackageDirectory(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd returned error: %v", err)
	}
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Chdir returned error: %v", err)
	}
	defer func() {
		if err := os.Chdir(cwd); err != nil {
			t.Fatalf("failed to restore cwd: %v", err)
		}
	}()

	renderer, err := NewTemplateRenderer()
	if err != nil {
		t.Fatalf("NewTemplateRenderer returned error: %v", err)
	}
	page := Page{
		Title:  "Dashboard",
		Locale: "en",
		Areas: []PageArea{
			{
				Slot: "main",
				Code: "admin.dashboard.main",
				Widgets: []WidgetFrame{
					{
						ID:         "stats-1",
						Definition: "admin.widget.user_stats",
						Template:   "widgets/user_stats.html",
						Config:     map[string]any{"metric": "total"},
						Data: struct {
							Title  string         `json:"title"`
							Values map[string]int `json:"values"`
						}{
							Title:  "Users",
							Values: map[string]int{"total": 42},
						},
					},
				},
			},
			{Slot: "sidebar", Code: "admin.dashboard.sidebar"},
			{Slot: "footer", Code: "admin.dashboard.footer"},
		},
	}

	var buf bytes.Buffer
	if _, err := renderer.RenderPage("dashboard.html", page, &buf); err != nil {
		t.Fatalf("RenderPage returned error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Dashboard") {
		t.Fatalf("expected rendered output to include page title, got %q", out)
	}
	if !strings.Contains(out, "42") {
		t.Fatalf("expected rendered output to include widget data serialized from struct payload, got %q", out)
	}
}
