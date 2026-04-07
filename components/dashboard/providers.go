package dashboard

import (
	"context"
	"time"
)

var defaultProviders = map[string]Provider{
	"admin.widget.user_stats":      newUserStatsProvider(),
	"admin.widget.recent_activity": newRecentActivityProvider(nil),
	"admin.widget.sales_chart": NewSalesChartProvider(
		NewStaticSalesRepository(defaultSalesSeries()),
		NewEChartsProvider("line"),
	),
	"admin.widget.quick_actions":    newQuickActionsProvider(),
	"admin.widget.system_status":    newSystemStatusProvider(),
	"admin.widget.analytics_funnel": NewFunnelAnalyticsProvider(DemoFunnelRepository{}),
	"admin.widget.cohort_overview":  NewCohortAnalyticsProvider(DemoCohortRepository{}),
	"admin.widget.alert_trends":     NewAlertTrendsProvider(DemoAlertRepository{}),
}

func registerDefaultWidgetRuntimes(reg *Registry) {
	if reg == nil {
		return
	}
	for code, provider := range defaultProviders {
		_ = reg.RegisterProvider(code, provider)
	}
}

func newRecentActivityRuntime(feed ActivityFeed) widgetSpecRuntime {
	return NewWidgetRuntime(newRecentActivitySpec(feed))
}

type userStatsConfig struct {
	Metric string `json:"metric,omitempty"`
}

type userStatsData struct {
	Title  string
	Metric string
	Values map[string]int
}

type userStatsView struct {
	Title  string         `json:"title"`
	Metric string         `json:"metric,omitempty"`
	Values map[string]int `json:"values"`
}

func newUserStatsProvider() Provider {
	return NewWidgetProvider(WidgetSpec[userStatsConfig, userStatsData, JSONViewModel[userStatsView]]{
		Definition: WidgetDefinition{Code: "admin.widget.user_stats"},
		Fetch: func(ctx context.Context, req WidgetRequest[userStatsConfig]) (userStatsData, error) {
			title := translateOrFallback(ctx, req.Translator, "dashboard.widget.user_stats.data_title", req.Viewer.Locale, "Users", nil)
			return userStatsData{
				Title:  title,
				Metric: req.Config.Metric,
				Values: map[string]int{"total": 1200, "active": 875, "new": 32},
			}, nil
		},
		BuildView: func(_ context.Context, data userStatsData, _ WidgetViewContext[userStatsConfig]) (JSONViewModel[userStatsView], error) {
			return JSONViewModel[userStatsView]{
				Value: userStatsView{
					Title:  data.Title,
					Metric: data.Metric,
					Values: data.Values,
				},
			}, nil
		},
	})
}

type recentActivityConfig struct {
	Limit int `json:"limit,omitempty"`
}

type recentActivityData struct {
	Items []ActivityItem
}

type recentActivityItemView struct {
	User    string        `json:"user"`
	Action  string        `json:"action"`
	Details string        `json:"details"`
	Ago     time.Duration `json:"ago"`
}

type recentActivityView struct {
	Items []recentActivityItemView `json:"items"`
}

func newRecentActivityProvider(feed ActivityFeed) Provider {
	return NewWidgetProvider(newRecentActivitySpec(feed))
}

func newRecentActivitySpec(feed ActivityFeed) WidgetSpec[recentActivityConfig, recentActivityData, JSONViewModel[recentActivityView]] {
	return WidgetSpec[recentActivityConfig, recentActivityData, JSONViewModel[recentActivityView]]{
		Definition: WidgetDefinition{Code: "admin.widget.recent_activity"},
		Fetch: func(ctx context.Context, req WidgetRequest[recentActivityConfig]) (recentActivityData, error) {
			if feed == nil {
				feed = DefaultActivityFeed()
			}
			limit := req.Config.Limit
			if limit <= 0 {
				limit = 10
			}
			items, err := feed.Recent(ctx, req.Viewer, limit)
			if err != nil {
				return recentActivityData{}, err
			}
			return recentActivityData{Items: items}, nil
		},
		BuildView: func(_ context.Context, data recentActivityData, _ WidgetViewContext[recentActivityConfig]) (JSONViewModel[recentActivityView], error) {
			items := make([]recentActivityItemView, 0, len(data.Items))
			for _, item := range data.Items {
				items = append(items, recentActivityItemView{
					User:    item.User,
					Action:  item.Action,
					Details: item.Details,
					Ago:     item.Ago,
				})
			}
			return JSONViewModel[recentActivityView]{
				Value: recentActivityView{Items: items},
			}, nil
		},
	}
}

type quickActionView struct {
	Label string `json:"label"`
	Route string `json:"route"`
	Icon  string `json:"icon"`
}

type quickActionsView struct {
	Actions []quickActionView `json:"actions"`
}

func newQuickActionsProvider() Provider {
	return NewWidgetProvider(WidgetSpec[struct{}, quickActionsView, JSONViewModel[quickActionsView]]{
		Definition: WidgetDefinition{Code: "admin.widget.quick_actions"},
		Fetch: func(ctx context.Context, req WidgetRequest[struct{}]) (quickActionsView, error) {
			inviteLabel := translateOrFallback(ctx, req.Translator, "dashboard.widget.quick_actions.invite_user", req.Viewer.Locale, "Invite user", nil)
			pageLabel := translateOrFallback(ctx, req.Translator, "dashboard.widget.quick_actions.create_page", req.Viewer.Locale, "Create page", nil)
			return quickActionsView{
				Actions: []quickActionView{
					{Label: inviteLabel, Route: "/admin/users/invite", Icon: "user-plus"},
					{Label: pageLabel, Route: "/admin/pages/new", Icon: "file-plus"},
				},
			}, nil
		},
		BuildView: func(_ context.Context, data quickActionsView, _ WidgetViewContext[struct{}]) (JSONViewModel[quickActionsView], error) {
			return JSONViewModel[quickActionsView]{Value: data}, nil
		},
	})
}

type systemStatusCheckView struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type systemStatusView struct {
	Checks []systemStatusCheckView `json:"checks"`
}

func newSystemStatusProvider() Provider {
	return NewWidgetProvider(WidgetSpec[struct{}, systemStatusView, JSONViewModel[systemStatusView]]{
		Definition: WidgetDefinition{Code: "admin.widget.system_status"},
		Fetch: func(ctx context.Context, req WidgetRequest[struct{}]) (systemStatusView, error) {
			dbLabel := translateOrFallback(ctx, req.Translator, "dashboard.widget.system_status.database", req.Viewer.Locale, "Database", nil)
			cacheLabel := translateOrFallback(ctx, req.Translator, "dashboard.widget.system_status.cache", req.Viewer.Locale, "Cache", nil)
			workerLabel := translateOrFallback(ctx, req.Translator, "dashboard.widget.system_status.worker", req.Viewer.Locale, "Worker", nil)
			return systemStatusView{
				Checks: []systemStatusCheckView{
					{Name: dbLabel, Status: "ok"},
					{Name: cacheLabel, Status: "ok"},
					{Name: workerLabel, Status: "warning"},
				},
			}, nil
		},
		BuildView: func(_ context.Context, data systemStatusView, _ WidgetViewContext[struct{}]) (JSONViewModel[systemStatusView], error) {
			return JSONViewModel[systemStatusView]{Value: data}, nil
		},
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
