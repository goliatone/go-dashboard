package dashboard

import (
	"context"
	"reflect"
	"testing"
	"time"
)

func TestFunnelProviderFetchesThroughRepository(t *testing.T) {
	repo := &stubFunnelRepo{}
	provider := NewFunnelAnalyticsProvider(repo)
	_, err := provider.Fetch(context.Background(), WidgetContext{
		Instance: WidgetInstance{
			Configuration: map[string]any{
				"range":   "14d",
				"segment": "enterprise",
				"goal":    52,
			},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.query.Range != "14d" || repo.query.Segment != "enterprise" || repo.query.Goal != 52 {
		t.Fatalf("unexpected funnel query: %#v", repo.query)
	}
	if repo.calls != 1 {
		t.Fatalf("expected repository to be called once, got %d", repo.calls)
	}
}

func TestCohortProviderReturnsRows(t *testing.T) {
	repo := &stubCohortRepo{
		report: CohortReport{
			Interval: "weekly",
			Metric:   "retained",
			Rows: []CohortRow{
				{Label: "Week 1", Size: 100, Retention: []float64{100, 80, 70}},
			},
		},
	}
	provider := NewCohortAnalyticsProvider(repo)
	data, err := provider.Fetch(context.Background(), WidgetContext{
		Instance: WidgetInstance{
			Configuration: map[string]any{
				"interval": "monthly",
				"periods":  4,
				"metric":   "active",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	rows, _ := data["rows"].([]map[string]any)
	if len(rows) != 1 || rows[0]["label"] != "Week 1" {
		t.Fatalf("expected mapped rows, got %#v", data["rows"])
	}
	if repo.query.Interval != "monthly" || repo.query.Periods != 4 || repo.query.Metric != "active" {
		t.Fatalf("unexpected cohort query %#v", repo.query)
	}
}

func TestAlertProviderShapesSeries(t *testing.T) {
	now := time.Date(2024, 11, 10, 0, 0, 0, 0, time.UTC)
	repo := &stubAlertRepo{
		report: AlertTrendsReport{
			Service: "api",
			Totals:  map[string]int{"critical": 5},
			Series: []AlertSeries{
				{Day: now, Counts: map[string]int{"critical": 3}},
			},
		},
	}
	provider := NewAlertTrendsProvider(repo)
	data, err := provider.Fetch(context.Background(), WidgetContext{
		Instance: WidgetInstance{
			Configuration: map[string]any{
				"lookback_days": 10,
				"severity":      []string{"critical"},
				"service":       "api",
			},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	series, _ := data["series"].([]map[string]any)
	if len(series) != 1 || series[0]["day"] != "2024-11-10" {
		t.Fatalf("unexpected series payload: %#v", data["series"])
	}
	if repo.query.Service != "api" || repo.query.LookbackDays != 10 || !reflect.DeepEqual(repo.query.Severities, []string{"critical"}) {
		t.Fatalf("unexpected alert query %#v", repo.query)
	}
}

type stubFunnelRepo struct {
	query FunnelQuery
	calls int
}

func (s *stubFunnelRepo) FetchFunnelReport(ctx context.Context, query FunnelQuery) (FunnelReport, error) {
	s.calls++
	s.query = query
	return FunnelReport{Range: query.Range, Segment: query.Segment, Goal: query.Goal}, nil
}

type stubCohortRepo struct {
	query  CohortQuery
	report CohortReport
}

func (s *stubCohortRepo) FetchCohortReport(ctx context.Context, query CohortQuery) (CohortReport, error) {
	s.query = query
	return s.report, nil
}

type stubAlertRepo struct {
	query  AlertTrendQuery
	report AlertTrendsReport
}

func (s *stubAlertRepo) FetchAlertTrends(ctx context.Context, query AlertTrendQuery) (AlertTrendsReport, error) {
	s.query = query
	return s.report, nil
}
