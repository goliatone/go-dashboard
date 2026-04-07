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

// NewFunnelAnalyticsProvider wires a FunnelReportRepository into a Provider.
func NewFunnelAnalyticsProvider(repo FunnelReportRepository) Provider {
	if repo == nil {
		repo = DemoFunnelRepository{}
	}
	return NewWidgetProvider(WidgetSpec[FunnelQuery, FunnelReport, JSONViewModel[funnelView]]{
		Definition: WidgetDefinition{Code: "admin.widget.analytics_funnel"},
		DecodeConfig: func(raw map[string]any) (FunnelQuery, error) {
			return extractFunnelQuery(raw), nil
		},
		Fetch: func(ctx context.Context, req WidgetRequest[FunnelQuery]) (FunnelReport, error) {
			return repo.FetchFunnelReport(ctx, req.Config)
		},
		BuildView: func(_ context.Context, report FunnelReport, _ WidgetViewContext[FunnelQuery]) (JSONViewModel[funnelView], error) {
			baseline := 1.0
			if len(report.Steps) > 0 && report.Steps[0].Value > 0 {
				baseline = report.Steps[0].Value
			}
			steps := make([]funnelStepView, 0, len(report.Steps))
			for _, step := range report.Steps {
				percentage := 0.0
				if baseline > 0 {
					percentage = (step.Value / baseline) * 100
				}
				steps = append(steps, funnelStepView{
					Label:    step.Label,
					Value:    step.Value,
					DropOff:  step.DropOff,
					Position: step.Position,
					Percent:  percentage,
				})
			}
			return JSONViewModel[funnelView]{
				Value: funnelView{
					Range:          report.Range,
					Segment:        report.Segment,
					ConversionRate: report.ConversionRate,
					Steps:          steps,
					Goal:           report.Goal,
				},
			}, nil
		},
	})
}

// NewCohortAnalyticsProvider wires cohort repositories for retention widgets.
func NewCohortAnalyticsProvider(repo CohortReportRepository) Provider {
	if repo == nil {
		repo = DemoCohortRepository{}
	}
	return NewWidgetProvider(WidgetSpec[CohortQuery, CohortReport, JSONViewModel[cohortView]]{
		Definition: WidgetDefinition{Code: "admin.widget.cohort_overview"},
		DecodeConfig: func(raw map[string]any) (CohortQuery, error) {
			return extractCohortQuery(raw), nil
		},
		Fetch: func(ctx context.Context, req WidgetRequest[CohortQuery]) (CohortReport, error) {
			return repo.FetchCohortReport(ctx, req.Config)
		},
		BuildView: func(_ context.Context, report CohortReport, _ WidgetViewContext[CohortQuery]) (JSONViewModel[cohortView], error) {
			rows := make([]cohortRowView, 0, len(report.Rows))
			for _, row := range report.Rows {
				rows = append(rows, cohortRowView{
					Label:     row.Label,
					Size:      row.Size,
					Retention: row.Retention,
				})
			}
			return JSONViewModel[cohortView]{
				Value: cohortView{
					Interval: report.Interval,
					Metric:   report.Metric,
					Rows:     rows,
				},
			}, nil
		},
	})
}

// NewAlertTrendsProvider wires alert repositories into a Provider instance.
func NewAlertTrendsProvider(repo AlertTrendsRepository) Provider {
	if repo == nil {
		repo = DemoAlertRepository{}
	}
	return NewWidgetProvider(WidgetSpec[AlertTrendQuery, AlertTrendsReport, JSONViewModel[alertTrendsView]]{
		Definition: WidgetDefinition{Code: "admin.widget.alert_trends"},
		DecodeConfig: func(raw map[string]any) (AlertTrendQuery, error) {
			return extractAlertQuery(raw), nil
		},
		Fetch: func(ctx context.Context, req WidgetRequest[AlertTrendQuery]) (AlertTrendsReport, error) {
			return repo.FetchAlertTrends(ctx, req.Config)
		},
		BuildView: func(_ context.Context, report AlertTrendsReport, meta WidgetViewContext[AlertTrendQuery]) (JSONViewModel[alertTrendsView], error) {
			order := normalizeSeverities(meta.Request.Config.Severities)
			series := make([]alertSeriesView, 0, len(report.Series))
			for _, bucket := range report.Series {
				series = append(series, alertSeriesView{
					Day:    bucket.Day.Format("2006-01-02"),
					Counts: countsForOrder(order, bucket.Counts),
				})
			}
			return JSONViewModel[alertTrendsView]{
				Value: alertTrendsView{
					LookbackDays: meta.Request.Config.LookbackDays,
					Severities:   countsForOrder(order, report.Totals),
					Service:      report.Service,
					Series:       series,
				},
			}, nil
		},
	})
}

type funnelStepView struct {
	Label    string  `json:"label"`
	Value    float64 `json:"value"`
	DropOff  float64 `json:"dropoff"`
	Position int     `json:"position"`
	Percent  float64 `json:"percent"`
}

type funnelView struct {
	Range          string           `json:"range"`
	Segment        string           `json:"segment"`
	ConversionRate float64          `json:"conversion_rate"`
	Steps          []funnelStepView `json:"steps"`
	Goal           float64          `json:"goal"`
}

type cohortRowView struct {
	Label     string    `json:"label"`
	Size      int       `json:"size"`
	Retention []float64 `json:"retention"`
}

type cohortView struct {
	Interval string          `json:"interval"`
	Metric   string          `json:"metric"`
	Rows     []cohortRowView `json:"rows"`
}

type alertSeriesView struct {
	Day    string           `json:"day"`
	Counts []map[string]any `json:"counts"`
}

type alertTrendsView struct {
	LookbackDays int               `json:"lookback_days"`
	Severities   []map[string]any  `json:"severities"`
	Service      string            `json:"service"`
	Series       []alertSeriesView `json:"series"`
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
	severities := normalizeSeverities(sliceOr(config["severity"], []string{"warning", "critical"}))
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

func normalizeSeverities(values []string) []string {
	if len(values) == 0 {
		return []string{"warning", "critical"}
	}
	order := []string{}
	seen := map[string]bool{}
	preferred := []string{"critical", "warning", "info"}
	for _, candidate := range preferred {
		for _, v := range values {
			if v == candidate && !seen[v] {
				order = append(order, v)
				seen[v] = true
			}
		}
	}
	for _, v := range values {
		if !seen[v] {
			order = append(order, v)
			seen[v] = true
		}
	}
	return order
}

func countsForOrder(order []string, counts map[string]int) []map[string]any {
	rows := make([]map[string]any, 0, len(order))
	for _, severity := range order {
		rows = append(rows, map[string]any{
			"severity": severity,
			"count":    counts[severity],
		})
	}
	return rows
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
	days := min(query.LookbackDays, 14)
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
