package main

import (
	"html/template"

	"github.com/goliatone/go-dashboard/components/dashboard"
)

func exampleShellPage() dashboard.Page {
	assets := dashboard.PageAssets{}
	assets.AddShellAssets("")
	return dashboard.Page{
		Title:       "Dashboard Shell Example",
		Description: "Domain-neutral workbench shell",
		Assets:      &assets,
		Shell: &dashboard.Shell{
			SurfaceID: "demo-workbench",
			Label:     "Demo workbench",
			Regions: []dashboard.ShellRegion{
				{
					ID:          "nav",
					Role:        dashboard.ShellRegionRoleNavigation,
					Placement:   dashboard.ShellRegionPlacementLeading,
					Label:       "Navigation",
					Collapsible: true,
					Resizable:   true,
					Sizing:      dashboard.ShellPaneSizing{Min: 220, Default: 280, Max: 420},
					Content:     dashboard.ShellRegionContent{HTML: template.HTML("<nav>Overview<br>Reports<br>Settings</nav>")},
				},
				{
					ID:          "workspace",
					Role:        dashboard.ShellRegionRoleMain,
					Placement:   dashboard.ShellRegionPlacementMain,
					Label:       "Workspace",
					FocusTarget: true,
					Content:     dashboard.ShellRegionContent{HTML: template.HTML("<main><h2>Workspace</h2><p>Module-owned content renders here.</p></main>")},
				},
				{
					ID:          "inspector",
					Role:        dashboard.ShellRegionRoleInspector,
					Placement:   dashboard.ShellRegionPlacementTrailing,
					Label:       "Inspector",
					Collapsible: true,
					Resizable:   true,
					ResizeEdge:  dashboard.ShellResizeEdgeLeading,
					Sizing:      dashboard.ShellPaneSizing{Min: 240, Default: 320, Max: 520},
					Content:     dashboard.ShellRegionContent{Text: "Selection details"},
				},
			},
		},
	}
}
