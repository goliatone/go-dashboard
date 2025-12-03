package dashboard

import (
	"context"
	"time"
)

var defaultProviders = map[string]Provider{
	"admin.widget.user_stats": ProviderFunc(func(ctx context.Context, meta WidgetContext) (WidgetData, error) {
		title := translateOrFallback(ctx, meta.Translator, "dashboard.widget.user_stats.data_title", meta.Viewer.Locale, "Users", nil)
		return WidgetData{
			"title":  title,
			"metric": meta.Instance.Configuration["metric"],
			"values": map[string]int{"total": 1200, "active": 875, "new": 32},
		}, nil
	}),
	"admin.widget.recent_activity": newRecentActivityProvider(nil),
	"admin.widget.sales_chart": NewSalesChartProvider(
		NewStaticSalesRepository(defaultSalesSeries()),
		NewEChartsProvider("line"),
	),
	"admin.widget.quick_actions": ProviderFunc(func(ctx context.Context, meta WidgetContext) (WidgetData, error) {
		inviteLabel := translateOrFallback(ctx, meta.Translator, "dashboard.widget.quick_actions.invite_user", meta.Viewer.Locale, "Invite user", nil)
		pageLabel := translateOrFallback(ctx, meta.Translator, "dashboard.widget.quick_actions.create_page", meta.Viewer.Locale, "Create page", nil)
		return WidgetData{
			"actions": []map[string]any{
				{"label": inviteLabel, "route": "/admin/users/invite", "icon": "user-plus"},
				{"label": pageLabel, "route": "/admin/pages/new", "icon": "file-plus"},
			},
		}, nil
	}),
	"admin.widget.system_status": ProviderFunc(func(ctx context.Context, meta WidgetContext) (WidgetData, error) {
		dbLabel := translateOrFallback(ctx, meta.Translator, "dashboard.widget.system_status.database", meta.Viewer.Locale, "Database", nil)
		cacheLabel := translateOrFallback(ctx, meta.Translator, "dashboard.widget.system_status.cache", meta.Viewer.Locale, "Cache", nil)
		workerLabel := translateOrFallback(ctx, meta.Translator, "dashboard.widget.system_status.worker", meta.Viewer.Locale, "Worker", nil)
		return WidgetData{
			"checks": []map[string]any{
				{"name": dbLabel, "status": "ok"},
				{"name": cacheLabel, "status": "ok"},
				{"name": workerLabel, "status": "warning"},
			},
		}, nil
	}),
	"admin.widget.analytics_funnel": NewFunnelAnalyticsProvider(DemoFunnelRepository{}),
	"admin.widget.cohort_overview":  NewCohortAnalyticsProvider(DemoCohortRepository{}),
	"admin.widget.alert_trends":     NewAlertTrendsProvider(DemoAlertRepository{}),
}

func newRecentActivityProvider(feed ActivityFeed) Provider {
	return ProviderFunc(func(ctx context.Context, meta WidgetContext) (WidgetData, error) {
		if feed == nil {
			feed = DefaultActivityFeed()
		}
		limit := 10
		if v, ok := meta.Instance.Configuration["limit"].(int); ok && v > 0 {
			limit = v
		}
		items, err := feed.Recent(ctx, meta.Viewer, limit)
		if err != nil {
			return nil, err
		}
		payload := make([]map[string]any, 0, len(items))
		for _, item := range items {
			payload = append(payload, map[string]any{
				"user":    item.User,
				"action":  item.Action,
				"details": item.Details,
				"ago":     item.Ago,
			})
		}
		return WidgetData{"items": payload}, nil
	})
}

func defaultSalesSeries() []SalesSeriesPoint {
	now := time.Now().UTC()
	values := []float64{11800, 12650, 13200, 14150, 15200, 16120}
	points := make([]SalesSeriesPoint, len(values))
	for i, value := range values {
		points[i] = SalesSeriesPoint{
			Timestamp: now.AddDate(0, 0, -7*(len(values)-i)),
			Value:     value,
		}
	}
	return points
}
