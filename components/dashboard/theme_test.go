package dashboard

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

const canonicalSemanticThemeFixtureSHA256 = "f59a074a2082da29c0ceec7594e9d4e7bf9af082282b97f7a7396e08d0f4e712"

func TestThemeSelectionCSSVariablesSanitizeUnsafeTokens(t *testing.T) {
	selection := &ThemeSelection{
		Tokens: map[string]string{
			"dashboard-accent":       "#22d3ee",
			"dashboard-shell-bg":     "#0f172a",
			"dashboard-shell-radius": "6px",
			"bad-name;":              "#fff",
			"dashboard-bg":           `url("javascript:alert(1)")`,
			"dashboard-gap":          "1.5rem",
		},
	}

	vars := selection.CSSVariables()
	if vars["--dashboard-accent"] != "#22d3ee" {
		t.Fatalf("expected safe accent token preserved, got %+v", vars)
	}
	if vars["--dashboard-gap"] != "1.5rem" {
		t.Fatalf("expected safe numeric token preserved, got %+v", vars)
	}
	if vars["--dashboard-shell-bg"] != "#0f172a" || vars["--dashboard-shell-radius"] != "6px" {
		t.Fatalf("expected shell tokens preserved, got %+v", vars)
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

func TestPortableSemanticProjectionMatchesCanonicalFixture(t *testing.T) {
	data, err := os.ReadFile("testdata/semantic-theme-projection.json")
	if err != nil {
		t.Fatalf("read semantic projection fixture: %v", err)
	}
	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != canonicalSemanticThemeFixtureSHA256 {
		t.Fatalf("canonical fixture drifted: want %s, got %s", canonicalSemanticThemeFixtureSHA256, got)
	}

	var fixture semanticThemeFixture
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatalf("decode semantic projection fixture: %v", err)
	}
	if fixture.Version != 1 {
		t.Fatalf("unsupported semantic fixture version %d", fixture.Version)
	}

	profile := PortableSemanticProfile()
	for _, test := range fixture.Cases {
		t.Run(test.Name, func(t *testing.T) {
			tokens := maps.Clone(test.Tokens)
			maps.Copy(tokens, test.VariantTokens)
			projection := ProjectThemeCSSVariables(tokens, ProjectionOptions{Profile: &profile})
			if !reflect.DeepEqual(projection.Variables, test.Variables) {
				t.Fatalf("variables mismatch:\nwant: %#v\ngot:  %#v", test.Variables, projection.Variables)
			}
			if projection.Inline != test.Inline {
				t.Fatalf("inline mismatch:\nwant: %q\ngot:  %q", test.Inline, projection.Inline)
			}
			if got := diagnosticStatuses(projection.Diagnostics); !reflect.DeepEqual(got, test.Statuses) {
				t.Fatalf("diagnostic statuses mismatch:\nwant: %#v\ngot:  %#v", test.Statuses, got)
			}
		})
	}
}

func TestDashboardSemanticProjectionSupportsExtensionsAndDiagnostics(t *testing.T) {
	selection := &ThemeSelection{Tokens: map[string]string{
		"dashboard.card.background": "#ffffff",
		"color.surface.raised":      "#f8fafc",
		"custom.safe":               "calc(100% - 1rem)",
	}}
	projection := selection.SemanticProjection()

	if projection.Variables["--dashboard-card-background"] != "#ffffff" {
		t.Fatalf("dashboard extension was not projected: %#v", projection.Variables)
	}
	statuses := diagnosticStatuses(projection.Diagnostics)
	for _, expected := range []string{
		"dashboard.card.background:resolved",
		"dashboard.card.background:supported",
		"custom.safe:resolved",
		"custom.safe:unsupported",
	} {
		if !slicesContain(statuses, expected) {
			t.Fatalf("missing status %q from %#v", expected, statuses)
		}
	}

	diagnostics := projection.ConsumerDiagnostics("dashboard.card", "dashboard.card.background")
	statuses = diagnosticStatuses(diagnostics)
	if !slicesContain(statuses, "dashboard.card.background:consumed") {
		t.Fatalf("expected consumed dashboard token, got %#v", statuses)
	}
	if !slicesContain(statuses, "color.surface.raised:unused") {
		t.Fatalf("expected unused supported fallback token, got %#v", statuses)
	}
}

func TestResolveSemanticTokenUsesValidatedFallbackChain(t *testing.T) {
	tests := []struct {
		name      string
		tokens    map[string]string
		wantToken string
		wantValue string
	}{
		{
			name: "package override",
			tokens: map[string]string{
				"dashboard.card.background": "#111827",
				"color.surface.raised":      "#f8fafc",
			},
			wantToken: "dashboard.card.background",
			wantValue: "#111827",
		},
		{
			name: "portable fallback",
			tokens: map[string]string{
				"color.surface.raised": "#f8fafc",
			},
			wantToken: "color.surface.raised",
			wantValue: "#f8fafc",
		},
		{
			name: "invalid package token falls through",
			tokens: map[string]string{
				"dashboard.card.background": "url(https://example.test/image)",
				"color.surface.raised":      "#f8fafc",
			},
			wantToken: "color.surface.raised",
			wantValue: "#f8fafc",
		},
		{
			name:      "current default",
			tokens:    map[string]string{},
			wantValue: "#ffffff",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			selection := &ThemeSelection{Tokens: test.tokens}
			got := selection.ResolveSemanticToken(
				"dashboard.card.background",
				[]string{"color.surface.raised", "color.surface.default"},
				"#ffffff",
			)
			if got.Token != test.wantToken || got.Value != test.wantValue {
				t.Fatalf("unexpected resolution: want %q=%q, got %q=%q", test.wantToken, test.wantValue, got.Token, got.Value)
			}
		})
	}
}

