package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	router "github.com/goliatone/go-router"

	"github.com/goliatone/go-dashboard/components/dashboard"
	"github.com/goliatone/go-dashboard/components/dashboard/commands"
	"github.com/goliatone/go-dashboard/components/dashboard/gorouter"
	"github.com/goliatone/go-dashboard/components/dashboard/httpapi"
	"github.com/goliatone/go-dashboard/pkg/analytics"
	dashboardpkg "github.com/goliatone/go-dashboard/pkg/dashboard"
	"github.com/goliatone/go-dashboard/pkg/goadmin"
)

func main() {
	ctx := context.Background()

	service, registry, store := setupDemoDashboard(ctx)

	renderer := newSampleRenderer()
	controller := dashboard.NewController(dashboard.ControllerOptions{
		Service:  service,
		Renderer: renderer,
	})

	executor := &httpapi.CommandExecutor{
		AssignCommander:  commands.NewAssignWidgetCommand(service, nil),
		RemoveCommander:  commands.NewRemoveWidgetCommand(service, nil),
		ReorderCommander: commands.NewReorderWidgetsCommand(service, nil),
		RefreshCommander: commands.NewRefreshWidgetCommand(service, nil),
		PrefsCommander:   commands.NewSaveLayoutPreferencesCommand(service, nil),
	}

	hook := dashboard.NewBroadcastHook()

	server := router.NewFiberAdapter()
	appRouter := server.Router()
	if err := gorouter.Register(gorouter.Config[*fiber.App]{
		Router:     appRouter,
		Controller: controller,
		API:        executor,
		Broadcast:  hook,
		ViewerResolver: func(ctx router.Context) dashboard.ViewerContext {
			return dashboard.ViewerContext{UserID: "admin@example.com", Roles: []string{"admin"}, Locale: "en"}
		},
	}); err != nil {
		log.Fatalf("register routes: %v", err)
	}

	admin, err := goadmin.New(goadmin.Config{
		EnableDashboard: true,
		Service: dashboardpkg.NewService(dashboard.Options{
			WidgetStore: store,
			Providers:   registry,
		}),
		MenuBuilder: &loggingMenuBuilder{},
	})
	if err != nil {
		log.Fatalf("goadmin init: %v", err)
	}
	if err := admin.Bootstrap(ctx); err != nil {
		log.Fatalf("bootstrap: %v", err)
	}

	log.Printf("dashboard routes ready: http://localhost:8080/admin/dashboard")
	log.Printf("API endpoints: POST %s, DELETE %s, WebSocket %s",
		"/admin/dashboard/widgets",
		"/admin/dashboard/widgets/:id",
		"/admin/dashboard/ws",
	)
	if err := server.Serve(":9876"); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// --- In-memory demo dependencies below. ---

type memoryWidgetStore struct {
	mu           sync.Mutex
	areas        map[string]dashboard.WidgetAreaDefinition
	definitions  map[string]dashboard.WidgetDefinition
	instances    map[string]dashboard.WidgetInstance
	assignments  map[string][]string
	nextInstance int
}

func newMemoryWidgetStore() *memoryWidgetStore {
	return &memoryWidgetStore{
		areas:       map[string]dashboard.WidgetAreaDefinition{},
		definitions: map[string]dashboard.WidgetDefinition{},
		instances:   map[string]dashboard.WidgetInstance{},
		assignments: map[string][]string{},
	}
}

func (s *memoryWidgetStore) EnsureArea(ctx context.Context, def dashboard.WidgetAreaDefinition) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.areas[def.Code]
	s.areas[def.Code] = def
	return !exists, nil
}

func (s *memoryWidgetStore) EnsureDefinition(ctx context.Context, def dashboard.WidgetDefinition) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, exists := s.definitions[def.Code]
	s.definitions[def.Code] = def
	return !exists, nil
}

func (s *memoryWidgetStore) CreateInstance(ctx context.Context, input dashboard.CreateWidgetInstanceInput) (dashboard.WidgetInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextInstance++
	id := fmt.Sprintf("inst-%d", s.nextInstance)
	instance := dashboard.WidgetInstance{
		ID:            id,
		DefinitionID:  input.DefinitionID,
		Configuration: input.Configuration,
		Metadata:      input.Metadata,
	}
	s.instances[id] = instance
	return instance, nil
}

func (s *memoryWidgetStore) DeleteInstance(ctx context.Context, instanceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.instances, instanceID)
	for area, ids := range s.assignments {
		s.assignments[area] = filterIDs(ids, instanceID)
	}
	return nil
}

