package analytics

import (
	"context"
	"sync"

	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

// MockData seeds deterministic analytics responses for tests or local demos.
type MockData struct {
	Funnel dashboard.FunnelReport
	Cohort dashboard.CohortReport
	Alerts dashboard.AlertTrendsReport
}

// MockClient implements Client using in-memory fixtures.
type MockClient struct {
	data MockData
	mu   sync.RWMutex
}

// NewMockClient builds a mock analytics client from the provided fixtures.
func NewMockClient(data MockData) *MockClient {
	return &MockClient{data: data}
}

// FetchFunnel returns the configured funnel report ignoring query filters.
func (c *MockClient) FetchFunnel(context.Context, dashboard.FunnelQuery) (dashboard.FunnelReport, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneFunnel(c.data.Funnel), nil
}

// FetchCohorts returns the configured cohort report ignoring query filters.
func (c *MockClient) FetchCohorts(context.Context, dashboard.CohortQuery) (dashboard.CohortReport, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneCohort(c.data.Cohort), nil
}

// FetchAlerts returns the configured alert trends ignoring query filters.
func (c *MockClient) FetchAlerts(context.Context, dashboard.AlertTrendQuery) (dashboard.AlertTrendsReport, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return cloneAlerts(c.data.Alerts), nil
}

func cloneFunnel(report dashboard.FunnelReport) dashboard.FunnelReport {
	out := dashboard.FunnelReport{
		Range:          report.Range,
		Segment:        report.Segment,
		ConversionRate: report.ConversionRate,
		Goal:           report.Goal,
		Steps:          make([]dashboard.FunnelStep, len(report.Steps)),
	}
	copy(out.Steps, report.Steps)
	return out
}

func cloneCohort(report dashboard.CohortReport) dashboard.CohortReport {
	out := dashboard.CohortReport{Interval: report.Interval, Metric: report.Metric, Rows: make([]dashboard.CohortRow, len(report.Rows))}
	for i, row := range report.Rows {
		out.Rows[i] = dashboard.CohortRow{
			Label:     row.Label,
			Size:      row.Size,
			Retention: append([]float64(nil), row.Retention...),
		}
	}
	return out
}

func cloneAlerts(report dashboard.AlertTrendsReport) dashboard.AlertTrendsReport {
	out := dashboard.AlertTrendsReport{Service: report.Service, Series: make([]dashboard.AlertSeries, len(report.Series)), Totals: map[string]int{}}
	for i, series := range report.Series {
		counts := make(map[string]int, len(series.Counts))
		for k, v := range series.Counts {
			counts[k] = v
		}
		out.Series[i] = dashboard.AlertSeries{Day: series.Day, Counts: counts}
	}
	for k, v := range report.Totals {
		out.Totals[k] = v
	}
	return out
}
