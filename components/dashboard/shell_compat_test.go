package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellContractRepresentsTwoRailWorkbench(t *testing.T) {
	shell := Shell{
		SurfaceID: "workbench",
		Regions: []ShellRegion{
			{
				ID:          "list",
				Role:        ShellRegionRoleNavigation,
				Placement:   ShellRegionPlacementLeading,
				Collapsible: true,
				Resizable:   true,
				Sizing:      ShellPaneSizing{Min: 200, Default: 240, Max: 420},
			},
			{
				ID:          "workspace",
				Role:        ShellRegionRoleMain,
				Placement:   ShellRegionPlacementMain,
				FocusTarget: true,
			},
			{
				ID:          "palette",
				Role:        ShellRegionRolePalette,
				Placement:   ShellRegionPlacementTrailing,
				Collapsible: true,
				Resizable:   true,
				ResizeEdge:  ShellResizeEdgeLeading,
				Sizing:      ShellPaneSizing{Min: 220, Default: 260, Max: 400},
			},
		},
	}

	normalized, err := shell.Normalize()
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if len(normalized.Regions) != 3 {
		t.Fatalf("expected three shell regions, got %+v", normalized.Regions)
	}
	if len(normalized.FocusTargets) != 1 || normalized.FocusTargets[0].ID != "workspace" {
		t.Fatalf("expected workspace focus target, got %+v", normalized.FocusTargets)
	}
	payload := normalized.legacyPayload()
	regions := payload["region_by_id"].(map[string]any)
	if _, ok := regions["list"]; !ok {
		t.Fatalf("expected list rail in payload")
	}
	if _, ok := regions["palette"]; !ok {
		t.Fatalf("expected palette rail in payload")
	}
}

func TestShellPrimitiveSourceAvoidsConsumerSpecificTerms(t *testing.T) {
	files := []string{
		"shell.go",
		"shell_assets.go",
		"templates/components/dashboard/shell.html",
		"templates/components/dashboard/shell_region.html",
		"assets/shell/shell.js",
		"assets/shell/shell.css",
	}
	for _, file := range files {
		t.Run(file, func(t *testing.T) {
			raw, err := os.ReadFile(filepath.Join(".", file))
			if err != nil {
				t.Fatalf("read %s: %v", file, err)
			}
			text := strings.ToLower(string(raw))
			for _, term := range []string{"content-modeling", "content type", "block-library", "cm-"} {
				if strings.Contains(text, term) {
					t.Fatalf("generic shell source %s contains consumer-specific term %q", file, term)
				}
			}
		})
	}
}