func TestThemeSelectionLegacyPayloadShapeRemainsCompatible(t *testing.T) {
	selection := &ThemeSelection{
		Name:    "admin",
		Variant: "dark",
		Tokens: map[string]string{
			"dashboard-accent": "#22d3ee",
			"--color-primary":  "#111827",
		},
		Assets: ThemeAssets{
			Values: map[string]string{"logo": "logo.svg"},
			Prefix: "/themes/admin",
		},
		Templates:  map[string]string{"widget": "widgets/custom.html"},
		ChartTheme: "wonderland",
	}
	payload := themePayload(selection)

	for _, key := range []string{
		"name", "variant", "tokens", "css_vars", "css_vars_inline",
		"asset_prefix", "assets", "templates", "chart_theme",
	} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("legacy theme payload lost %q: %#v", key, payload)
		}
	}
	if got := payload["css_vars_inline"]; got != "--color-primary: #111827;--dashboard-accent: #22d3ee;" {
		t.Fatalf("unexpected deterministic inline projection: %#v", got)
	}
	vars := requireTestValue[map[string]string](t, payload["css_vars"])
	if vars["--color-primary"] != "#111827" {
		t.Fatalf("pre-prefixed legacy token changed variable name: %#v", vars)
	}
}

func TestThemeSelectionLegacyDashboardStylesTrackOnlyEmittedTokens(t *testing.T) {
	selection := &ThemeSelection{Tokens: map[string]string{
		"--dashboard-accent": "#22d3ee",
		"color.focus.ring":   "#0ea5e9",
		"dashboard-muted":    "url(https://example.test/image)",
	}}
	styles := selection.legacyDashboardStyles()
	if !styles["accent"] {
		t.Fatalf("pre-prefixed legacy accent lost its consumer: %#v", styles)
	}
	if styles["muted"] || styles["surface"] || styles["foreground"] {
		t.Fatalf("invalid or absent legacy tokens activated declarations: %#v", styles)
	}
}

