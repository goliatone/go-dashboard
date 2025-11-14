package dashboard

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSalesChartProviderBuildsSeries(t *testing.T) {
	provider := NewSalesChartProvider(stubSalesRepo{}, NewEChartsProvider("line", WithChartCache(nil)))
	ctx := WidgetContext{
		Instance: WidgetInstance{
			ID:           "sales-1",
			DefinitionID: "admin.widget.sales_chart",
			Configuration: map[string]any{
				"period":            "30d",
				"metric":            "revenue",
				"dynamic":           true,
				"refresh_endpoint":  "/sales/revenue",
				"comparison_metric": "orders",
			},
		},
		Viewer: ViewerContext{UserID: "tester"},
	}

	data, err := provider.Fetch(context.Background(), ctx)
	require.NoError(t, err)
	assert.Equal(t, true, data["dynamic"])
	assert.Equal(t, "/sales/revenue", data["refresh_endpoint"])
	html := html(data)
	assert.Contains(t, html, "revenue")
	assert.Contains(t, html, "orders")
}

type stubSalesRepo struct{}

func (stubSalesRepo) FetchSalesSeries(_ context.Context, query SalesSeriesQuery) ([]SalesSeriesPoint, error) {
	points := make([]SalesSeriesPoint, 3)
	for i := 0; i < len(points); i++ {
		points[i] = SalesSeriesPoint{
			Timestamp: time.Date(2024, time.January, i+1, 0, 0, 0, 0, time.UTC),
			Value:     float64(1000 + (i * 100)),
		}
	}
	return points, nil
}
