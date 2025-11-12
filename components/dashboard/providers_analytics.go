package dashboard

import (
	"context"
	"time"
)

// FunnelReportRepository loads funnel metrics from analytics/reporting systems.
type FunnelReportRepository interface {
	FetchFunnelReport(ctx context.Context, query FunnelQuery) (FunnelReport, error)
}

// CohortReportRepository loads cohort retention data.
type CohortReportRepository interface {
	FetchCohortReport(ctx context.Context, query CohortQuery) (CohortReport, error)
}

// AlertTrendsRepository loads alert volume grouped by severity.
type AlertTrendsRepository interface {
	FetchAlertTrends(ctx context.Context, query AlertTrendQuery) (AlertTrendsReport, error)
}

// FunnelQuery describes funnel configuration.
type FunnelQuery struct {
	Range   string
	Segment string
	Goal    float64
}

// FunnelReport captures drop-off per stage.
type FunnelReport struct {
	Range          string
	Segment        string
	ConversionRate float64
	Steps          []FunnelStep
	Goal           float64
}

// FunnelStep is a single funnel stage.
type FunnelStep struct {
	Label    string
	Value    float64
	DropOff  float64
	Position int
}

// CohortQuery controls retention cohort rendering.
type CohortQuery struct {
	Interval string
	Periods  int
	Metric   string
}

// CohortReport contains rows (cohort) and their retention.
type CohortReport struct {
	Interval string
	Metric   string
	Rows     []CohortRow
}

// CohortRow describes a single cohort.
type CohortRow struct {
	Label     string
	Size      int
	Retention []float64
}

// AlertTrendQuery configures alert chart queries.
type AlertTrendQuery struct {
	LookbackDays int
	Severities   []string
	Service      string
}

// AlertTrendsReport carries severity counts per day.
type AlertTrendsReport struct {
	Service string
	Series  []AlertSeries
	Totals  map[string]int
}

// AlertSeries is a single day/time bucket.
type AlertSeries struct {
	Day    time.Time
	Counts map[string]int
}

type funnelProvider struct {
	repo FunnelReportRepository
}

// NewFunnelAnalyticsProvider wires a FunnelReportRepository into a Provider.
func NewFunnelAnalyticsProvider(repo FunnelReportRepository) Provider {
	if repo == nil {
		repo = DemoFunnelRepository{}
	}
	return &funnelProvider{repo: repo}
}

func (p *funnelProvider) Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error) {
	cfg := extractFunnelQuery(meta.Instance.Configuration)
	report, err := p.repo.FetchFunnelReport(ctx, cfg)
	if err != nil {
		return nil, err
	}
	steps := make([]map[string]any, 0, len(report.Steps))
	for _, step := range report.Steps {
		steps = append(steps, map[string]any{
			"label":    step.Label,
			"value":    step.Value,
			"dropoff":  step.DropOff,
			"position": step.Position,
		})
	}
	return WidgetData{
		"range":           report.Range,
		"segment":         report.Segment,
		"conversion_rate": report.ConversionRate,
		"steps":           steps,
		"goal":            report.Goal,
	}, nil
}

type cohortProvider struct {
	repo CohortReportRepository
}

// NewCohortAnalyticsProvider wires cohort repositories for retention widgets.
func NewCohortAnalyticsProvider(repo CohortReportRepository) Provider {
	if repo == nil {
		repo = DemoCohortRepository{}
	}
	return &cohortProvider{repo: repo}
}

func (p *cohortProvider) Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error) {
	query := extractCohortQuery(meta.Instance.Configuration)
	report, err := p.repo.FetchCohortReport(ctx, query)
	if err != nil {
		return nil, err
	}
	rows := make([]map[string]any, 0, len(report.Rows))
	for _, row := range report.Rows {
		rows = append(rows, map[string]any{
			"label":     row.Label,
			"size":      row.Size,
			"retention": row.Retention,
		})
	}
	return WidgetData{
		"interval": report.Interval,
		"metric":   report.Metric,
		"rows":     rows,
	}, nil
}

type alertProvider struct {
	repo AlertTrendsRepository
}

// NewAlertTrendsProvider wires alert repositories into a Provider instance.
func NewAlertTrendsProvider(repo AlertTrendsRepository) Provider {
	if repo == nil {
		repo = DemoAlertRepository{}
	}
	return &alertProvider{repo: repo}
}

func (p *alertProvider) Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error) {
	query := extractAlertQuery(meta.Instance.Configuration)
	report, err := p.repo.FetchAlertTrends(ctx, query)
	if err != nil {
		return nil, err
	}
	series := make([]map[string]any, 0, len(report.Series))
	for _, bucket := range report.Series {
		series = append(series, map[string]any{
			"day":    bucket.Day.Format("2006-01-02"),
			"counts": bucket.Counts,
		})
	}
	return WidgetData{
		"lookback_days": query.LookbackDays,
		"severities":    query.Severities,
		"service":       report.Service,
		"series":        series,
		"totals":        report.Totals,
	}, nil
}

