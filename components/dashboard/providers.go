package dashboard

import (
	"context"
	"math/rand"
	"time"
)

var defaultProviders = map[string]Provider{
	"admin.widget.user_stats": ProviderFunc(func(ctx context.Context, meta WidgetContext) (WidgetData, error) {
		return WidgetData{
			"title":  "Users",
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
	"admin.widget.quick_actions": ProviderFunc(func(context.Context, WidgetContext) (WidgetData, error) {
		return WidgetData{
			"actions": []map[string]any{
				{"label": "Invite user", "route": "/admin/users/invite", "icon": "user-plus"},
				{"label": "Create page", "route": "/admin/pages/new", "icon": "file-plus"},
			},
		}, nil
	}),
	"admin.widget.system_status": ProviderFunc(func(context.Context, WidgetContext) (WidgetData, error) {
		return WidgetData{
			"checks": []map[string]any{
				{"name": "Database", "status": "ok"},
				{"name": "Cache", "status": "ok"},
				{"name": "Worker", "status": "warning"},
			},
		}, nil
	}),
	"admin.widget.analytics_funnel": NewFunnelAnalyticsProvider(DemoFunnelRepository{}),
	"admin.widget.cohort_overview":  NewCohortAnalyticsProvider(DemoCohortRepository{}),
	"admin.widget.alert_trends":     NewAlertTrendsProvider(DemoAlertRepository{}),
}
