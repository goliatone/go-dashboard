package dashboard

import (
	"bytes"
	"html/template"
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

func TestTemplateRendererRendersShellMarkup(t *testing.T) {
	renderer, err := NewTemplateRenderer()
	if err != nil {
		t.Fatalf("NewTemplateRenderer returned error: %v", err)
	}
	page := Page{
		Title: "Workbench",
		Shell: &Shell{
			SurfaceID: "settings",
			Label:     "Settings workbench",
			Regions: []ShellRegion{
				{
					ID:          "list",
					Role:        ShellRegionRoleNavigation,
					Placement:   ShellRegionPlacementLeading,
					Label:       "Settings list",
					Collapsible: true,
					Resizable:   true,
					Sizing:      ShellPaneSizing{Min: 220, Default: 280, Max: 420},
					Content:     ShellRegionContent{HTML: template.HTML("<nav>Settings</nav>")},
				},
				{
					ID:          "main",
					Role:        ShellRegionRoleMain,
					Placement:   ShellRegionPlacementMain,
					Label:       "Editor",
					FocusTarget: true,
					Content:     ShellRegionContent{Text: "Edit settings"},
				},
				{
					ID:          "inspector",
					Role:        ShellRegionRoleInspector,
					Placement:   ShellRegionPlacementTrailing,
					Label:       "Inspector",
					Collapsible: true,
					Resizable:   true,
					ResizeEdge:  ShellResizeEdgeLeading,
					Sizing:      ShellPaneSizing{Min: 240, Default: 320, Max: 520},
					Content:     ShellRegionContent{Text: "Details"},
				},
			},
			Actions: []ShellAction{
				{
					ID:       "toggle-inspector",
					Label:    "Inspector",
					Kind:     ShellActionKindToggleRegion,
					RegionID: "inspector",
				},
				{
					ID:      "pin",
					Label:   "Pin",
					Kind:    ShellActionKindButton,
					Pressed: true,
				},
			},
		},
	}

	var buf bytes.Buffer
	if _, err := renderer.RenderPage("dashboard.html", page, &buf); err != nil {
		t.Fatalf("RenderPage returned error: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		`data-dashboard-shell`,
		`data-dashboard-shell-surface="settings"`,
		`data-dashboard-shell-namespace="go-dashboard:shell"`,
		`data-shell-region="list"`,
		`data-shell-toggle="list"`,
		`aria-expanded="true"`,
		`data-shell-focus-toggle="main"`,
		`aria-pressed="false"`,
		`data-shell-resize="list"`,
		`data-shell-resize="inspector"`,
		`data-shell-action="toggle-inspector"`,
		`data-shell-action-kind="toggle-region"`,
		`data-shell-action="pin"`,
		`role="separator"`,
		`aria-orientation="vertical"`,
		`aria-valuemin="220.000000"`,
		`aria-valuemax="420.000000"`,
		`<nav>Settings</nav>`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("expected shell output to contain %q, got %s", want, out)
		}
	}
	if strings.Contains(out, "content-modeling") || strings.Contains(out, "block-library") {
		t.Fatalf("expected shell template to stay domain-neutral, got %s", out)
	}
	inspectorSplitter := strings.Index(out, `data-shell-resize="inspector"`)
	inspectorRegion := strings.Index(out, `data-shell-region="inspector"`)
	if inspectorSplitter < 0 || inspectorRegion < 0 || inspectorSplitter > inspectorRegion {
		t.Fatalf("expected leading-edge trailing rail splitter before region, got %s", out)
	}
	listSplitter := strings.Index(out, `data-shell-resize="list"`)
	listRegion := strings.Index(out, `data-shell-region="list"`)
	if listSplitter < 0 || listRegion < 0 || listSplitter < listRegion {
		t.Fatalf("expected trailing-edge leading rail splitter after region, got %s", out)
	}
}

func TestTemplateRendererRejectsInvalidShell(t *testing.T) {
	renderer, err := NewTemplateRenderer()
	if err != nil {
		t.Fatalf("NewTemplateRenderer returned error: %v", err)
	}
	page := Page{
		Title: "Broken",
		Shell: &Shell{
			SurfaceID: "bad surface",
			Regions:   []ShellRegion{{ID: "main", Role: ShellRegionRoleMain}},
		},
	}

	if _, err := renderer.RenderPage("dashboard.html", page); err == nil {
		t.Fatalf("expected invalid shell to fail template rendering")
	}
}

func TestTemplateRendererKeepsNoShellDashboardLayout(t *testing.T) {
	renderer, err := NewTemplateRenderer()
	if err != nil {
		t.Fatalf("NewTemplateRenderer returned error: %v", err)
	}
	page := Page{
		Title: "Dashboard",
		Areas: []PageArea{
			{Slot: "main", Code: "admin.dashboard.main"},
			{Slot: "sidebar", Code: "admin.dashboard.sidebar"},
			{Slot: "footer", Code: "admin.dashboard.footer"},
		},
	}

	var buf bytes.Buffer
	if _, err := renderer.RenderPage("dashboard.html", page, &buf); err != nil {
		t.Fatalf("RenderPage returned error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, `data-dashboard-shell`) {
		t.Fatalf("expected no-shell page to use existing dashboard layout, got %s", out)
	}
	if !strings.Contains(out, `dashboard__column--main`) || !strings.Contains(out, `dashboard__column--sidebar`) {
		t.Fatalf("expected existing dashboard columns to render, got %s", out)
	}
}