func extractFunnelQuery(config map[string]any) FunnelQuery {
	rangeVal := stringOr(config["range"], "30d")
	return FunnelQuery{
		Range:   rangeVal,
		Segment: stringOr(config["segment"], "all users"),
		Goal:    floatOr(config["goal"], 45),
	}
}

func extractCohortQuery(config map[string]any) CohortQuery {
	return CohortQuery{
		Interval: stringOr(config["interval"], "weekly"),
		Periods:  intOr(config["periods"], 8),
		Metric:   stringOr(config["metric"], "retained"),
	}
}

func extractAlertQuery(config map[string]any) AlertTrendQuery {
	severities := sliceOr(config["severity"], []string{"warning", "critical"})
	return AlertTrendQuery{
		LookbackDays: intOr(config["lookback_days"], 30),
		Severities:   severities,
		Service:      stringOr(config["service"], "All Services"),
	}
}

func stringOr(value any, fallback string) string {
	if v, ok := value.(string); ok && v != "" {
		return v
	}
	return fallback
}

func floatOr(value any, fallback float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return fallback
}

func intOr(value any, fallback int) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}
	return fallback
}

func sliceOr(value any, fallback []string) []string {
	if value == nil {
		return fallback
	}
	result := []string{}
	switch typed := value.(type) {
	case []string:
		result = append(result, typed...)
	case []any:
		for _, item := range typed {
			if s, ok := item.(string); ok && s != "" {
				result = append(result, s)
			}
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}

// DemoFunnelRepository returns static funnel data for demos/tests.
type DemoFunnelRepository struct{}

func (DemoFunnelRepository) FetchFunnelReport(ctx context.Context, query FunnelQuery) (FunnelReport, error) {
	steps := []FunnelStep{
		{Label: "Visitors", Value: 18500, Position: 0},
		{Label: "Signed Up", Value: 7200, Position: 1},
		{Label: "Activated", Value: 3100, Position: 2},
		{Label: "Paying", Value: 980, Position: 3},
	}
	for i := 1; i < len(steps); i++ {
		prev := steps[i-1].Value
		if prev == 0 {
			continue
		}
		steps[i].DropOff = 100 - (steps[i].Value/prev)*100
	}
	conversion := 0.0
	if len(steps) > 0 && steps[0].Value > 0 {
		conversion = (steps[len(steps)-1].Value / steps[0].Value) * 100
	}
	return FunnelReport{
		Range:          query.Range,
		Segment:        query.Segment,
		Steps:          steps,
		ConversionRate: conversion,
		Goal:           query.Goal,
	}, nil
}

// DemoCohortRepository returns static retention grids.
type DemoCohortRepository struct{}

func (DemoCohortRepository) FetchCohortReport(ctx context.Context, query CohortQuery) (CohortReport, error) {
	rows := []CohortRow{
		{Label: "Jan 2024", Size: 740, Retention: []float64{100, 78, 69, 61, 55}},
		{Label: "Feb 2024", Size: 680, Retention: []float64{100, 81, 72, 63, 57}},
		{Label: "Mar 2024", Size: 705, Retention: []float64{100, 79, 70, 64, 60}},
		{Label: "Apr 2024", Size: 750, Retention: []float64{100, 82, 74, 66, 59}},
	}
	return CohortReport{
		Interval: query.Interval,
		Metric:   query.Metric,
		Rows:     rows[:queryPeriods(query.Periods, len(rows))],
	}, nil
}

func queryPeriods(requested, max int) int {
	if requested <= 0 || requested > max {
		return max
	}
	return requested
}

// DemoAlertRepository returns synthetic alert data.
type DemoAlertRepository struct{}

func (DemoAlertRepository) FetchAlertTrends(ctx context.Context, query AlertTrendQuery) (AlertTrendsReport, error) {
	now := time.Now().UTC()
	days := query.LookbackDays
	if days > 14 {
		days = 14
	}
	series := make([]AlertSeries, 0, days)
	totals := map[string]int{"info": 0, "warning": 0, "critical": 0}
	severities := query.Severities
	if len(severities) == 0 {
		severities = []string{"info", "warning", "critical"}
	}
	for i := days - 1; i >= 0; i-- {
		day := now.AddDate(0, 0, -i)
		counts := map[string]int{}
		for _, sev := range severities {
			value := 5 + (i*len(sev))%9
			counts[sev] = value
			totals[sev] += value
		}
		series = append(series, AlertSeries{Day: day, Counts: counts})
	}
	return AlertTrendsReport{
		Service: query.Service,
		Series:  series,
		Totals:  totals,
	}, nil
}
