package dashboard

import (
	"strings"
	"testing"
)

func TestThemeSelectionCSSVariablesSanitizeUnsafeTokens(t *testing.T) {
	selection := &ThemeSelection{
		Tokens: map[string]string{
			"dashboard-accent": "#22d3ee",
			"bad-name;":        "#fff",
			"dashboard-bg":     `url("javascript:alert(1)")`,
			"dashboard-gap":    "1.5rem",
		},
	}

	vars := selection.CSSVariables()
	if vars["--dashboard-accent"] != "#22d3ee" {
		t.Fatalf("expected safe accent token preserved, got %+v", vars)
	}
	if vars["--dashboard-gap"] != "1.5rem" {
		t.Fatalf("expected safe numeric token preserved, got %+v", vars)
	}
	if _, ok := vars["--bad-name;"]; ok {
		t.Fatalf("expected unsafe variable name rejected, got %+v", vars)
	}
	if _, ok := vars["--dashboard-bg"]; ok {
		t.Fatalf("expected unsafe css value rejected, got %+v", vars)
	}

	inline := selection.CSSVariablesInline()
	if inline == "" {
		t.Fatalf("expected safe css variables inline output")
	}
	if strings.Contains(inline, "url(") || strings.Contains(inline, "javascript:") || strings.Contains(inline, "--bad-name") {
		t.Fatalf("expected inline css sanitized, got %q", inline)
	}
}
