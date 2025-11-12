package analytics

import (
	"context"
	"testing"
	"time"

	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

func TestRepositoriesDelegateToClient(t *testing.T) {
	mock := NewMockClient(MockData{
		Funnel: dashboard.FunnelReport{Range: "30d", Steps: []dashboard.FunnelStep{{Label: "Visitors"}}},
		Cohort: dashboard.CohortReport{Interval: "weekly", Rows: []dashboard.CohortRow{{Label: "Week 1"}}},
		Alerts: dashboard.AlertTrendsReport{Service: "api", Series: []dashboard.AlertSeries{{Day: time.Now().UTC(), Counts: map[string]int{"critical": 1}}}},
	})

	funnelRepo := NewFunnelRepository(mock)
	if report, err := funnelRepo.FetchFunnelReport(context.Background(), dashboard.FunnelQuery{}); err != nil || len(report.Steps) != 1 {
		t.Fatalf("funnel repo returned %v, %v", report, err)
	}

	cohortRepo := NewCohortRepository(mock)
	if report, err := cohortRepo.FetchCohortReport(context.Background(), dashboard.CohortQuery{}); err != nil || len(report.Rows) != 1 {
		t.Fatalf("cohort repo returned %v, %v", report, err)
	}

	alertRepo := NewAlertRepository(mock)
	if report, err := alertRepo.FetchAlertTrends(context.Background(), dashboard.AlertTrendQuery{}); err != nil || len(report.Series) != 1 {
		t.Fatalf("alert repo returned %v, %v", report, err)
	}
}