func (s *memoryWidgetStore) AssignInstance(ctx context.Context, input dashboard.AssignWidgetInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	order := s.assignments[input.AreaCode]
	if input.Position != nil && *input.Position <= len(order) {
		idx := *input.Position
		order = append(order[:idx], append([]string{input.InstanceID}, order[idx:]...)...)
	} else {
		order = append(order, input.InstanceID)
	}
	s.assignments[input.AreaCode] = order
	return nil
}

func (s *memoryWidgetStore) ReorderArea(ctx context.Context, input dashboard.ReorderAreaInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assignments[input.AreaCode] = append([]string{}, input.WidgetIDs...)
	return nil
}

func (s *memoryWidgetStore) ResolveArea(ctx context.Context, input dashboard.ResolveAreaInput) (dashboard.ResolvedArea, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.assignments[input.AreaCode]
	widgets := make([]dashboard.WidgetInstance, 0, len(ids))
	for _, id := range ids {
		if inst, ok := s.instances[id]; ok {
			widgets = append(widgets, inst)
		}
	}
	return dashboard.ResolvedArea{
		AreaCode: input.AreaCode,
		Widgets:  widgets,
	}, nil
}

func filterIDs(ids []string, drop string) []string {
	out := ids[:0]
	for _, id := range ids {
		if id != drop {
			out = append(out, id)
		}
	}
	return out
}

type loggingMenuBuilder struct{}

func (loggingMenuBuilder) EnsureMenuItem(context.Context, string, goadmin.MenuItem) error {
	return nil
}

type sampleRenderer struct {
	tmpl *template.Template
}

func newSampleRenderer() sampleRenderer {
	tmpl := template.Must(template.New("dashboard").Funcs(template.FuncMap{
		"isType":      func(definition, code string) bool { return definition == code },
		"widgetTitle": widgetTitle,
		"add":         func(a, b int) int { return a + b },
		"formatNumber": func(value any) string {
			return formatNumber(value)
		},
		"percent": func(value any) string {
			return percentValue(value)
		},
		"statusClass": func(value any) string {
			return statusClass(fmt.Sprint(value))
		},
		"valueOr": func(primary, fallback any) any {
			return valueOr(primary, fallback)
		},
	}).Parse(dashboardTemplate))
	return sampleRenderer{tmpl: tmpl}
}

func registerAnalyticsProviders(reg *dashboard.Registry) error {
	mock := analytics.NewMockClient(analytics.MockData{
		Funnel: dashboard.FunnelReport{
			Range:          "30d",
			Segment:        "enterprise",
			ConversionRate: 48.2,
			Goal:           52,
			Steps: []dashboard.FunnelStep{
				{Label: "Visitors", Value: 18500, Position: 0},
				{Label: "Signed Up", Value: 7200, Position: 1},
				{Label: "Activated", Value: 3100, Position: 2},
				{Label: "Paying", Value: 980, Position: 3},
			},
		},
		Cohort: dashboard.CohortReport{
			Interval: "weekly",
			Metric:   "retained",
			Rows: []dashboard.CohortRow{
				{Label: "Week 1", Size: 740, Retention: []float64{100, 82, 74, 66}},
				{Label: "Week 2", Size: 705, Retention: []float64{100, 79, 70, 64}},
			},
		},
		Alerts: dashboard.AlertTrendsReport{
			Service: "goadmin",
			Totals:  map[string]int{"critical": 12, "warning": 27},
			Series: []dashboard.AlertSeries{
				{Day: time.Now().UTC().AddDate(0, 0, -1), Counts: map[string]int{"critical": 2, "warning": 5}},
				{Day: time.Now().UTC(), Counts: map[string]int{"critical": 10, "warning": 22}},
			},
		},
	})

	if err := reg.RegisterProvider("admin.widget.analytics_funnel", dashboard.NewFunnelAnalyticsProvider(analytics.NewFunnelRepository(mock))); err != nil {
		return err
	}
	if err := reg.RegisterProvider("admin.widget.cohort_overview", dashboard.NewCohortAnalyticsProvider(analytics.NewCohortRepository(mock))); err != nil {
		return err
	}
	if err := reg.RegisterProvider("admin.widget.alert_trends", dashboard.NewAlertTrendsProvider(analytics.NewAlertRepository(mock))); err != nil {
		return err
	}
	return nil
}

