package dashboard

import (
	"context"
	"fmt"
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
	assert.Contains(t, html(data), "echarts")
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
	assert.Contains(t, html(data), "echarts")
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
	assert.Contains(t, html(data), "echarts")
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

	title := data["title"].(string)
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
	data, ok := chart.Metadata["data"].(WidgetData)
	require.True(t, ok, "chart metadata should include widget data")

	markup := html(data)
	assert.Contains(t, markup, `nonce="service-nonce"`)
	assert.Contains(t, data["title"], "Layout Chart")
	assert.Contains(t, data["chart_type"], "bar")
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
	val, _ := data["chart_html"].(string)
	return strings.ToLower(val)
}

type countingCache struct {
	calls int32
	value string
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
	for k, v := range cfg {
		out[k] = v
	}
	return out
}