func TestConsumerDiagnosticsNeverConsumeInvalidSupportedTokens(t *testing.T) {
	selection := &ThemeSelection{Tokens: map[string]string{
		"dashboard.card.background": "url(https://example.test/image)",
	}}
	diagnostics := selection.SemanticProjection().ConsumerDiagnostics(
		"dashboard.card",
		"dashboard.card.background",
	)
	for _, diagnostic := range diagnostics {
		if diagnostic.Status == TokenConsumed || diagnostic.Status == TokenUnused {
			t.Fatalf("invalid token received consumption status: %+v", diagnostic)
		}
	}
}

func TestConsumerDiagnosticsKeepsIgnoredAliasUnused(t *testing.T) {
	selection := &ThemeSelection{Tokens: map[string]string{
		"surface":               "#f8fafc",
		"color.surface.default": "#ffffff",
	}}
	diagnostics := selection.SemanticProjection().ConsumerDiagnostics(
		"test.consumer",
		"color.surface.default",
	)
	statuses := diagnosticStatuses(diagnostics)
	if !slicesContain(statuses, "color.surface.default:consumed") {
		t.Fatalf("canonical token was not consumed: %#v", statuses)
	}
	if !slicesContain(statuses, "surface:unused") ||
		slicesContain(statuses, "surface:consumed") {
		t.Fatalf("ignored alias received the wrong consumption status: %#v", statuses)
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Token == "surface" &&
			diagnostic.Status == TokenUnused &&
			!strings.Contains(diagnostic.Reason, "canonical token takes precedence") {
			t.Fatalf("ignored alias lost its precedence reason: %+v", diagnostic)
		}
	}
}

func TestProjectThemeCSSVariablesUsesTokenOrderAndRejectsCollisions(t *testing.T) {
	t.Run("configured variables retain token order and prefix", func(t *testing.T) {
		profile := TokenProfile{
			Name: "dashboard-test",
			Tokens: map[string]TokenSpec{
				"a": {Constraint: ConstraintCSSValue, Variable: "--z"},
				"b": {Constraint: ConstraintCSSValue, Variable: "--a"},
			},
		}
		projection := ProjectThemeCSSVariables(
			map[string]string{"b": "second", "a": "first"},
			ProjectionOptions{Prefix: "--theme-", Profile: &profile},
		)
		if projection.Inline != "--theme-z:first;--theme-a:second;" {
			t.Fatalf("inline output is not in lexical token order: %q", projection.Inline)
		}
	})

	t.Run("colliding outputs are all omitted", func(t *testing.T) {
		projection := ProjectThemeCSSVariables(
			map[string]string{"a-b": "first", "a.b": "second"},
			ProjectionOptions{},
		)
		if len(projection.Variables) != 0 || projection.Inline != "" {
			t.Fatalf("colliding variables were emitted: %+v", projection)
		}
		for _, diagnostic := range projection.Diagnostics {
			if diagnostic.Status != TokenInvalid ||
				diagnostic.Reason != "CSS variable collision with tokens: a-b, a.b" {
				t.Fatalf("unexpected collision diagnostic: %+v", diagnostic)
			}
		}
	})
}