func registerDemoContentProviders(reg *dashboard.Registry) error {
	providers := map[string]dashboard.Provider{
		"admin.widget.user_stats": dashboard.ProviderFunc(func(ctx context.Context, meta dashboard.WidgetContext) (dashboard.WidgetData, error) {
			return dashboard.WidgetData{
				"title": "Account Health",
				"values": map[string]any{
					"total":  16804,
					"active": 12034,
					"new":    482,
				},
			}, nil
		}),
		"admin.widget.recent_activity": dashboard.ProviderFunc(func(ctx context.Context, meta dashboard.WidgetContext) (dashboard.WidgetData, error) {
			return dashboard.WidgetData{
				"items": demoActivityFeed(time.Now()),
			}, nil
		}),
		"admin.widget.quick_actions": dashboard.ProviderFunc(func(context.Context, dashboard.WidgetContext) (dashboard.WidgetData, error) {
			return dashboard.WidgetData{
				"actions": demoQuickActions(),
			}, nil
		}),
		"admin.widget.system_status": dashboard.ProviderFunc(func(context.Context, dashboard.WidgetContext) (dashboard.WidgetData, error) {
			return dashboard.WidgetData{
				"checks": demoStatusChecks(),
			}, nil
		}),
	}
	for code, provider := range providers {
		if err := reg.RegisterProvider(code, provider); err != nil {
			return err
		}
	}
	return nil
}

func setupDemoDashboard(ctx context.Context) (*dashboard.Service, *dashboard.Registry, *memoryWidgetStore) {
	store := newMemoryWidgetStore()
	registry := dashboard.NewRegistry()

	customDefinition := dashboard.WidgetDefinition{
		Code:        "demo.widget.welcome",
		Name:        "Welcome Banner",
		Description: "Greets the signed-in administrator.",
		Category:    "demo",
		Schema: map[string]any{
			"type":       "object",
			"properties": map[string]any{"message": map[string]any{"type": "string"}},
		},
	}
	_, _ = store.EnsureDefinition(ctx, customDefinition)
	_ = registry.RegisterDefinition(customDefinition)
	_ = registry.RegisterProvider(customDefinition.Code, dashboard.ProviderFunc(func(ctx context.Context, meta dashboard.WidgetContext) (dashboard.WidgetData, error) {
		return dashboard.WidgetData{
			"headline": fmt.Sprintf("Hey %s 👋", meta.Viewer.UserID),
			"message":  meta.Instance.Configuration["message"],
		}, nil
	}))

	service := dashboard.NewService(dashboard.Options{
		WidgetStore: store,
		Providers:   registry,
	})

	seed := commands.NewSeedDashboardCommand(store, registry, service, nil)
	if err := seed.Execute(ctx, commands.SeedDashboardInput{SeedLayout: false}); err != nil {
		log.Fatalf("seed dashboard: %v", err)
	}

	if err := registerAnalyticsProviders(registry); err != nil {
		log.Fatalf("register analytics providers: %v", err)
	}
	if err := registerDemoContentProviders(registry); err != nil {
		log.Fatalf("register demo providers: %v", err)
	}

	addWidgetOrDie(ctx, service, "conversion funnel", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.analytics_funnel",
		AreaCode:     "admin.dashboard.main",
		Position:     intPtr(0),
		Configuration: map[string]any{
			"range":   "30d",
			"segment": "enterprise",
			"goal":    52,
		},
	})
	addWidgetOrDie(ctx, service, "welcome banner", dashboard.AddWidgetRequest{
		DefinitionID: customDefinition.Code,
		AreaCode:     "admin.dashboard.main",
		Position:     intPtr(1),
		Configuration: map[string]any{
			"message": "Operations look steady. Use this space for runbook snippets or rotating notices.",
		},
	})
	addWidgetOrDie(ctx, service, "user stats", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.user_stats",
		AreaCode:     "admin.dashboard.main",
		Position:     intPtr(2),
		Configuration: map[string]any{
			"metric": "active",
		},
	})
	addWidgetOrDie(ctx, service, "cohort overview", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.cohort_overview",
		AreaCode:     "admin.dashboard.main",
		Position:     intPtr(3),
		Configuration: map[string]any{
			"interval": "weekly",
			"periods":  6,
			"metric":   "retained",
		},
	})
	addWidgetOrDie(ctx, service, "activity feed", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.recent_activity",
		AreaCode:     "admin.dashboard.sidebar",
		Position:     intPtr(0),
		Configuration: map[string]any{
			"limit": 5,
		},
	})
	addWidgetOrDie(ctx, service, "system status", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.system_status",
		AreaCode:     "admin.dashboard.sidebar",
		Position:     intPtr(1),
	})
	addWidgetOrDie(ctx, service, "quick actions", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.quick_actions",
		AreaCode:     "admin.dashboard.footer",
		Position:     intPtr(0),
	})
	addWidgetOrDie(ctx, service, "alert trends", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.alert_trends",
		AreaCode:     "admin.dashboard.footer",
		Position:     intPtr(1),
		Configuration: map[string]any{
			"lookback_days": 7,
			"severity":      []any{"critical", "warning"},
			"service":       "Checkout API",
		},
	})
	return service, registry, store
}

