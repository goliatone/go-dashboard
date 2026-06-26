package dashboard

import (
	"encoding/json"
	"html/template"
	"testing"
)

func TestShellNormalizeDefaultsAndStorageKey(t *testing.T) {
	shell := Shell{
		SurfaceID: "workbench",
		Storage: ShellStorage{
			ModuleID: "settings",
		},
		Regions: []ShellRegion{
			{
				ID:          "nav",
				Role:        ShellRegionRoleNavigation,
				Placement:   ShellRegionPlacementLeading,
				Collapsible: true,
				Resizable:   true,
				Sizing:      ShellPaneSizing{Min: 240, Max: 420, Default: 999},
			},
			{
				ID:          "main",
				Role:        ShellRegionRoleMain,
				FocusTarget: true,
				Content:     ShellRegionContent{HTML: template.HTML("<strong>Owned by host</strong>")},
			},
		},
	}

	normalized, err := shell.Normalize()
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if normalized.Storage.Namespace != DefaultShellStateNamespace {
		t.Fatalf("expected default namespace, got %q", normalized.Storage.Namespace)
	}
	if normalized.Storage.Version != DefaultShellStateVersion {
		t.Fatalf("expected default version, got %d", normalized.Storage.Version)
	}
	if normalized.Regions[0].Sizing.Default != 420 {
		t.Fatalf("expected default width clamped to max, got %+v", normalized.Regions[0].Sizing)
	}
	if normalized.Regions[1].Placement != ShellRegionPlacementMain {
		t.Fatalf("expected main role to default to main placement, got %q", normalized.Regions[1].Placement)
	}
	if len(normalized.FocusTargets) != 1 || normalized.FocusTargets[0].ID != "main" {
		t.Fatalf("expected focus target derived from region, got %+v", normalized.FocusTargets)
	}
	key := normalized.Storage.StorageKey(normalized.SurfaceID)
	want := "go-dashboard:shell:v1:workbench:module:settings:viewer:anonymous"
	if key != want {
		t.Fatalf("expected storage key %q, got %q", want, key)
	}
}

func TestShellNormalizeRejectsInvalidDefinitions(t *testing.T) {
	cases := []struct {
		name  string
		shell Shell
	}{
		{
			name: "missing surface",
			shell: Shell{
				Regions: []ShellRegion{{ID: "main", Role: ShellRegionRoleMain}},
			},
		},
		{
			name: "duplicate region",
			shell: Shell{
				SurfaceID: "surface",
				Regions: []ShellRegion{
					{ID: "main", Role: ShellRegionRoleMain},
					{ID: "main", Role: ShellRegionRolePreview},
				},
			},
		},
		{
			name: "invalid role",
			shell: Shell{
				SurfaceID: "surface",
				Regions:   []ShellRegion{{ID: "main", Role: "content-type"}},
			},
		},
		{
			name: "unknown focus target",
			shell: Shell{
				SurfaceID:    "surface",
				Regions:      []ShellRegion{{ID: "main", Role: ShellRegionRoleMain}},
				FocusTargets: []ShellFocusTarget{{ID: "missing"}},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := tc.shell.Normalize(); err == nil {
				t.Fatalf("expected invalid shell to fail")
			}
		})
	}
}

