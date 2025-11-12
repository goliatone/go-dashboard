package analytics

import (
	"context"

	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

// FunnelClient fetches conversion funnel reports from upstream analytics services.
type FunnelClient interface {
	FetchFunnel(ctx context.Context, query dashboard.FunnelQuery) (dashboard.FunnelReport, error)
}

// CohortClient fetches cohort/retention metrics from BI systems.
type CohortClient interface {
	FetchCohorts(ctx context.Context, query dashboard.CohortQuery) (dashboard.CohortReport, error)
}

// AlertClient fetches alert trend metrics from observability providers.
type AlertClient interface {
	FetchAlerts(ctx context.Context, query dashboard.AlertTrendQuery) (dashboard.AlertTrendsReport, error)
}

// Client is a convenience union for services that implement all analytics calls.
type Client interface {
	FunnelClient
	CohortClient
	AlertClient
}