func demoActivityFeed(now time.Time) []map[string]any {
	entries := []struct {
		User    string
		Action  string
		Context string
		When    time.Duration
	}{
		{"Candice Reed", "published the spring pricing update", "Billing · Plan v3 rollout", 5 * time.Minute},
		{"Noah Patel", "invited 24 enterprise seats", "Acme Industrial — Enterprise", 22 * time.Minute},
		{"Marcos Valle", "resolved 14 aging invoices", "Finance · Treasury automation", 49 * time.Minute},
		{"Sara Ndlovu", "shipped a dashboard theme change", "Design System · Canary env", 2 * time.Hour},
		{"Elena Ibarra", "closed incident #782", "Checkout API · On-call", 6 * time.Hour},
	}
	items := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		items = append(items, map[string]any{
			"user":   entry.User,
			"action": entry.Action,
			"ago":    formatAgo(now.Add(-entry.When)),
			"detail": entry.Context,
		})
	}
	return items
}

func demoQuickActions() []map[string]any {
	return []map[string]any{
		{"label": "Invite sales team", "route": "/admin/users/invite", "description": "Bulk import SDR seats", "icon": "users"},
		{"label": "Plan change simulator", "route": "/admin/billing/simulator", "description": "Estimate ARR impact", "icon": "activity"},
		{"label": "Add status update", "route": "/admin/incidents/new", "description": "Publish to StatusPage", "icon": "alert-circle"},
		{"label": "Create automation", "route": "/admin/workflows/new", "description": "Connect alerts to Zendesk", "icon": "zap"},
	}
}

func demoStatusChecks() []map[string]any {
	return []map[string]any{
		{"name": "Checkout API", "status": "healthy"},
		{"name": "Billing jobs", "status": "degraded"},
		{"name": "Notifications", "status": "healthy"},
		{"name": "Background sync", "status": "investigating"},
	}
}

func formatAgo(ts time.Time) string {
	diff := time.Since(ts)
	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%dh", int(diff.Hours()))
	}
	days := int(diff.Hours()) / 24
	return fmt.Sprintf("%dd", days)
}

