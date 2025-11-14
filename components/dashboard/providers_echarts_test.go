package dashboard

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

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
	assert.Contains(t, html(data), "test chart")
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
	assert.Contains(t, html(data), "line test")
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
	assert.Contains(t, html(data), "pie test")
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