func TestShellNormalizeValidatesActions(t *testing.T) {
	_, err := (Shell{
		SurfaceID: "surface",
		Regions:   []ShellRegion{{ID: "main", Role: ShellRegionRoleMain}},
		Actions: []ShellAction{{
			ID:   "bad-toggle",
			Kind: ShellActionKindToggleRegion,
		}},
	}).Normalize()
	if err == nil {
		t.Fatalf("expected toggle action without region to fail")
	}

	normalized, err := (Shell{
		SurfaceID: "surface",
		Regions: []ShellRegion{{
			ID:          "inspector",
			Role:        ShellRegionRoleInspector,
			Placement:   ShellRegionPlacementTrailing,
			Collapsible: true,
			Collapsed:   true,
		}},
		Actions: []ShellAction{{
			ID:       "toggle-inspector",
			Kind:     ShellActionKindToggleRegion,
			RegionID: "inspector",
		}},
	}).Normalize()
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	payload := normalized.legacyPayload()
	actions := payload["actions"].([]map[string]any)
	if actions[0]["expanded"] != false {
		t.Fatalf("expected toggle action expansion derived from collapsed region, got %+v", actions[0])
	}

	normalized, err = (Shell{
		SurfaceID: "surface",
		Regions:   []ShellRegion{{ID: "main", Role: ShellRegionRoleMain}},
		Actions: []ShellAction{{
			ID:       "focus-main",
			Kind:     ShellActionKindFocus,
			TargetID: "main",
		}},
	}).Normalize()
	if err != nil {
		t.Fatalf("Normalize returned error: %v", err)
	}
	if !normalized.Regions[0].FocusTarget {
		t.Fatalf("expected focus action to mark target region as focusable")
	}
	if len(normalized.FocusTargets) != 1 || normalized.FocusTargets[0].ID != "main" {
		t.Fatalf("expected focus action to populate focus target payload, got %+v", normalized.FocusTargets)
	}
}

func TestPageLegacyPayloadIncludesShellOnlyWhenConfigured(t *testing.T) {
	page := Page{
		Title: "Dashboard",
		Areas: []PageArea{
			{Slot: "main", Code: "admin.dashboard.main"},
			{Slot: "sidebar", Code: "admin.dashboard.sidebar"},
			{Slot: "footer", Code: "admin.dashboard.footer"},
		},
	}
	payload := page.LegacyPayload()
	if _, ok := payload["shell"]; ok {
		t.Fatalf("expected no shell payload for ordinary dashboard pages")
	}
	areas := payload["areas"].(map[string]any)
	if _, ok := areas["main"]; !ok {
		t.Fatalf("expected existing main area payload to remain available")
	}

	page.Shell = &Shell{
		SurfaceID: "workbench",
		Regions: []ShellRegion{
			{
				ID:        "main",
				Role:      ShellRegionRoleMain,
				Placement: ShellRegionPlacementMain,
				Content:   ShellRegionContent{Text: "Host content"},
			},
		},
	}
	payload = page.LegacyPayload()
	shell, ok := payload["shell"].(map[string]any)
	if !ok {
		t.Fatalf("expected shell payload when page shell is configured, got %+v", payload["shell"])
	}
	if shell["surface_id"] != "workbench" {
		t.Fatalf("expected shell surface id in payload, got %+v", shell)
	}
}

func TestPageJSONNormalizesShellAndRejectsInvalidShell(t *testing.T) {
	page := Page{
		Title: "Workbench",
		Shell: &Shell{
			SurfaceID: "workbench",
			Regions: []ShellRegion{{
				ID:          "main",
				Role:        ShellRegionRoleMain,
				Resizable:   true,
				Sizing:      ShellPaneSizing{Min: 240, Max: 320, Default: 999},
				FocusTarget: true,
			}},
		},
	}

	raw, err := json.Marshal(page)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	shell := payload["shell"].(map[string]any)
	storage := shell["storage"].(map[string]any)
	if storage["namespace"] != DefaultShellStateNamespace {
		t.Fatalf("expected normalized shell storage, got %+v", storage)
	}
	regions := shell["regions"].([]any)
	region := regions[0].(map[string]any)
	if region["placement"] != ShellRegionPlacementMain {
		t.Fatalf("expected normalized main placement, got %+v", region)
	}
	sizing := region["sizing"].(map[string]any)
	if sizing["default"].(float64) != 320 {
		t.Fatalf("expected normalized sizing in JSON, got %+v", sizing)
	}

	page.Shell.SurfaceID = "bad surface"
	if _, err := json.Marshal(page); err == nil {
		t.Fatalf("expected invalid shell to fail JSON marshaling")
	}
}
