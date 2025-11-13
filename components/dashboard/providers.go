package dashboard

import (
	"context"
	"math/rand"
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
	"admin.widget.recent_activity": ProviderFunc(func(ctx context.Context, meta WidgetContext) (WidgetData, error) {
		limit := 10
		if v, ok := meta.Instance.Configuration["limit"].(int); ok && v > 0 {
			limit = v
		}
		items := make([]map[string]any, 0, limit)
		for i := 0; i < limit; i++ {
			items = append(items, map[string]any{
				"user":    "User " + string(rune('A'+i)),
				"action":  "updated content",
				"ago":     time.Duration(i+1) * time.Minute,
				"details": "Placeholder event",
			})
		}
		return WidgetData{"items": items}, nil
	}),
	"admin.widget.sales_chart": ProviderFunc(func(ctx context.Context, meta WidgetContext) (WidgetData, error) {
		points := make([]int, 7)
		for i := range points {
			points[i] = rand.Intn(100) //nolint:gosec
		}
		return WidgetData{"series": points, "range": meta.Instance.Configuration["range"]}, nil
	}),
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
