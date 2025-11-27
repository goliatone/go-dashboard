package dashboard

import (
	"context"
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
		if name == "" {
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
	var builder strings.Builder
	for key, value := range vars {
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

func normalizeCSSVariable(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.HasPrefix(name, "--") {
		return name
	}
	return "--" + name
}

func cloneThemeSelection(selection *ThemeSelection) *ThemeSelection {
	if selection == nil {
		return nil
	}
	cloned := *selection
	if len(selection.Tokens) > 0 {
		cloned.Tokens = make(map[string]string, len(selection.Tokens))
		for key, value := range selection.Tokens {
			cloned.Tokens[key] = value
		}
	}
	if len(selection.Templates) > 0 {
		cloned.Templates = make(map[string]string, len(selection.Templates))
		for key, value := range selection.Templates {
			cloned.Templates[key] = value
		}
	}
	if len(selection.Assets.Values) > 0 {
		cloned.Assets.Values = make(map[string]string, len(selection.Assets.Values))
		for key, value := range selection.Assets.Values {
			cloned.Assets.Values[key] = value
		}
	}
	return &cloned
}
