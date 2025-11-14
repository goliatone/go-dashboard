package dashboard

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SalesSeriesPoint represents a single time-series value.
type SalesSeriesPoint struct {
	Timestamp time.Time
	Value     float64
}

// SalesSeriesQuery describes the requested metric.
type SalesSeriesQuery struct {
	Period  string
	Metric  string
	Segment string
	Viewer  ViewerContext
}

// SalesSeriesRepository fetches time-series data for the sales chart provider.
type SalesSeriesRepository interface {
	FetchSalesSeries(ctx context.Context, query SalesSeriesQuery) ([]SalesSeriesPoint, error)
}

// SalesChartProvider composes dynamic sales data into an echarts widget.
type SalesChartProvider struct {
	repo     SalesSeriesRepository
	renderer *EChartsProvider
}

// NewSalesChartProvider builds a provider backed by the given repository.
func NewSalesChartProvider(repo SalesSeriesRepository, renderer *EChartsProvider) Provider {
	if renderer == nil {
		renderer = NewEChartsProvider("line")
	}
	return &SalesChartProvider{
		repo:     repo,
		renderer: renderer,
	}
}

// Fetch renders the sales chart widget.
func (p *SalesChartProvider) Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error) {
	if p.repo == nil {
		return nil, fmt.Errorf("sales chart provider: repository is required")
	}

	cfg := meta.Instance.Configuration
	if cfg == nil {
		cfg = map[string]any{}
	}

	period := strings.ToLower(stringValue(cfg["period"], "30d"))
	metric := strings.ToLower(stringValue(cfg["metric"], "revenue"))
	segment := stringValue(cfg["segment"], "all customers")
	comparison := strings.ToLower(stringValue(cfg["comparison_metric"], ""))

	points, err := p.repo.FetchSalesSeries(ctx, SalesSeriesQuery{
		Period:  period,
		Metric:  metric,
		Segment: segment,
		Viewer:  meta.Viewer,
	})
	if err != nil {
		return nil, fmt.Errorf("sales chart provider: %w", err)
	}

	seriesData := []map[string]any{{
		"name": titleize(metric),
		"data": seriesValues(points),
	}}
	xAxis := axisLabels(points)

	if comparison != "" && comparison != metric {
		altPoints, altErr := p.repo.FetchSalesSeries(ctx, SalesSeriesQuery{
			Period:  period,
			Metric:  comparison,
			Segment: segment,
			Viewer:  meta.Viewer,
		})
		if altErr != nil {
			return nil, fmt.Errorf("sales chart comparison: %w", altErr)
		}
		seriesData = append(seriesData, map[string]any{
			"name": titleize(comparison),
			"data": seriesValues(altPoints),
		})
		if len(altPoints) > len(points) {
			xAxis = axisLabels(altPoints)
		}
	}

	temp := meta
	temp.Instance = meta.Instance
	temp.Instance.Configuration = map[string]any{
		"title":            fmt.Sprintf("%s (%s)", titleize(metric), segment),
		"subtitle":         strings.ToUpper(period),
		"x_axis":           xAxis,
		"series":           seriesData,
		"dynamic":          boolValue(cfg["dynamic"]),
		"refresh_endpoint": cfg["refresh_endpoint"],
		"theme":            cfg["theme"],
		"footer_note":      cfg["footer_note"],
	}

	data, err := p.renderer.Fetch(ctx, temp)
	if err != nil {
		return nil, err
	}

	data["source"] = map[string]any{
		"metric":  metric,
		"period":  period,
		"segment": segment,
	}
	return data, nil
}

func seriesValues(points []SalesSeriesPoint) []float64 {
	values := make([]float64, len(points))
	for i, point := range points {
		values[i] = point.Value
	}
	return values
}

func axisLabels(points []SalesSeriesPoint) []string {
	labels := make([]string, len(points))
	for i, point := range points {
		labels[i] = point.Timestamp.Format("Jan 2")
	}
	return labels
}

func titleize(value string) string {
	if value == "" {
		return value
	}
	lower := strings.ToLower(value)
	return strings.ToUpper(string(lower[0])) + lower[1:]
}

// NewStaticSalesRepository returns a repository that always serves the provided points.
func NewStaticSalesRepository(points []SalesSeriesPoint) SalesSeriesRepository {
	return staticSalesRepository{points: points}
}

type staticSalesRepository struct {
	points []SalesSeriesPoint
}

func (s staticSalesRepository) FetchSalesSeries(_ context.Context, _ SalesSeriesQuery) ([]SalesSeriesPoint, error) {
	out := make([]SalesSeriesPoint, len(s.points))
	copy(out, s.points)
	return out, nil
}