func (r sampleRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	view := toDashboardView(data)
	var buf bytes.Buffer
	if err := r.tmpl.Execute(&buf, view); err != nil {
		return "", err
	}
	if len(out) > 0 && out[0] != nil {
		if _, err := io.Copy(out[0], bytes.NewReader(buf.Bytes())); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

type dashboardView struct {
	Title       string
	Description string
	LastUpdated string
	Areas       []areaView
}

type areaView struct {
	Code    string
	Widgets []widgetView
}

type widgetView struct {
	ID         string
	Definition string
	Config     map[string]any
	Data       map[string]any
}

func toDashboardView(data any) dashboardView {
	raw, _ := data.(map[string]any)
	view := dashboardView{
		Title:       stringOrDefault(raw["title"], "Dashboard"),
		Description: stringOrDefault(raw["description"], ""),
		LastUpdated: time.Now().Format("Jan 2 · 3:04 PM MST"),
	}
	if ordered := orderedAreas(raw["ordered_areas"]); len(ordered) > 0 {
		for _, areaRaw := range ordered {
			view.Areas = append(view.Areas, buildAreaView(areaRaw, areaCodeOrSlot(areaRaw)))
		}
		return view
	}
	if areas, ok := raw["areas"].(map[string]any); ok {
		appendAreasByOrder(&view, areas)
	}
	return view
}

func buildAreaView(areaRaw map[string]any, fallback string) areaView {
	area := areaView{Code: stringOrDefault(areaRaw["code"], fallback)}
	for _, widgetMap := range toWidgetMaps(areaRaw["widgets"]) {
		widget := widgetView{
			ID:         stringOrDefault(widgetMap["id"], ""),
			Definition: extractDefinition(widgetMap),
		}
		if cfg, ok := widgetMap["config"].(map[string]any); ok {
			widget.Config = cfg
		}
		if dataMap := toDataMap(widgetMap["data"]); dataMap != nil {
			widget.Data = dataMap
		} else if widgetMap["data"] != nil {
			widget.Data = map[string]any{"value": widgetMap["data"]}
		}
		area.Widgets = append(area.Widgets, widget)
	}
	return area
}

func orderedAreas(raw any) []map[string]any {
	switch v := raw.(type) {
	case []map[string]any:
		return v
	case []any:
		items := make([]map[string]any, 0, len(v))
		for _, item := range v {
			if m, ok := item.(map[string]any); ok {
				items = append(items, m)
			}
		}
		return items
	default:
		return nil
	}
}

func appendAreasByOrder(view *dashboardView, areas map[string]any) {
	order := []string{
		"admin.dashboard.main",
		"admin.dashboard.sidebar",
		"admin.dashboard.footer",
	}
	handled := map[string]bool{}
	for _, key := range order {
		if areaRaw, ok := areas[key].(map[string]any); ok {
			view.Areas = append(view.Areas, buildAreaView(areaRaw, key))
			handled[key] = true
		}
	}
	for code, areaRaw := range areas {
		if handled[code] {
			continue
		}
		if typed, ok := areaRaw.(map[string]any); ok {
			view.Areas = append(view.Areas, buildAreaView(typed, code))
		}
	}
}

func areaCodeOrSlot(areaRaw map[string]any) string {
	if code := stringOrDefault(areaRaw["code"], ""); code != "" {
		return code
	}
	slot := stringOrDefault(areaRaw["slot"], "")
	if slot == "main" {
		return "admin.dashboard.main"
	}
	if slot == "sidebar" {
		return "admin.dashboard.sidebar"
	}
	if slot == "footer" {
		return "admin.dashboard.footer"
	}
	return slot
}

func toDataMap(value any) map[string]any {
	switch v := value.(type) {
	case dashboard.WidgetData:
		return map[string]any(v)
	case map[string]any:
		return v
	default:
		if m, ok := value.(map[string]interface{}); ok {
			return map[string]any(m)
		}
		return nil
	}
}

func toWidgetMaps(raw any) []map[string]any {
	if raw == nil {
		return nil
	}
	if list, ok := raw.([]map[string]any); ok {
		return list
	}
	if list, ok := raw.([]any); ok {
		out := make([]map[string]any, 0, len(list))
		for _, item := range list {
			if widgetMap, ok := item.(map[string]any); ok {
				out = append(out, widgetMap)
			}
		}
		return out
	}
	return nil
}

func stringOrDefault(value any, fallback string) string {
	if s, ok := value.(string); ok && s != "" {
		return s
	}
	return fallback
}

func extractDefinition(widget map[string]any) string {
	if def, ok := widget["definition"].(string); ok && def != "" {
		return def
	}
	if tpl, ok := widget["template"].(string); ok && tpl != "" {
		parts := strings.Split(tpl, "/")
		last := parts[len(parts)-1]
		return strings.TrimSuffix(last, ".html")
	}
	return "widget"
}

func widgetTitle(def string) string {
	switch def {
	case "admin.widget.user_stats":
		return "User Statistics"
	case "admin.widget.recent_activity":
		return "Recent Activity"
	case "admin.widget.quick_actions":
		return "Quick Actions"
	case "admin.widget.system_status":
		return "System Status"
	case "admin.widget.analytics_funnel":
		return "Conversion Funnel"
	case "admin.widget.cohort_overview":
		return "Cohort Overview"
	case "admin.widget.alert_trends":
		return "Alert Trends"
	case "demo.widget.welcome":
		return "Welcome"
	default:
		return def
	}
}

func formatNumber(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case int:
		return formatInt64(int64(v))
	case int64:
		return formatInt64(v)
	case float64:
		if v >= 1000 {
			return fmt.Sprintf("%.1fk", v/1000)
		}
		return fmt.Sprintf("%.0f", v)
	case float32:
		return formatNumber(float64(v))
	default:
		return fmt.Sprint(value)
	}
}

func formatInt64(n int64) string {
	sign := ""
	if n < 0 {
		sign = "-"
		n = -n
	}
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return sign + s
	}
	var builder strings.Builder
	builder.WriteString(sign)
	prefix := len(s) % 3
	if prefix == 0 {
		prefix = 3
	}
	builder.WriteString(s[:prefix])
	for i := prefix; i < len(s); i += 3 {
		builder.WriteByte(',')
		builder.WriteString(s[i : i+3])
	}
	return builder.String()
}

func percentValue(value any) string {
	return fmt.Sprintf("%.1f%%", float64OrDefault(value, 0))
}

func statusClass(status string) string {
	switch strings.ToLower(status) {
	case "healthy", "ok":
		return "status--ok"
	case "degraded", "warning":
		return "status--warn"
	default:
		return "status--bad"
	}
}

func valueOr(primary, fallback any) any {
	if primary == nil {
		return fallback
	}
	if str, ok := primary.(string); ok {
		if strings.TrimSpace(str) == "" {
			return fallback
		}
	}
	return primary
}

func float64OrDefault(value any, fallback float64) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case float32:
		return float64(v)
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case string:
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func intPtr(value int) *int {
	v := value
	return &v
}