func TestThemeSelectionSemanticDashboardOptInAndConsumption(t *testing.T) {
	legacy := &ThemeSelection{Tokens: map[string]string{"dashboard-accent": "#2563eb"}}
	if legacy.SemanticDashboardEnabled() {
		t.Fatal("legacy token unexpectedly enabled semantic dashboard consumers")
	}
	if _, ok := themePayload(legacy)["semantic_enabled"]; ok {
		t.Fatal("legacy payload unexpectedly emitted semantic consumer metadata")
	}
	for name, tokens := range map[string]map[string]string{
		"unsupported dotted token": {"custom.brand": "#2563eb"},
		"chart-only token":         {"chart.series.1": "#2563eb"},
	} {
		t.Run(name, func(t *testing.T) {
			selection := &ThemeSelection{Tokens: tokens}
			if selection.SemanticDashboardEnabled() {
				t.Fatalf("non-chrome tokens enabled dashboard consumers: %#v", tokens)
			}
		})
	}

	selection := &ThemeSelection{Tokens: map[string]string{
		"dashboard.card.background": "#ffffff",
		"color.surface.raised":      "#f8fafc",
		"color.focus.ring":          "#0ea5e9",
	}}
	if !selection.SemanticDashboardEnabled() {
		t.Fatal("canonical dashboard token did not enable semantic consumers")
	}
	payload := themePayload(selection)
	if payload["semantic_enabled"] != true {
		t.Fatalf("semantic payload flag missing: %#v", payload)
	}
	diagnostics, ok := payload["semantic_diagnostics"].([]TokenDiagnostic)
	if !ok {
		t.Fatalf("semantic diagnostics missing from payload: %#v", payload)
	}
	statuses := diagnosticStatuses(diagnostics)
	if !slicesContain(statuses, "dashboard.card.background:unused") {
		t.Fatalf("inventory-free payload overclaimed package consumption: %#v", statuses)
	}
	if !slicesContain(statuses, "color.surface.raised:unused") {
		t.Fatalf("shadowed portable fallback was not reported unused: %#v", statuses)
	}
	if !slicesContain(statuses, "color.focus.ring:unused") {
		t.Fatalf("inventory-free payload overclaimed focus consumption: %#v", statuses)
	}
}

func TestSemanticDashboardPlanUsesRenderedSurfaceInventory(t *testing.T) {
	selection := &ThemeSelection{Tokens: map[string]string{
		"dashboard.card.background": "#ffffff",
		"dashboard.metric.label":    "#64748b",
		"dashboard.metric.value":    "#111827",
		"color.surface.default":     "#f8fafc",
		"color.action.accent":       "#2563eb",
	}}

	empty := selection.SemanticDashboardPlan(DashboardSemanticUsage{
		Dashboard:  true,
		Header:     true,
		Areas:      true,
		EmptyState: true,
	})
	if empty.Styles.CardBackground || empty.Styles.MetricValue {
		t.Fatalf("empty dashboard activated widget-only styles: %+v", empty.Styles)
	}
	if !empty.Styles.MetricLabel || !empty.Styles.EmptyState {
		t.Fatalf("empty dashboard did not activate its rendered state consumer: %+v", empty.Styles)
	}
	emptyStatuses := diagnosticStatuses(empty.Diagnostics)
	for _, token := range []string{
		"dashboard.card.background:unused",
		"dashboard.metric.label:consumed",
		"dashboard.metric.value:unused",
		"color.surface.default:unused",
		"color.action.accent:unused",
	} {
		if !slicesContain(emptyStatuses, token) {
			t.Fatalf("empty dashboard missing %q from %#v", token, emptyStatuses)
		}
	}

	shell := selection.SemanticDashboardPlan(DashboardSemanticUsage{Shell: true})
	if shell.Styles.Active() {
		t.Fatalf("shell inventory activated stock dashboard styles: %+v", shell.Styles)
	}
	shellStatuses := diagnosticStatuses(shell.Diagnostics)
	for _, token := range []string{
		"dashboard.card.background:consumed",
		"color.surface.default:consumed",
		"color.action.accent:consumed",
		"dashboard.metric.value:unused",
	} {
		if !slicesContain(shellStatuses, token) {
			t.Fatalf("shell inventory missing %q from %#v", token, shellStatuses)
		}
	}

	contradictory := selection.SemanticDashboardPlan(DashboardSemanticUsage{
		Shell:     true,
		Dashboard: true,
		Widgets:   true,
	})
	if contradictory.Styles.Active() {
		t.Fatalf("shell inventory did not take precedence over dashboard flags: %+v", contradictory.Styles)
	}
}

