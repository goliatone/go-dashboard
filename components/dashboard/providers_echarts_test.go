package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-echarts/go-echarts/v2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEChartsBarProvider(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("bar")
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title":  "Test Chart",
		"x_axis": []string{"A", "B", "C"},
		"series": []map[string]any{
			{"name": "Series 1", "data": []float64{10, 20, 30}},
		},
	})

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)

	assert.Equal(t, "bar", data["chart_type"])
	assert.Equal(t, "Test Chart", data["title"])
	assert.Contains(t, html(data), "echarts.init")
	assert.NotContains(t, html(data), "<!doctype html>")
	options, ok := data["chart_options"].(map[string]any)
	require.True(t, ok, "chart_options should expose safe structured ECharts configuration")
	assert.NotEmpty(t, options["series"])
	assert.Equal(t, DefaultEChartsAssetsHost(), data["chart_assets_host"])
	assert.Equal(t, []string{
		DefaultEChartsAssetsPath + "echarts.min.js",
		DefaultEChartsAssetsPath + "themes/westeros.js",
	}, jsAssets(data))
}

func TestEChartsLineProvider(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("line")
	ctx := sampleChartContext("admin.widget.line_chart", map[string]any{
		"title":  "Line Test",
		"x_axis": []string{"Day 1", "Day 2", "Day 3"},
		"series": []map[string]any{
			{"name": "Metric", "data": []float64{100, 150, 120}},
		},
	})

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	assert.Equal(t, "line", data["chart_type"])
	assert.Equal(t, "Line Test", data["title"])
	assert.Contains(t, html(data), "echarts.init")
	assert.NotContains(t, html(data), "<html")
}

func TestEChartsPieProvider(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("pie")
	ctx := sampleChartContext("admin.widget.pie_chart", map[string]any{
		"title": "Pie Test",
		"series": []map[string]any{
			{
				"name": "Categories",
				"data": []map[string]any{
					{"name": "Cat A", "value": 100},
					{"name": "Cat B", "value": 200},
				},
			},
		},
	})

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	assert.Equal(t, "pie", data["chart_type"])
	assert.Equal(t, "Pie Test", data["title"])
	assert.Contains(t, html(data), "echarts.init")
	assert.NotContains(t, html(data), "<head>")
}

func TestEChartsProviderInvalidType(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("bubble")
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title": "Unsupported",
		"series": []map[string]any{
			{"name": "Series", "data": []float64{1}},
		},
	})

	_, err := provider.Fetch(context.Background(), ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestEChartsProviderUsesCache(t *testing.T) {
	t.Parallel()
	cache := &countingCache{}
	provider := NewEChartsProvider("bar", WithChartCache(cache))
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title":  "Cached",
		"series": []map[string]any{{"name": "Series", "data": []float64{1, 2}}},
	})

	_, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	_, err = provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)

	assert.Equal(t, int32(1), cache.calls)
}

func TestEChartsProviderThemeOverride(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("bar", WithChartThemeResolver(func(viewer ViewerContext) string {
		return string(types.ThemeWalden)
	}))
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title": "Theme Override",
		"series": []map[string]any{
			{"name": "Series", "data": []float64{5, 6}},
		},
		"theme": "wonderland",
	})

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	assert.Equal(t, "wonderland", data["theme"])
}

func TestEChartsProviderThemeSelectionDefault(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("bar")
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title":  "Theme From Selection",
		"x_axis": []string{"A", "B"},
		"series": []map[string]any{
			{"name": "Series", "data": []float64{1, 2}},
		},
	})
	ctx.Theme = &ThemeSelection{Variant: "dark"}

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)

	assert.Equal(t, string(types.ThemeWonderland), data["theme"])
	require.NotNil(t, ctx.Theme)
	assert.Equal(t, string(types.ThemeWonderland), ctx.Theme.ChartTheme)
}

func TestEChartsProviderCustomThemeBeatsSelection(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("bar", WithChartTheme(string(types.ThemeWalden)))
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title":  "Explicit Theme",
		"x_axis": []string{"A"},
		"series": []map[string]any{{"name": "Series", "data": []float64{1}}},
	})
	ctx.Theme = &ThemeSelection{Variant: "dark"}

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	assert.Equal(t, string(types.ThemeWalden), data["theme"])
}