func addWidgetOrDie(ctx context.Context, svc *dashboard.Service, label string, req dashboard.AddWidgetRequest) {
	if err := svc.AddWidget(ctx, req); err != nil {
		log.Fatalf("add %s widget: %v", label, err)
	}
}

const dashboardTemplate = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>{{ .Title }}</title>
    <style>
      :root {
        color-scheme: light;
      }
      body {
        font-family: "Inter", -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif;
        background: #0f172a;
        margin: 0;
      }
      header {
        background: radial-gradient(circle at top, #1d4ed8, #0f172a);
        color: #f8fafc;
        padding: 2rem 3rem 2.5rem;
        box-shadow: inset 0 0 80px rgba(15, 23, 42, 0.35);
      }
      .header__content {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 2rem;
      }
      .eyebrow {
        text-transform: uppercase;
        font-size: 0.8rem;
        letter-spacing: 0.2em;
        margin: 0 0 0.5rem;
        opacity: 0.75;
      }
      header h1 {
        font-size: 2.4rem;
        margin: 0;
      }
      header p {
        margin: 0.35rem 0 0;
        color: #cbd5f5;
      }
      .header__meta {
        text-align: right;
        font-size: 0.9rem;
        color: #cbd5f5;
      }
      .header__meta .badge {
        display: inline-block;
        margin-top: 0.35rem;
        padding: 0.25rem 0.85rem;
        border-radius: 999px;
        background: rgba(15, 23, 42, 0.45);
        border: 1px solid rgba(148, 163, 184, 0.4);
      }
      #save-status {
        margin: 0;
        padding: 0.85rem 3rem;
        background: #e0f2fe;
        color: #0369a1;
        font-size: 0.9rem;
      }
      .dashboard {
        padding: 2rem 3rem 3rem;
        display: grid;
        gap: 1.5rem;
        grid-template-columns: 2fr 1fr;
        background: #f8fafc;
        min-height: calc(100vh - 220px);
      }
      .area-footer {
        grid-column: 1 / -1;
      }
      section.area {
        background: white;
        border-radius: 16px;
        box-shadow: 0 25px 80px rgba(15, 23, 42, 0.08);
        padding: 1.5rem;
        border: 1px solid #e2e8f0;
      }
      section.area h2 {
        margin-top: 0;
        font-size: 0.95rem;
        text-transform: uppercase;
        letter-spacing: 0.08em;
        color: #94a3b8;
      }
      .widget {
        border: 1px solid #e2e8f0;
        border-radius: 12px;
        padding: 1.1rem;
        margin-top: 1rem;
        background: #fcfdff;
        cursor: grab;
        transition: transform 120ms ease, box-shadow 120ms ease;
      }
      .widget.dragging {
        opacity: 0.7;
        box-shadow: 0 15px 35px rgba(15, 23, 42, 0.15);
      }
      .widget.is-hidden {
        opacity: 0.35;
      }
      .widget h3 {
        margin: 0 0 0.5rem;
        font-size: 1.05rem;
      }
      .metrics {
        display: flex;
        gap: 1rem;
        flex-wrap: wrap;
      }
      .metric {
        flex: 1;
        min-width: 140px;
        background: #0f172a;
        color: #f8fafc;
        border-radius: 10px;
        padding: 0.85rem 1rem;
      }
      .metric small {
        text-transform: uppercase;
        letter-spacing: 0.12em;
        font-size: 0.7rem;
        color: #cbd5f5;
      }
      .metric span {
        display: block;
        font-size: 1.4rem;
        font-weight: 600;
        margin-top: 0.3rem;
      }
      .activity {
        list-style: none;
        padding: 0;
        margin: 0;
      }
      .activity li {
        padding: 0.65rem 0;
        border-bottom: 1px solid #e2e8f0;
      }
      .activity li:last-child {
        border-bottom: none;
      }
      .activity small {
        display: block;
        color: #94a3b8;
      }
      .actions {
        display: flex;
        flex-wrap: wrap;
        gap: 0.75rem;
      }
      .actions a {
        text-decoration: none;
        background: #1d4ed8;
        color: white;
        padding: 0.8rem 1rem;
        border-radius: 10px;
        font-size: 0.9rem;
        flex: 1;
        min-width: 180px;
        box-shadow: 0 6px 18px rgba(29, 78, 216, 0.3);
      }
      .actions a small {
        display: block;
        margin-top: 0.25rem;
        opacity: 0.85;
      }
      .status {
        list-style: none;
        padding: 0;
        margin: 0;
      }
      .status li {
        display: flex;
        justify-content: space-between;
        padding: 0.4rem 0;
        align-items: center;
      }
      .status-badge {
        padding: 0.15rem 0.65rem;
        border-radius: 999px;
        font-size: 0.75rem;
        text-transform: capitalize;
      }
      .status--ok {
        background: #dcfce7;
        color: #166534;
      }
      .status--warn {
        background: #fef3c7;
        color: #92400e;
      }
      .status--bad {
        background: #fee2e2;
        color: #b91c1c;
      }
      .widget__toolbar {
        display: flex;
        justify-content: flex-end;
        gap: 0.5rem;
        margin-bottom: 0.5rem;
      }
      .widget__toolbar button {
        border: none;
        background: #e2e8f0;
        border-radius: 6px;
        padding: 0.2rem 0.6rem;
        cursor: pointer;
        font-size: 0.75rem;
      }
      .funnel-callout {
        display: flex;
        align-items: center;
        justify-content: space-between;
        border: 1px dashed #cbd5f5;
        padding: 0.6rem 0.8rem;
        border-radius: 10px;
        background: #eef2ff;
        margin-bottom: 0.6rem;
      }
      .funnel-callout strong {
        font-size: 1.3rem;
      }
      .goal-pill {
        background: white;
        border-radius: 999px;
        padding: 0.2rem 0.9rem;
        font-size: 0.85rem;
        color: #4338ca;
        border: 1px solid rgba(67, 56, 202, 0.2);
      }
      .widget ol {
        padding-left: 1.2rem;
        color: #475569;
      }
    </style>
  </head>
  <body>
    <header>
      <div class="header__content">
        <div>
          <p class="eyebrow">Northwind Control Center</p>
          <h1>{{ .Title }}</h1>
          {{ if .Description }}
            <p>{{ .Description }}</p>
          {{ else }}
            <p>Live health for revenue, adoption, and operations.</p>
          {{ end }}
        </div>
        <div class="header__meta">
          <span>Last updated {{ .LastUpdated }}</span>
          <div class="badge">SLO · 99.95%</div>
        </div>
      </div>
    </header>
    <p id="save-status">Drag widgets between areas or tap "Toggle Hide" to personalize your workspace. Preferences save immediately.</p>
    <div class="dashboard" id="dashboard">
      {{ range $idx, $area := .Areas }}
        {{ if eq $area.Code "admin.dashboard.main" }}
          <section class="area area-main" data-area="{{ $area.Code }}">
            <h2>Main</h2>
            {{ template "widgets" $area.Widgets }}
          </section>
        {{ else if eq $area.Code "admin.dashboard.sidebar" }}
          <section class="area area-sidebar" data-area="{{ $area.Code }}">
            <h2>Sidebar</h2>
            {{ template "widgets" $area.Widgets }}
          </section>
        {{ else if eq $area.Code "admin.dashboard.footer" }}
          <section class="area area-footer" data-area="{{ $area.Code }}">
            <h2>Operations</h2>
            {{ template "widgets" $area.Widgets }}
          </section>
        {{ end }}
      {{ end }}
    </div>
    <script>
      (function () {
        const areas = document.querySelectorAll("[data-area]");
        const status = document.getElementById("save-status");
        let dragged = null;

        document.querySelectorAll(".widget").forEach(widget => {
          widget.draggable = true;
          widget.addEventListener("dragstart", () => {
            dragged = widget;
            widget.classList.add("dragging");
          });
          widget.addEventListener("dragend", () => {
            widget.classList.remove("dragging");
          });
        });

        areas.forEach(area => {
          area.addEventListener("dragover", event => {
            event.preventDefault();
            const after = getDragAfterElement(area, event.clientY);
            if (!dragged) return;
            if (after == null) {
              area.appendChild(dragged);
            } else if (after !== dragged) {
              area.insertBefore(dragged, after);
            }
          });
          area.addEventListener("drop", event => {
            event.preventDefault();
            saveLayout();
          });
        });

        document.querySelectorAll(".hide-widget").forEach(btn => {
          btn.addEventListener("click", () => {
            const widget = btn.closest(".widget");
            widget.classList.toggle("is-hidden");
            saveLayout();
          });
        });

        function getDragAfterElement(container, y) {
          const elements = [...container.querySelectorAll(".widget:not(.dragging)")];
          return elements.reduce((closest, child) => {
            const box = child.getBoundingClientRect();
            const offset = y - box.top - box.height / 2;
            if (offset < 0 && offset > closest.offset) {
              return { offset: offset, element: child };
            } else {
              return closest;
            }
          }, { offset: Number.NEGATIVE_INFINITY }).element;
        }

        let saveTimer;
        function saveLayout() {
          const payload = { area_order: {}, hidden_widget_ids: [] };
          document.querySelectorAll("[data-area]").forEach(area => {
            const code = area.getAttribute("data-area");
            payload.area_order[code] = Array.from(area.querySelectorAll(".widget:not(.is-hidden)")).map(w => w.getAttribute("data-widget"));
          });
          document.querySelectorAll(".widget.is-hidden").forEach(widget => {
            payload.hidden_widget_ids.push(widget.getAttribute("data-widget"));
          });

          clearTimeout(saveTimer);
          status.textContent = "Saving layout…";
          saveTimer = setTimeout(() => {
            fetch("/admin/dashboard/preferences", {
              method: "POST",
              headers: { "Content-Type": "application/json" },
              body: JSON.stringify(payload),
            })
              .then(res => {
                if (!res.ok) throw new Error("Failed request");
                status.textContent = "Layout saved";
              })
              .catch(() => {
                status.textContent = "Save failed. Check console.";
              });
          }, 200);
        }
      })();
    </script>
  </body>