func TestThemePayloadForPageDerivesRenderedSurfaceInventory(t *testing.T) {
	selection := &ThemeSelection{Tokens: map[string]string{
		"dashboard.card.background": "#ffffff",
		"dashboard.metric.label":    "#64748b",
		"color.action.accent":       "#2563eb",
	}}

	emptyPayload := themePayloadForPage(selection, Page{
		Areas: []PageArea{{Slot: "main", Code: "admin.dashboard.main"}},
	})
	emptyStyles, ok := emptyPayload["semantic_styles"].(map[string]bool)
	if !ok {
		t.Fatalf("empty page semantic styles missing: %#v", emptyPayload)
	}
	if !emptyStyles["metric_label"] || !emptyStyles["empty_state"] ||
		emptyStyles["card_background"] {
		t.Fatalf("empty page derived the wrong semantic consumers: %#v", emptyStyles)
	}
	emptyDiagnostics, ok := emptyPayload["semantic_diagnostics"].([]TokenDiagnostic)
	if !ok {
		t.Fatalf("empty page semantic diagnostics missing: %#v", emptyPayload)
	}
	emptyStatuses := diagnosticStatuses(emptyDiagnostics)
	if !slicesContain(emptyStatuses, "dashboard.metric.label:consumed") ||
		!slicesContain(emptyStatuses, "dashboard.card.background:unused") {
		t.Fatalf("empty page diagnostics do not match rendered output: %#v", emptyStatuses)
	}

	shellPayload := themePayloadForPage(selection, Page{Shell: &Shell{}})
	if _, enabled := shellPayload["semantic_enabled"]; enabled {
		t.Fatalf("shell page activated the stock dashboard stylesheet: %#v", shellPayload)
	}
	shellDiagnostics, ok := shellPayload["semantic_diagnostics"].([]TokenDiagnostic)
	if !ok {
		t.Fatalf("shell page semantic diagnostics missing: %#v", shellPayload)
	}
	shellStatuses := diagnosticStatuses(shellDiagnostics)
	if !slicesContain(shellStatuses, "dashboard.card.background:consumed") ||
		!slicesContain(shellStatuses, "color.action.accent:consumed") ||
		!slicesContain(shellStatuses, "dashboard.metric.label:consumed") {
		t.Fatalf("shell page diagnostics do not match shell CSS: %#v", shellStatuses)
	}

	raw, err := json.Marshal(Page{
		Shell: &Shell{
			SurfaceID: "theme-test",
			Regions: []ShellRegion{{
				ID:        "main",
				Role:      ShellRegionRoleMain,
				Placement: ShellRegionPlacementMain,
			}},
		},
		Theme: selection,
	})
	if err != nil {
		t.Fatalf("marshal typed shell page: %v", err)
	}
	var typedPayload struct {
		Theme struct {
			SemanticEnabled     bool              `json:"semantic_enabled"`
			SemanticDiagnostics []TokenDiagnostic `json:"semantic_diagnostics"`
		} `json:"theme"`
	}
	if err := json.Unmarshal(raw, &typedPayload); err != nil {
		t.Fatalf("decode typed shell page: %v", err)
	}
	if typedPayload.Theme.SemanticEnabled {
		t.Fatalf("typed shell page activated stock dashboard styles: %s", raw)
	}
	typedStatuses := diagnosticStatuses(typedPayload.Theme.SemanticDiagnostics)
	if !slicesContain(typedStatuses, "color.action.accent:consumed") {
		t.Fatalf("typed shell page lost render-aware diagnostics: %#v", typedStatuses)
	}
}

type semanticThemeFixture struct {
	Version int                 `json:"version"`
	Cases   []semanticThemeCase `json:"cases"`
}

type semanticThemeCase struct {
	Name          string            `json:"name"`
	Tokens        map[string]string `json:"tokens"`
	VariantTokens map[string]string `json:"variant_tokens"`
	Variables     map[string]string `json:"variables"`
	Inline        string            `json:"inline"`
	Statuses      []string          `json:"statuses"`
}

func diagnosticStatuses(diagnostics []TokenDiagnostic) []string {
	out := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, diagnostic.Token+":"+string(diagnostic.Status))
	}
	return out
}

func slicesContain(values []string, expected string) bool {
	return slices.Contains(values, expected)
}