func TestEChartsProviderAppliesSemanticPaletteWithoutReplacingNamedTheme(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("bar", WithChartTheme(string(types.ThemeWalden)))
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title":  "Semantic Palette",
		"x_axis": []string{"A", "B"},
		"series": []map[string]any{
			{"name": "First", "data": []float64{1, 2}},
			{"name": "Second", "data": []float64{2, 3}},
		},
	})
	ctx.Theme = &ThemeSelection{Tokens: map[string]string{
		"chart.series.1":                  "#2563eb",
		"chart.series.2":                  "#f59e0b",
		"dashboard.chart.axis":            "#64748b",
		"dashboard.chart.grid":            "#cbd5e1",
		"dashboard.chart.tooltip-surface": "#ffffff",
		"dashboard.chart.tooltip-text":    "#0f172a",
	}}

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	assert.Equal(t, string(types.ThemeWalden), data["theme"])
	assert.Equal(t, []string{
		"#2563eb", "#f59e0b", "#2563eb", "#f59e0b",
		"#2563eb", "#f59e0b", "#2563eb", "#f59e0b",
	}, stringSliceValue(data["semantic_palette"]))

	markup := html(data)
	for _, want := range []string{
		`"itemstyle":{"color":"#2563eb"}`,
		`"itemstyle":{"color":"#f59e0b"}`,
		`"color":"#64748b"`,
		`"color":"#cbd5e1"`,
		`"backgroundcolor":"#ffffff"`,
		`"textstyle":{"color":"#0f172a"}`,
	} {
		assert.Contains(t, markup, want)
	}
	options, ok := data["chart_options"].(map[string]any)
	require.True(t, ok, "semantic chart options should be structured")
	encodedOptions, err := json.Marshal(options)
	require.NoError(t, err)
	for _, color := range []string{"#2563eb", "#f59e0b", "#64748b", "#cbd5e1", "#ffffff", "#0f172a"} {
		assert.Contains(t, string(encodedOptions), color)
	}
}

func TestEChartsProviderCacheSeparatesResolvedThemeAndPalette(t *testing.T) {
	t.Parallel()
	cache := newKeyedCountingCache()
	provider := NewEChartsProvider("bar", WithChartCache(cache))
	config := map[string]any{
		"title":  "Variant Cache",
		"x_axis": []string{"A"},
		"series": []map[string]any{{"name": "Series", "data": []float64{1}}},
	}

	dark := sampleChartContext("admin.widget.bar_chart", config)
	dark.Theme = &ThemeSelection{
		Variant: "dark",
		Tokens:  map[string]string{"chart.series.1": "#111827"},
	}
	light := sampleChartContext("admin.widget.bar_chart", config)
	light.Theme = &ThemeSelection{
		Variant: "light",
		Tokens:  map[string]string{"chart.series.1": "#f8fafc"},
	}

	darkData, err := provider.Fetch(context.Background(), dark)
	require.NoError(t, err)
	lightData, err := provider.Fetch(context.Background(), light)
	require.NoError(t, err)

	assert.Equal(t, int32(2), cache.calls.Load())
	assert.Contains(t, html(darkData), "#111827")
	assert.NotContains(t, html(darkData), "#f8fafc")
	assert.Contains(t, html(lightData), "#f8fafc")
	assert.NotContains(t, html(lightData), "#111827")
	assert.Equal(t, string(types.ThemeWonderland), darkData["theme"])
	assert.Equal(t, string(types.ThemeWesteros), lightData["theme"])
}

func TestEChartsProviderAppliesSemanticPaletteToPieData(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("pie")
	ctx := sampleChartContext("admin.widget.pie_chart", map[string]any{
		"title": "Semantic Pie",
		"series": []map[string]any{{
			"name": "Categories",
			"data": []map[string]any{
				{"name": "A", "value": 1},
				{"name": "B", "value": 2},
			},
		}},
	})
	ctx.Theme = &ThemeSelection{Tokens: map[string]string{
		"chart.series.1": "#2563eb",
		"chart.series.2": "#f59e0b",
	}}

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	markup := html(data)
	assert.Contains(t, markup, `"itemstyle":{"color":"#2563eb"}`)
	assert.Contains(t, markup, `"itemstyle":{"color":"#f59e0b"}`)
}

