package dashboard

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"strings"
)

// ThemeProvider matches the go-theme provider interface used by adapters. It is
// optional; when absent the dashboard behaves exactly as before.
type ThemeProvider interface {
	SelectTheme(ctx context.Context, selector ThemeSelector) (*ThemeSelection, error)
}

// ThemeSelectorFunc chooses the theme name/variant for a given viewer.
type ThemeSelectorFunc func(ctx context.Context, viewer ViewerContext) ThemeSelector

// ThemeSelector describes the desired theme/variant.
type ThemeSelector struct {
	Name    string
	Variant string
}

// ThemeSelection carries resolved theme details (tokens, assets, templates).
type ThemeSelection struct {
	Name       string
	Variant    string
	Tokens     map[string]string
	Assets     ThemeAssets
	Templates  map[string]string
	ChartTheme string
}

// ThemeAssets provides asset metadata plus optional prefix/resolver.
type ThemeAssets struct {
	Values   map[string]string
	Prefix   string
	Resolver func(string) string
}

// AssetURL resolves the final URL for a named asset (logo, favicon, etc.).
func (assets ThemeAssets) AssetURL(name string) string {
	if len(assets.Values) == 0 {
		return ""
	}
	path := assets.Values[name]
	if path == "" {
		return ""
	}
	if assets.Resolver != nil {
		if resolved := assets.Resolver(path); resolved != "" {
			return resolved
		}
	}
	if assets.Prefix != "" {
		return strings.TrimRight(assets.Prefix, "/") + "/" + strings.TrimLeft(path, "/")
	}
	return path
}

// Resolved returns a map of asset keys to resolved URLs.
func (assets ThemeAssets) Resolved() map[string]string {
	if len(assets.Values) == 0 {
		return nil
	}
	out := make(map[string]string, len(assets.Values))
	for key := range assets.Values {
		if url := assets.AssetURL(key); url != "" {
			out[key] = url
		}
	}
	return out
}

// CSSVariables normalizes token keys into CSS variable names.
func (theme *ThemeSelection) CSSVariables() map[string]string {
	if theme == nil || len(theme.Tokens) == 0 {
		return nil
	}
	vars := make(map[string]string, len(theme.Tokens))
	for key, value := range theme.Tokens {
		name := normalizeCSSVariable(key)
		value = sanitizeCSSVariableValue(value)
		if name == "" || value == "" {
			continue
		}
		vars[name] = value
	}
	return vars
}

// CSSVariablesInline renders the CSS variable map as a style string.
func (theme *ThemeSelection) CSSVariablesInline() string {
	vars := theme.CSSVariables()
	if len(vars) == 0 {
		return ""
	}
	keys := make([]string, 0, len(vars))
	for key := range vars {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	var builder strings.Builder
	for _, key := range keys {
		value := vars[key]
		if value == "" {
			continue
		}
		builder.WriteString(key)
		builder.WriteString(": ")
		builder.WriteString(value)
		builder.WriteString("; ")
	}
	return strings.TrimSpace(builder.String())
}

// AssetURL resolves a named asset using the selection assets.
func (theme *ThemeSelection) AssetURL(name string) string {
	if theme == nil {
		return ""
	}
	return theme.Assets.AssetURL(name)
}

// TemplatePath retrieves a theme-specific template if present.
func (theme *ThemeSelection) TemplatePath(key string) string {
	if theme == nil || len(theme.Templates) == 0 {
		return ""
	}
	return theme.Templates[key]
}

// MarshalJSON preserves the canonical dashboard theme transport shape while
// reusing ThemeSelection directly on the typed page contract.
func (theme *ThemeSelection) MarshalJSON() ([]byte, error) {
	return json.Marshal(themePayload(theme))
}

func normalizeCSSVariable(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "--") {
		if isSafeCSSVariableName(name) {
			return name
		}
		return ""
	}
	name = "--" + name
	if !isSafeCSSVariableName(name) {
		return ""
	}
	return name
}

func isSafeCSSVariableName(name string) bool {
	if !strings.HasPrefix(name, "--") || len(name) < 3 {
		return false
	}
	for _, r := range name[2:] {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}

func sanitizeCSSVariableValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	lower := strings.ToLower(value)
	if strings.ContainsAny(value, `;{}<>"'`+"`") {
		return ""
	}
	if strings.ContainsRune(value, '\n') || strings.ContainsRune(value, '\r') || strings.ContainsRune(value, '\x00') {
		return ""
	}
	for _, token := range []string{"url(", "expression(", "@import", "javascript:", "vbscript:", "data:"} {
		if strings.Contains(lower, token) {
			return ""
		}
	}
	return value
}

func cloneThemeSelection(selection *ThemeSelection) *ThemeSelection {
	if selection == nil {
		return nil
	}
	cloned := *selection
	if len(selection.Tokens) > 0 {
		cloned.Tokens = make(map[string]string, len(selection.Tokens))
		maps.Copy(cloned.Tokens, selection.Tokens)
	}
	if len(selection.Templates) > 0 {
		cloned.Templates = make(map[string]string, len(selection.Templates))
		maps.Copy(cloned.Templates, selection.Templates)
	}
	if len(selection.Assets.Values) > 0 {
		cloned.Assets.Values = make(map[string]string, len(selection.Assets.Values))
		maps.Copy(cloned.Assets.Values, selection.Assets.Values)
	}
	return &cloned
}

func themePayload(selection *ThemeSelection) map[string]any {
	if selection == nil {
		return nil
	}
	payload := map[string]any{}
	if selection.Name != "" {
		payload["name"] = selection.Name
	}
	if selection.Variant != "" {
		payload["variant"] = selection.Variant
	}
	if len(selection.Tokens) > 0 {
		payload["tokens"] = selection.Tokens
	}
	if cssVars := selection.CSSVariables(); len(cssVars) > 0 {
		payload["css_vars"] = cssVars
	}
	if inline := selection.CSSVariablesInline(); inline != "" {
		payload["css_vars_inline"] = inline
	}
	if selection.Assets.Prefix != "" {
		payload["asset_prefix"] = selection.Assets.Prefix
	}
	if assets := selection.Assets.Resolved(); len(assets) > 0 {
		payload["assets"] = assets
	}
	if len(selection.Templates) > 0 {
		payload["templates"] = selection.Templates
	}
	if selection.ChartTheme != "" {
		payload["chart_theme"] = selection.ChartTheme
	}
	return payload
}
