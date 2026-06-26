package dashboard

import (
	"fmt"
	"html/template"
	"regexp"
	"strings"
)

const (
	DefaultShellStateNamespace = "go-dashboard:shell"
	DefaultShellStateVersion   = 1

	ShellRegionRoleNavigation = "navigation"
	ShellRegionRoleMain       = "main"
	ShellRegionRolePalette    = "palette"
	ShellRegionRolePreview    = "preview"
	ShellRegionRoleInspector  = "inspector"
	ShellRegionRoleFooter     = "footer"
	ShellRegionRoleUtility    = "utility"

	ShellRegionPlacementLeading  = "leading"
	ShellRegionPlacementMain     = "main"
	ShellRegionPlacementTrailing = "trailing"
	ShellRegionPlacementFooter   = "footer"
	ShellRegionPlacementUtility  = "utility"

	ShellResizeEdgeLeading  = "leading"
	ShellResizeEdgeTrailing = "trailing"

	ShellActionKindButton       = "button"
	ShellActionKindToggleRegion = "toggle-region"
	ShellActionKindFocus        = "focus"
	ShellActionKindExitFocus    = "exit-focus"
)

var shellTokenPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]*$`)

// Shell describes an opt-in dashboard application shell. Shell regions compose
// app chrome and workbench panes; widget PageArea placement remains unchanged.
type Shell struct {
	SurfaceID      string             `json:"surface_id"`
	Label          string             `json:"label,omitempty"`
	Storage        ShellStorage       `json:"storage,omitempty"`
	Regions        []ShellRegion      `json:"regions"`
	Actions        []ShellAction      `json:"actions,omitempty"`
	FocusTargets   []ShellFocusTarget `json:"focus_targets,omitempty"`
	ThemeVariables map[string]string  `json:"theme_variables,omitempty"`
	Attributes     map[string]string  `json:"attributes,omitempty"`
}

// ShellStorage controls browser-state scoping. ViewerID and ModuleID are
// optional because server-rendered pages may not expose identity to client code.
type ShellStorage struct {
	Namespace string `json:"namespace,omitempty"`
	Version   int    `json:"version,omitempty"`
	ViewerID  string `json:"viewer_id,omitempty"`
	ModuleID  string `json:"module_id,omitempty"`
}

// ShellRegion is one rendered shell pane or rail.
type ShellRegion struct {
	ID          string             `json:"id"`
	Role        string             `json:"role"`
	Placement   string             `json:"placement"`
	Label       string             `json:"label,omitempty"`
	Content     ShellRegionContent `json:"content,omitempty"`
	Collapsible bool               `json:"collapsible,omitempty"`
	Collapsed   bool               `json:"collapsed,omitempty"`
	Resizable   bool               `json:"resizable,omitempty"`
	ResizeEdge  string             `json:"resize_edge,omitempty"`
	Sizing      ShellPaneSizing    `json:"sizing,omitempty"`
	FocusTarget bool               `json:"focus_target,omitempty"`
	Attributes  map[string]string  `json:"attributes,omitempty"`
}

// ShellRegionContent represents module-owned content. HTML must be trusted by
// the caller; plain text remains escaped by templates.
type ShellRegionContent struct {
	HTML template.HTML `json:"html,omitempty"`
	Text string        `json:"text,omitempty"`
}

// ShellPaneSizing describes resizable pane dimensions in CSS pixels.
type ShellPaneSizing struct {
	Default int `json:"default,omitempty"`
	Min     int `json:"min,omitempty"`
	Max     int `json:"max,omitempty"`
}

// ShellAction describes a control rendered in shell chrome.
type ShellAction struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	RegionID string `json:"region_id,omitempty"`
	Kind     string `json:"kind,omitempty"`
	TargetID string `json:"target_id,omitempty"`
	Pressed  bool   `json:"pressed,omitempty"`
	Expanded *bool  `json:"expanded,omitempty"`
}

// ShellFocusTarget declares panes that can enter focus/maximize mode.
type ShellFocusTarget struct {
	ID     string `json:"id"`
	Label  string `json:"label,omitempty"`
	Active bool   `json:"active,omitempty"`
}

// Normalize validates the shell and returns a copy with safe defaults applied.
func (shell Shell) Normalize() (Shell, error) {
	shell.SurfaceID = strings.TrimSpace(shell.SurfaceID)
	if shell.SurfaceID == "" {
		return Shell{}, fmt.Errorf("dashboard shell: surface id is required")
	}
	if !validShellToken(shell.SurfaceID) {
		return Shell{}, fmt.Errorf("dashboard shell: invalid surface id %q", shell.SurfaceID)
	}
	if shell.Storage.Namespace == "" {
		shell.Storage.Namespace = DefaultShellStateNamespace
	}
	if shell.Storage.Version <= 0 {
		shell.Storage.Version = DefaultShellStateVersion
	}
	if len(shell.Regions) == 0 {
		return Shell{}, fmt.Errorf("dashboard shell: at least one region is required")
	}

	seenRegions := map[string]bool{}
	focusTargets := map[string]bool{}
	for idx := range shell.Regions {
		region, err := normalizeShellRegion(shell.Regions[idx])
		if err != nil {
			return Shell{}, err
		}
		if seenRegions[region.ID] {
			return Shell{}, fmt.Errorf("dashboard shell: duplicate region id %q", region.ID)
		}
		seenRegions[region.ID] = true
		if region.FocusTarget {
			focusTargets[region.ID] = true
		}
		shell.Regions[idx] = region
	}

	for idx := range shell.FocusTargets {
		target := shell.FocusTargets[idx]
		target.ID = strings.TrimSpace(target.ID)
		if !seenRegions[target.ID] {
			return Shell{}, fmt.Errorf("dashboard shell: focus target %q does not match a region", target.ID)
		}
		if target.Label == "" {
			target.Label = target.ID
		}
		focusTargets[target.ID] = true
		shell.FocusTargets[idx] = target
	}
	if len(shell.FocusTargets) == 0 && len(focusTargets) > 0 {
		for _, region := range shell.Regions {
			if focusTargets[region.ID] {
				shell.FocusTargets = append(shell.FocusTargets, ShellFocusTarget{ID: region.ID, Label: region.Label})
			}
		}
	}

	for idx := range shell.Actions {
		action, err := normalizeShellAction(shell.Actions[idx], seenRegions)
		if err != nil {
			return Shell{}, err
		}
		if action.Kind == ShellActionKindFocus {
			focusTargets[action.TargetID] = true
		}
		shell.Actions[idx] = action
	}

	for idx := range shell.Regions {
		if focusTargets[shell.Regions[idx].ID] {
			shell.Regions[idx].FocusTarget = true
		}
	}
	if len(shell.FocusTargets) == 0 && len(focusTargets) > 0 {
		for _, region := range shell.Regions {
			if focusTargets[region.ID] {
				shell.FocusTargets = append(shell.FocusTargets, ShellFocusTarget{ID: region.ID, Label: region.Label})
			}
		}
	} else if len(shell.FocusTargets) > 0 {
		seenFocus := map[string]bool{}
		for _, target := range shell.FocusTargets {
			seenFocus[target.ID] = true
		}
		for _, region := range shell.Regions {
			if focusTargets[region.ID] && !seenFocus[region.ID] {
				shell.FocusTargets = append(shell.FocusTargets, ShellFocusTarget{ID: region.ID, Label: region.Label})
			}
		}
	}

	return shell, nil
}

// StorageKey returns the browser-storage key shape used by the shell runtime.
func (storage ShellStorage) StorageKey(surfaceID string) string {
	namespace := storage.Namespace
	if namespace == "" {
		namespace = DefaultShellStateNamespace
	}
	version := storage.Version
	if version <= 0 {
		version = DefaultShellStateVersion
	}
	parts := []string{namespace, fmt.Sprintf("v%d", version), surfaceID}
	if storage.ModuleID != "" {
		parts = append(parts, "module", storage.ModuleID)
	}
	if storage.ViewerID != "" {
		parts = append(parts, "viewer", storage.ViewerID)
	} else {
		parts = append(parts, "viewer", "anonymous")
	}
	return strings.Join(parts, ":")
}

func normalizeShellRegion(region ShellRegion) (ShellRegion, error) {
	region.ID = strings.TrimSpace(region.ID)
	if region.ID == "" || !validShellToken(region.ID) {
		return ShellRegion{}, fmt.Errorf("dashboard shell: invalid region id %q", region.ID)
	}
	if region.Role == "" {
		region.Role = ShellRegionRoleUtility
	}
	if !validShellRegionRole(region.Role) {
		return ShellRegion{}, fmt.Errorf("dashboard shell: invalid region role %q", region.Role)
	}
	if region.Placement == "" {
		if region.Role == ShellRegionRoleMain {
			region.Placement = ShellRegionPlacementMain
		} else {
			region.Placement = ShellRegionPlacementUtility
		}
	}
	if !validShellRegionPlacement(region.Placement) {
		return ShellRegion{}, fmt.Errorf("dashboard shell: invalid region placement %q", region.Placement)
	}
	if region.Label == "" {
		region.Label = region.ID
	}
	if region.ResizeEdge == "" {
		if region.Placement == ShellRegionPlacementTrailing {
			region.ResizeEdge = ShellResizeEdgeLeading
		} else {
			region.ResizeEdge = ShellResizeEdgeTrailing
		}
	}
	if region.ResizeEdge != ShellResizeEdgeLeading && region.ResizeEdge != ShellResizeEdgeTrailing {
		return ShellRegion{}, fmt.Errorf("dashboard shell: invalid resize edge %q", region.ResizeEdge)
	}
	if region.Resizable {
		region.Sizing = normalizeShellPaneSizing(region.Sizing)
	} else {
		region.Sizing = ShellPaneSizing{}
	}
	return region, nil
}

func normalizeShellPaneSizing(sizing ShellPaneSizing) ShellPaneSizing {
	if sizing.Min <= 0 {
		sizing.Min = 160
	}
	if sizing.Max <= 0 {
		sizing.Max = 640
	}
	if sizing.Max < sizing.Min {
		sizing.Min, sizing.Max = sizing.Max, sizing.Min
	}
	if sizing.Default <= 0 {
		sizing.Default = (sizing.Min + sizing.Max) / 2
	}
	if sizing.Default < sizing.Min {
		sizing.Default = sizing.Min
	}
	if sizing.Default > sizing.Max {
		sizing.Default = sizing.Max
	}
	return sizing
}

func normalizeShellAction(action ShellAction, regions map[string]bool) (ShellAction, error) {
	action.ID = strings.TrimSpace(action.ID)
	if action.ID == "" || !validShellToken(action.ID) {
		return ShellAction{}, fmt.Errorf("dashboard shell: invalid action id %q", action.ID)
	}
	if action.Label == "" {
		action.Label = action.ID
	}
	if action.Kind == "" {
		action.Kind = ShellActionKindButton
	}
	switch action.Kind {
	case ShellActionKindButton:
	case ShellActionKindToggleRegion:
		if action.RegionID == "" {
			return ShellAction{}, fmt.Errorf("dashboard shell: toggle action %q requires a region id", action.ID)
		}
	case ShellActionKindFocus:
		if action.TargetID == "" {
			return ShellAction{}, fmt.Errorf("dashboard shell: focus action %q requires a target id", action.ID)
		}
	case ShellActionKindExitFocus:
	default:
		return ShellAction{}, fmt.Errorf("dashboard shell: invalid action kind %q", action.Kind)
	}
	if action.RegionID != "" && !regions[action.RegionID] {
		return ShellAction{}, fmt.Errorf("dashboard shell: action %q references unknown region %q", action.ID, action.RegionID)
	}
	if action.TargetID != "" && !regions[action.TargetID] {
		return ShellAction{}, fmt.Errorf("dashboard shell: action %q references unknown target %q", action.ID, action.TargetID)
	}
	return action, nil
}

func validShellToken(value string) bool {
	return shellTokenPattern.MatchString(value)
}

func validShellRegionRole(value string) bool {
	switch value {
	case ShellRegionRoleNavigation, ShellRegionRoleMain, ShellRegionRolePalette,
		ShellRegionRolePreview, ShellRegionRoleInspector, ShellRegionRoleFooter,
		ShellRegionRoleUtility:
		return true
	default:
		return false
	}
}

func validShellRegionPlacement(value string) bool {
	switch value {
	case ShellRegionPlacementLeading, ShellRegionPlacementMain, ShellRegionPlacementTrailing,
		ShellRegionPlacementFooter, ShellRegionPlacementUtility:
		return true
	default:
		return false
	}
}

func (shell Shell) legacyPayload() map[string]any {
	normalized, err := shell.Normalize()
	if err != nil {
		return nil
	}
	regions := make([]map[string]any, 0, len(normalized.Regions))
	regionByID := make(map[string]any, len(normalized.Regions))
	regionStates := make(map[string]ShellRegion, len(normalized.Regions))
	for _, region := range normalized.Regions {
		payload := region.legacyPayload()
		regions = append(regions, payload)
		regionByID[region.ID] = payload
		regionStates[region.ID] = region
	}
	focusTargets := make([]map[string]any, 0, len(normalized.FocusTargets))
	for _, target := range normalized.FocusTargets {
		focusTargets = append(focusTargets, map[string]any{
			"id":     target.ID,
			"label":  target.Label,
			"active": target.Active,
		})
	}
	return map[string]any{
		"surface_id":      normalized.SurfaceID,
		"label":           normalized.Label,
		"storage":         normalized.Storage.legacyPayload(normalized.SurfaceID),
		"regions":         regions,
		"region_by_id":    regionByID,
		"actions":         shellActionsPayload(normalized.Actions, regionStates),
		"focus_targets":   focusTargets,
		"theme_variables": normalized.ThemeVariables,
		"attributes":      normalized.Attributes,
	}
}

func (storage ShellStorage) legacyPayload(surfaceID string) map[string]any {
	return map[string]any{
		"namespace": storage.Namespace,
		"version":   storage.Version,
		"viewer_id": storage.ViewerID,
		"module_id": storage.ModuleID,
		"key":       storage.StorageKey(surfaceID),
	}
}

func (region ShellRegion) legacyPayload() map[string]any {
	return map[string]any{
		"id":            region.ID,
		"role":          region.Role,
		"placement":     region.Placement,
		"label":         region.Label,
		"content":       region.Content.legacyPayload(),
		"collapsible":   region.Collapsible,
		"collapsed":     region.Collapsed,
		"resizable":     region.Resizable,
		"resize_edge":   region.ResizeEdge,
		"resize_before": region.Resizable && region.ResizeEdge == ShellResizeEdgeLeading,
		"resize_after":  region.Resizable && region.ResizeEdge == ShellResizeEdgeTrailing,
		"sizing":        region.Sizing.legacyPayload(),
		"focus_target":  region.FocusTarget,
		"attributes":    region.Attributes,
	}
}

func (content ShellRegionContent) legacyPayload() map[string]any {
	return map[string]any{
		"html": content.HTML,
		"text": content.Text,
	}
}

func (sizing ShellPaneSizing) legacyPayload() map[string]any {
	return map[string]any{
		"default": sizing.Default,
		"min":     sizing.Min,
		"max":     sizing.Max,
	}
}

func shellActionsPayload(actions []ShellAction, regions map[string]ShellRegion) []map[string]any {
	if len(actions) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(actions))
	for _, action := range actions {
		expanded := false
		if action.Expanded != nil {
			expanded = *action.Expanded
		} else if action.Kind == ShellActionKindToggleRegion {
			if region, ok := regions[action.RegionID]; ok {
				expanded = !region.Collapsed
			}
		}
		out = append(out, map[string]any{
			"id":             action.ID,
			"label":          action.Label,
			"region_id":      action.RegionID,
			"kind":           action.Kind,
			"target_id":      action.TargetID,
			"pressed":        action.Pressed,
			"expanded":       expanded,
			"toggle_region":  action.Kind == ShellActionKindToggleRegion,
			"focus":          action.Kind == ShellActionKindFocus,
			"exit_focus":     action.Kind == ShellActionKindExitFocus,
			"button":         action.Kind == ShellActionKindButton,
			"button_pressed": action.Kind == ShellActionKindButton && action.Pressed,
		})
	}
	return out
}