func TestEChartsProviderReportsOnlyPresentationAppliedByChartType(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("pie")
	ctx := sampleChartContext("admin.widget.pie_chart", map[string]any{
		"title": "Semantic Pie Diagnostics",
		"series": []map[string]any{{
			"name": "Categories",
			"data": []map[string]any{{"name": "A", "value": 1}},
		}},
	})
	ctx.Theme = &ThemeSelection{Tokens: map[string]string{
		"chart.series.1":        "#2563eb",
		"chart.axis":            "#64748b",
		"chart.grid":            "#cbd5e1",
		"chart.tooltip-surface": "#ffffff",
	}}

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	statuses := serializedThemeDiagnosticStatuses(data["theme_diagnostics"])
	assert.Contains(t, statuses, "chart.series.1:consumed")
	assert.Contains(t, statuses, "chart.tooltip-surface:consumed")
	assert.Contains(t, statuses, "chart.axis:unused")
	assert.Contains(t, statuses, "chart.grid:unused")
}

func TestEChartsProviderExposesInvalidChartDiagnosticsWithoutChangingPalette(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("bar")
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title":  "Invalid Semantic Palette",
		"x_axis": []string{"A"},
		"series": []map[string]any{{"name": "Series", "data": []float64{1}}},
	})
	ctx.Theme = &ThemeSelection{Tokens: map[string]string{
		"chart.series.1": "url(https://example.test/color)",
	}}

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	if values := stringSliceValue(data["semantic_palette"]); len(values) != 0 {
		t.Fatalf("invalid chart token changed the palette: %#v", values)
	}
	assert.Contains(t, serializedThemeDiagnosticStatuses(data["theme_diagnostics"]), "chart.series.1:invalid")
	assert.NotContains(t, html(data), "example.test")
}

func TestEChartsProviderSanitizesStrings(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("bar")
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title":  `<script>alert("xss")</script>`,
		"x_axis": []string{`<img src=x onerror=alert(1)>`},
		"series": []map[string]any{
			{"name": `<b onclick="hack">Series</b>`, "data": []float64{1, 2}},
		},
	})

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)

	title := requireTestValue[string](t, data["title"])
	assert.NotContains(t, title, "<script>")
	assert.Contains(t, title, "&lt;script&gt;")

	markup := html(data)
	assert.NotContains(t, markup, "<b onclick")
	assert.NotContains(t, markup, "<img src")
	assert.NotContains(t, markup, "<script>alert(\"xss\")</script>")
	assert.Contains(t, markup, "&amp;lt;img src=x onerror=alert(1)&amp;gt;")
}

func TestEChartsProviderAppliesNoncePerRequest(t *testing.T) {
	t.Parallel()
	provider := NewEChartsProvider("bar")
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title":  "Nonce Test",
		"x_axis": []string{"One"},
		"series": []map[string]any{
			{"name": "S1", "data": []float64{1}},
		},
	})
	ctx.Options = map[string]any{scriptNonceOptionKey: "nonce-a"}

	data1, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	assert.Contains(t, html(data1), `nonce="nonce-a"`)

	ctx.Options[scriptNonceOptionKey] = "nonce-b"
	data2, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	assert.Contains(t, html(data2), `nonce="nonce-b"`)
}