</html>

{{ define "widgets" }}
  {{ range . }}
    <article class="widget" data-widget="{{ .ID }}">
      <div class="widget__toolbar">
        <button type="button" class="hide-widget">Toggle Hide</button>
      </div>
      <h3>{{ widgetTitle .Definition }}</h3>
      {{ if isType .Definition "admin.widget.user_stats" }}
        <div class="metrics">
          {{ range $key, $value := index .Data "values" }}
            <div class="metric">
              <small>{{ $key }}</small>
              <span>{{ formatNumber $value }}</span>
            </div>
          {{ end }}
        </div>
      {{ else if isType .Definition "admin.widget.recent_activity" }}
        <ul class="activity">
          {{ range $item := index .Data "items" }}
            <li>
              <strong>{{ index $item "user" }}</strong> {{ index $item "action" }} · {{ index $item "ago" }}
              {{ if index $item "detail" }}<small>{{ index $item "detail" }}</small>{{ end }}
            </li>
          {{ end }}
        </ul>
      {{ else if isType .Definition "admin.widget.quick_actions" }}
        <div class="actions">
          {{ range $action := index .Data "actions" }}
            <a href="{{ index $action "route" }}">
              <strong>{{ index $action "label" }}</strong>
              {{ if index $action "description" }}<small>{{ index $action "description" }}</small>{{ end }}
            </a>
          {{ end }}
        </div>
      {{ else if isType .Definition "admin.widget.system_status" }}
        <ul class="status">
          {{ range $check := index .Data "checks" }}
            <li>
              <span>{{ index $check "name" }}</span>
              <strong class="status-badge {{ statusClass (index $check "status") }}">{{ index $check "status" }}</strong>
            </li>
          {{ end }}
        </ul>
      {{ else if isType .Definition "admin.widget.analytics_funnel" }}
        {{ $conversion := percent (index .Data "conversion_rate") }}
        {{ $goal := percent (valueOr (index .Data "goal") (index .Config "goal")) }}
        <div class="funnel-callout">
          <div>
            <strong>{{ $conversion }}</strong>
            <span>conversion</span>
          </div>
          <span class="goal-pill">Goal {{ $goal }}</span>
        </div>
        <ul class="activity">
          {{ range $step := index .Data "steps" }}
            <li>
              <strong>{{ index $step "label" }}</strong>
              {{ formatNumber (index $step "value") }} · {{ printf "%.1f%%" (index $step "percent") }} of entry
            </li>
          {{ end }}
        </ul>
      {{ else if isType .Definition "admin.widget.cohort_overview" }}
        <ul class="activity">
          {{ range $row := index .Data "rows" }}
            <li>
              <strong>{{ index $row "label" }}</strong> — {{ index $row "size" }} signups
              <div>
                {{ range $idx, $rate := index $row "retention" }}
                  <span style="margin-right:0.75rem;">P{{ add $idx 1 }} {{ printf "%.0f%%" $rate }}</span>
                {{ end }}
              </div>
            </li>
          {{ end }}
        </ul>
      {{ else if isType .Definition "admin.widget.alert_trends" }}
        <ul class="activity">
          {{ range $bucket := index .Data "series" }}
            <li>
              <strong>{{ index $bucket "day" }}</strong>
              {{ range $row := index $bucket "counts" }}
                <span style="margin-left:0.5rem;">{{ index $row "severity" }}: {{ index $row "count" }}</span>
              {{ end }}
            </li>
          {{ end }}
        </ul>
      {{ else if isType .Definition "demo.widget.welcome" }}
        <p><strong>{{ index .Data "headline" }}</strong></p>
        <p>{{ index .Data "message" }}</p>
      {{ else }}
        <pre>{{ printf "%+v" .Data }}</pre>
      {{ end }}
    </article>
  {{ else }}
    <p>No widgets configured.</p>
  {{ end }}
{{ end }}
`
