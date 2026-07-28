package dashboard

import (
	"strconv"
	"strings"
)

const semanticChartSeriesCount = 8

var currentChartSeriesDefaults = [semanticChartSeriesCount]string{
	"#5470c6",
	"#91cc75",
	"#fac858",
	"#ee6666",
	"#73c0de",
	"#3ba272",
	"#fc8452",
	"#9a60b4",
}

// SemanticChartPalette is the typed dashboard chart presentation resolved from
// validated semantic tokens. SeriesActive distinguishes a semantic series
// override from the untouched named/custom ECharts theme palette.
type SemanticChartPalette struct {
	Series         [semanticChartSeriesCount]string `json:"series"`
	SeriesActive   bool                             `json:"series_active"`
	Axis           string                           `json:"axis,omitempty"`
	Grid           string                           `json:"grid,omitempty"`
	TooltipSurface string                           `json:"tooltip_surface,omitempty"`
	TooltipText    string                           `json:"tooltip_text,omitempty"`
	Diagnostics    []TokenDiagnostic                `json:"diagnostics,omitempty"`
	Relevant       bool                             `json:"-"`
}

// SemanticChartPalette resolves the frozen chart fallback contract. A missing
// series position rotates through previously supplied valid semantic colors;
// before the first semantic color it retains that position's current default.
func (theme *ThemeSelection) SemanticChartPalette() SemanticChartPalette {
	palette := SemanticChartPalette{Series: currentChartSeriesDefaults}
	if theme == nil {
		return palette
	}
	for token := range theme.Tokens {
		if strings.HasPrefix(token, "chart.") || strings.HasPrefix(token, "dashboard.chart.") {
			palette.Relevant = true
			break
		}
	}

	consumed := make([]string, 0, semanticChartSeriesCount+4)
	seenConsumed := map[string]bool{}
	semanticColors := make([]string, 0, semanticChartSeriesCount)
	rotation := 0
	for index := range semanticChartSeriesCount {
		token := "chart.series." + strconv.Itoa(index+1)
		resolved := theme.ResolveSemanticToken("", []string{token}, "")
		if resolved.Token != "" {
			palette.Series[index] = resolved.Value
			palette.SeriesActive = true
			semanticColors = append(semanticColors, resolved.Value)
			rotation = 0
			appendConsumedToken(&consumed, seenConsumed, resolved.Token)
			continue
		}
		if len(semanticColors) > 0 {
			palette.Series[index] = semanticColors[rotation%len(semanticColors)]
			rotation++
		}
	}

	palette.Axis = resolveChartPresentationToken(
		theme,
		"dashboard.chart.axis",
		[]string{"chart.axis", "color.text.secondary"},
		&consumed,
		seenConsumed,
	)
	palette.Grid = resolveChartPresentationToken(
		theme,
		"dashboard.chart.grid",
		[]string{"chart.grid", "color.border.default"},
		&consumed,
		seenConsumed,
	)
	palette.TooltipSurface = resolveChartPresentationToken(
		theme,
		"dashboard.chart.tooltip-surface",
		[]string{"chart.tooltip-surface", "color.surface.raised"},
		&consumed,
		seenConsumed,
	)
	palette.TooltipText = resolveChartPresentationToken(
		theme,
		"dashboard.chart.tooltip-text",
		[]string{"chart.tooltip-text", "color.text.primary"},
		&consumed,
		seenConsumed,
	)
	palette.Relevant = palette.Relevant || palette.Active()
	palette.Diagnostics = theme.SemanticProjection().ConsumerDiagnostics(
		"go-dashboard.echarts",
		consumed...,
	)
	return palette
}

// Active reports whether semantic chart presentation overrides are present.
func (palette SemanticChartPalette) Active() bool {
	return palette.SeriesActive ||
		palette.Axis != "" ||
		palette.Grid != "" ||
		palette.TooltipSurface != "" ||
		palette.TooltipText != ""
}

// SeriesColors returns a copy only when semantic series colors are active.
func (palette SemanticChartPalette) SeriesColors() []string {
	if !palette.SeriesActive {
		return nil
	}
	return append([]string(nil), palette.Series[:]...)
}

func (palette SemanticChartPalette) cacheKey() string {
	parts := make([]string, 0, semanticChartSeriesCount+5)
	if palette.SeriesActive {
		parts = append(parts, palette.Series[:]...)
	}
	parts = append(parts, palette.Axis, palette.Grid, palette.TooltipSurface, palette.TooltipText)
	return strings.Join(parts, "|")
}

func resolveChartPresentationToken(
	theme *ThemeSelection,
	component string,
	portable []string,
	consumed *[]string,
	seen map[string]bool,
) string {
	resolved := theme.ResolveSemanticToken(component, portable, "")
	if resolved.Token == "" {
		return ""
	}
	appendConsumedToken(consumed, seen, resolved.Token)
	return resolved.Value
}

func appendConsumedToken(tokens *[]string, seen map[string]bool, token string) {
	if token == "" || seen[token] {
		return
	}
	seen[token] = true
	*tokens = append(*tokens, token)
}