func TestServiceIntegratesEChartsProvider(t *testing.T) {
	store := newMemoryWidgetStoreForCharts()
	registry := NewRegistry()
	service := NewService(Options{
		WidgetStore:     store,
		Providers:       registry,
		ConfigValidator: noopConfigValidator{},
		ScriptNonce: func(context.Context) string {
			return "service-nonce"
		},
	})
	err := service.AddWidget(context.Background(), AddWidgetRequest{
		DefinitionID: "admin.widget.bar_chart",
		AreaCode:     "admin.dashboard.main",
		Configuration: map[string]any{
			"title":  "Layout Chart",
			"x_axis": []string{"Mon", "Tue"},
			"series": []map[string]any{
				{"name": "Series", "data": []float64{1, 2}},
			},
		},
	})
	require.NoError(t, err)

	layout, err := service.ConfigureLayout(context.Background(), ViewerContext{UserID: "integration"})
	require.NoError(t, err)

	mainArea := layout.Areas["admin.dashboard.main"]
	require.NotEmpty(t, mainArea)

	var chart WidgetInstance
	for _, widget := range mainArea {
		if widget.DefinitionID == "admin.widget.bar_chart" {
			chart = widget
			break
		}
	}
	require.NotNil(t, chart.Metadata)
	view, ok := chart.Metadata[widgetViewModelMetadataKey].(WidgetViewModel)
	require.True(t, ok, "chart metadata should include widget view model")
	payload, err := view.Serialize()
	require.NoError(t, err)
	data, err := serializedWidgetData(payload)
	require.NoError(t, err)

	markup := html(data)
	assert.Contains(t, markup, `nonce="service-nonce"`)
	assert.Contains(t, data["title"], "Layout Chart")
	assert.Contains(t, data["chart_type"], "bar")
	assert.Equal(t, []string{
		DefaultEChartsAssetsPath + "echarts.min.js",
		DefaultEChartsAssetsPath + "themes/westeros.js",
	}, jsAssets(data))
}

func sampleChartContext(definition string, cfg map[string]any) WidgetContext {
	return WidgetContext{
		Instance: WidgetInstance{
			ID:            definition + "-instance",
			DefinitionID:  definition,
			Configuration: cfg,
		},
		Viewer: ViewerContext{UserID: "tester", Locale: "en"},
	}
}

func html(data WidgetData) string {
	val, valid := data["chart_html"].(string)
	if !valid {
		return ""
	}
	return strings.ToLower(val)
}

