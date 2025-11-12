package analytics

import (
	"context"

	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

// NewFunnelRepository adapts an analytics client into a dashboard repository.
func NewFunnelRepository(client FunnelClient) dashboard.FunnelReportRepository {
	return &funnelRepository{client: client}
}

type funnelRepository struct {
	client FunnelClient
}

func (r *funnelRepository) FetchFunnelReport(ctx context.Context, query dashboard.FunnelQuery) (dashboard.FunnelReport, error) {
	return r.client.FetchFunnel(ctx, query)
}

// NewCohortRepository adapts the analytics client for cohort widgets.
func NewCohortRepository(client CohortClient) dashboard.CohortReportRepository {
	return &cohortRepository{client: client}
}

type cohortRepository struct {
	client CohortClient
}

func (r *cohortRepository) FetchCohortReport(ctx context.Context, query dashboard.CohortQuery) (dashboard.CohortReport, error) {
	return r.client.FetchCohorts(ctx, query)
}

// NewAlertRepository adapts observability clients into the alert widget.
func NewAlertRepository(client AlertClient) dashboard.AlertTrendsRepository {
	return &alertRepository{client: client}
}

type alertRepository struct {
	client AlertClient
}

func (r *alertRepository) FetchAlertTrends(ctx context.Context, query dashboard.AlertTrendQuery) (dashboard.AlertTrendsReport, error) {
	return r.client.FetchAlerts(ctx, query)
}
