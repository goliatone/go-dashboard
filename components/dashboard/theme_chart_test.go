package dashboard

import (
	"reflect"
	"testing"
)

func TestSemanticChartPaletteFullAndPartialFallbacks(t *testing.T) {
	t.Run("full palette", func(t *testing.T) {
		tokens := map[string]string{}
		want := []string{
			"#111111", "#222222", "#333333", "#444444",
			"#555555", "#666666", "#777777", "#888888",
		}
		for index, color := range want {
			tokens["chart.series."+string(rune('1'+index))] = color
		}
		palette := (&ThemeSelection{Tokens: tokens}).SemanticChartPalette()
		if !palette.SeriesActive {
			t.Fatal("full semantic series did not activate palette")
		}
		if got := palette.SeriesColors(); !reflect.DeepEqual(got, want) {
			t.Fatalf("full palette mismatch:\nwant: %#v\ngot:  %#v", want, got)
		}
	})

	t.Run("partial palette rotates prior semantic colors", func(t *testing.T) {
		selection := &ThemeSelection{Tokens: map[string]string{
			"chart.series.1": "#2563eb",
			"chart.series.3": "#f59e0b",
		}}
		want := []string{
			"#2563eb", "#2563eb", "#f59e0b", "#2563eb",
			"#f59e0b", "#2563eb", "#f59e0b", "#2563eb",
		}
		if got := selection.SemanticChartPalette().SeriesColors(); !reflect.DeepEqual(got, want) {
			t.Fatalf("partial palette mismatch:\nwant: %#v\ngot:  %#v", want, got)
		}
	})

	t.Run("invalid first value retains current positional default", func(t *testing.T) {
		selection := &ThemeSelection{Tokens: map[string]string{
			"chart.series.1": "url(https://example.test/color)",
			"chart.series.3": "#f59e0b",
		}}
		palette := selection.SemanticChartPalette()
		if palette.Series[0] != currentChartSeriesDefaults[0] ||
			palette.Series[1] != currentChartSeriesDefaults[1] ||
			palette.Series[2] != "#f59e0b" ||
			palette.Series[3] != "#f59e0b" {
			t.Fatalf("invalid/missing fallback contract mismatch: %#v", palette.Series)
		}
	})

	t.Run("no semantic series preserves named theme colors", func(t *testing.T) {
		palette := (&ThemeSelection{}).SemanticChartPalette()
		if palette.SeriesActive || palette.SeriesColors() != nil {
			t.Fatalf("empty theme unexpectedly overrides named chart colors: %+v", palette)
		}
	})
}

func TestSemanticChartPalettePresentationFallbacksAndDiagnostics(t *testing.T) {
	selection := &ThemeSelection{Tokens: map[string]string{
		"chart.axis":                      "#64748b",
		"dashboard.chart.grid":            "#cbd5e1",
		"chart.tooltip-surface":           "#ffffff",
		"dashboard.chart.tooltip-text":    "#0f172a",
		"dashboard.chart.tooltip-surface": "url(https://example.test/image)",
	}}
	palette := selection.SemanticChartPalette()

	if palette.Axis != "#64748b" || palette.Grid != "#cbd5e1" ||
		palette.TooltipSurface != "#ffffff" || palette.TooltipText != "#0f172a" {
		t.Fatalf("unexpected chart presentation fallback: %+v", palette)
	}
	statuses := diagnosticStatuses(palette.Diagnostics)
	for _, expected := range []string{
		"chart.axis:consumed",
		"dashboard.chart.grid:consumed",
		"chart.tooltip-surface:consumed",
		"dashboard.chart.tooltip-surface:invalid",
		"dashboard.chart.tooltip-text:consumed",
	} {
		if !slicesContain(statuses, expected) {
			t.Fatalf("missing chart diagnostic %q from %#v", expected, statuses)
		}
	}
}

func TestSemanticChartPalettePortableBaseFallbackIsRelevant(t *testing.T) {
	selection := &ThemeSelection{Tokens: map[string]string{
		"color.text.secondary": "#64748b",
	}}
	palette := selection.SemanticChartPalette()
	if palette.Axis != "#64748b" || !palette.Active() || !palette.Relevant {
		t.Fatalf("portable base axis fallback did not activate chart presentation: %+v", palette)
	}
	if !slicesContain(diagnosticStatuses(palette.Diagnostics), "color.text.secondary:consumed") {
		t.Fatalf("portable chart fallback was not reported consumed: %+v", palette.Diagnostics)
	}
}

func TestSemanticChartPaletteKeepsInvalidChartDiagnosticsWithoutActivatingVisuals(t *testing.T) {
	selection := &ThemeSelection{Tokens: map[string]string{
		"chart.series.1": "url(https://example.test/color)",
	}}
	palette := selection.SemanticChartPalette()
	if !palette.Relevant {
		t.Fatal("invalid chart token was not classified as chart-relevant")
	}
	if palette.Active() {
		t.Fatalf("invalid chart token activated visual overrides: %+v", palette)
	}
	if !slicesContain(diagnosticStatuses(palette.Diagnostics), "chart.series.1:invalid") {
		t.Fatalf("invalid chart diagnostic missing: %+v", palette.Diagnostics)
	}
}