func jsAssets(data WidgetData) []string {
	raw, valid := data["js_assets"].([]string)
	if valid {
		return raw
	}
	values, valid := data["js_assets"].([]any)
	if !valid {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if s, ok := value.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func serializedThemeDiagnosticStatuses(value any) []string {
	if diagnostics, valid := value.([]TokenDiagnostic); valid {
		return diagnosticStatuses(diagnostics)
	}
	items := diagnosticMaps(value)
	out := make([]string, 0, len(items))
	for _, item := range items {
		token, tokenValid := item["token"].(string)
		status, statusValid := diagnosticStatusString(item["status"])
		if !tokenValid || !statusValid {
			continue
		}
		if token != "" && status != "" {
			out = append(out, token+":"+status)
		}
	}
	return out
}

func diagnosticStatusString(value any) (string, bool) {
	switch status := value.(type) {
	case string:
		return status, true
	case TokenStatus:
		return string(status), true
	default:
		return "", false
	}
}

func diagnosticMaps(value any) []map[string]any {
	if items, valid := value.([]map[string]any); valid {
		return items
	}
	values, valid := value.([]any)
	if !valid {
		return nil
	}
	items := make([]map[string]any, 0, len(values))
	for _, value := range values {
		if item, itemValid := value.(map[string]any); itemValid {
			items = append(items, item)
		}
	}
	return items
}

type countingCache struct {
	calls int32
	value string
}

type keyedCountingCache struct {
	values map[string]string
	calls  atomic.Int32
}

func newKeyedCountingCache() *keyedCountingCache {
	return &keyedCountingCache{values: map[string]string{}}
}

func (cache *keyedCountingCache) GetOrRender(key string, render func() (string, error)) (string, error) {
	if value, ok := cache.values[key]; ok {
		return value, nil
	}
	value, err := render()
	if err != nil {
		return "", err
	}
	cache.values[key] = value
	cache.calls.Add(1)
	return value, nil
}

func (c *countingCache) GetOrRender(_ string, render func() (string, error)) (string, error) {
	if c.value != "" {
		return c.value, nil
	}
	html, err := render()
	if err != nil {
		return "", err
	}
	atomic.AddInt32(&c.calls, 1)
	c.value = html
	return html, nil
}

func BenchmarkEChartsBarChart(b *testing.B) {
	provider := NewEChartsProvider("bar")
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title":  "Benchmark",
		"x_axis": []string{"A", "B", "C", "D", "E"},
		"series": []map[string]any{
			{"name": "S1", "data": []float64{10, 20, 30, 40, 50}},
			{"name": "S2", "data": []float64{11, 21, 31, 41, 51}},
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := provider.Fetch(context.Background(), ctx); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEChartsBarChartCached(b *testing.B) {
	cache := NewChartCache(5 * time.Minute)
	provider := NewEChartsProvider("bar", WithChartCache(cache))
	ctx := sampleChartContext("admin.widget.bar_chart", map[string]any{
		"title":  "Cached Benchmark",
		"x_axis": []string{"A", "B", "C", "D", "E"},
		"series": []map[string]any{
			{"name": "S1", "data": []float64{10, 20, 30, 40, 50}},
			{"name": "S2", "data": []float64{11, 21, 31, 41, 51}},
		},
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := provider.Fetch(context.Background(), ctx); err != nil {
			b.Fatal(err)
		}
	}
}

type memoryWidgetStoreForCharts struct {
	instances   map[string]WidgetInstance
	assignments map[string][]string
	next        int
}

func newMemoryWidgetStoreForCharts() *memoryWidgetStoreForCharts {
	return &memoryWidgetStoreForCharts{
		instances:   map[string]WidgetInstance{},
		assignments: map[string][]string{},
	}
}

func (m *memoryWidgetStoreForCharts) EnsureArea(context.Context, WidgetAreaDefinition) (bool, error) {
	return false, nil
}

func (m *memoryWidgetStoreForCharts) EnsureDefinition(context.Context, WidgetDefinition) (bool, error) {
	return false, nil
}

func (m *memoryWidgetStoreForCharts) CreateInstance(_ context.Context, input CreateWidgetInstanceInput) (WidgetInstance, error) {
	m.next++
	id := fmt.Sprintf("chart-%d", m.next)
	inst := WidgetInstance{
		ID:            id,
		DefinitionID:  input.DefinitionID,
		Configuration: cloneConfig(input.Configuration),
		Metadata:      map[string]any{},
	}
	m.instances[id] = inst
	return inst, nil
}

func (m *memoryWidgetStoreForCharts) GetInstance(_ context.Context, id string) (WidgetInstance, error) {
	inst, ok := m.instances[id]
	if !ok {
		return WidgetInstance{}, fmt.Errorf("instance %s not found", id)
	}
	return inst, nil
}

func (m *memoryWidgetStoreForCharts) DeleteInstance(context.Context, string) error {
	return nil
}

func (m *memoryWidgetStoreForCharts) AssignInstance(_ context.Context, input AssignWidgetInput) error {
	if _, ok := m.instances[input.InstanceID]; !ok {
		return fmt.Errorf("instance %s not found", input.InstanceID)
	}
	inst := m.instances[input.InstanceID]
	inst.AreaCode = input.AreaCode
	m.instances[input.InstanceID] = inst
	m.assignments[input.AreaCode] = append(m.assignments[input.AreaCode], input.InstanceID)
	return nil
}

func (m *memoryWidgetStoreForCharts) ReorderArea(context.Context, ReorderAreaInput) error {
	return nil
}

func (m *memoryWidgetStoreForCharts) ResolveArea(_ context.Context, input ResolveAreaInput) (ResolvedArea, error) {
	ids := m.assignments[input.AreaCode]
	widgets := make([]WidgetInstance, 0, len(ids))
	for _, id := range ids {
		if inst, ok := m.instances[id]; ok {
			widgets = append(widgets, inst)
		}
	}
	return ResolvedArea{AreaCode: input.AreaCode, Widgets: widgets}, nil
}

func (m *memoryWidgetStoreForCharts) UpdateInstance(_ context.Context, input UpdateWidgetInstanceInput) (WidgetInstance, error) {
	inst, ok := m.instances[input.InstanceID]
	if !ok {
		return WidgetInstance{}, fmt.Errorf("instance %s not found", input.InstanceID)
	}
	if input.Configuration != nil {
		inst.Configuration = cloneConfig(input.Configuration)
	}
	if input.Metadata != nil {
		inst.Metadata = input.Metadata
	}
	m.instances[input.InstanceID] = inst
	return inst, nil
}

func cloneConfig(cfg map[string]any) map[string]any {
	if cfg == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(cfg))
	maps.Copy(out, cfg)
	return out
}
