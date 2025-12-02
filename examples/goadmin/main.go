package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/compress"
	router "github.com/goliatone/go-router"

	"github.com/goliatone/go-dashboard/components/dashboard"
	"github.com/goliatone/go-dashboard/components/dashboard/commands"
	"github.com/goliatone/go-dashboard/components/dashboard/gorouter"
	"github.com/goliatone/go-dashboard/components/dashboard/httpapi"
	"github.com/goliatone/go-dashboard/pkg/analytics"
	dashboardpkg "github.com/goliatone/go-dashboard/pkg/dashboard"
	"github.com/goliatone/go-dashboard/pkg/goadmin"
)

const sampleViewerID = "admin@example.com"

type demoTranslationService struct {
	entries map[string]map[string]string
}

func newDemoTranslationService() *demoTranslationService {
	return &demoTranslationService{
		entries: map[string]map[string]string{
			"dashboard.page.title":                   {"es": "Panel de control"},
			"dashboard.page.description":             {"es": "Resumen administrativo"},
			"dashboard.widget.user_stats.data_title": {"es": "Usuarios"},
			"dashboard.widget.quick_actions.invite_user": {
				"es": "Invitar usuario",
			},
			"dashboard.widget.quick_actions.create_page": {
				"es": "Crear página",
			},
			"dashboard.widget.system_status.database": {"es": "Base de datos"},
			"dashboard.widget.system_status.cache":    {"es": "Caché"},
			"dashboard.widget.system_status.worker":   {"es": "Trabajador"},
			"dashboard.widget.sales_chart.title":      {"es": "Ventas"},
			"dashboard.widget.alert_trends.title":     {"es": "Tendencias de alertas"},
			"dashboard.widget.alert_trends.service_all": {
				"es": "Todos los servicios",
			},
			"dashboard.widget.alert_trends.lookback_prefix": {"es": "Últimos"},
			"dashboard.widget.alert_trends.lookback_suffix": {"es": "días"},
			"demo.widget.user_stats.title":                  {"es": "Salud de la cuenta"},
			"demo.activity.published_pricing":               {"es": "publicó la actualización de precios de primavera"},
			"demo.activity.invited_seats":                   {"es": "invitó 24 licencias empresariales"},
			"demo.activity.resolved_invoices":               {"es": "resolvió 14 facturas pendientes"},
			"demo.activity.shipped_theme":                   {"es": "envió un cambio de tema"},
			"demo.activity.closed_incident":                 {"es": "cerró el incidente #782"},
			"demo.quick_action.invite.label":                {"es": "Invitar equipo de ventas"},
			"demo.quick_action.invite.description":          {"es": "Importa cuentas SDR"},
			"demo.quick_action.plan.label":                  {"es": "Simulador de planes"},
			"demo.quick_action.plan.description":            {"es": "Estima el impacto ARR"},
			"demo.quick_action.status.label":                {"es": "Agregar actualización de estado"},
			"demo.quick_action.status.description":          {"es": "Publicar en StatusPage"},
			"demo.quick_action.automation.label":            {"es": "Crear automatización"},
			"demo.quick_action.automation.description":      {"es": "Conecta alertas a Zendesk"},
			"demo.status.checkout":                          {"es": "API de cobro"},
			"demo.status.billing":                           {"es": "Procesos de facturación"},
			"demo.status.notifications":                     {"es": "Notificaciones"},
			"demo.status.sync":                              {"es": "Sincronización en segundo plano"},
			"demo.time.just_now":                            {"es": "justo ahora"},
			"demo.time.minutes_suffix":                      {"es": "m"},
			"demo.time.hours_suffix":                        {"es": "h"},
			"demo.time.days_suffix":                         {"es": "d"},
			"demo.widget.welcome.headline":                  {"es": "Hola 👋"},
			"demo.widget.welcome.message":                   {"es": "Las operaciones se ven estables. Usa este espacio para notas o recordatorios."},
		},
	}
}

func (d *demoTranslationService) Translate(_ context.Context, key, locale string, _ map[string]any) (string, error) {
	if d == nil {
		return "", nil
	}
	locale = strings.ToLower(strings.TrimSpace(locale))
	values, ok := d.entries[key]
	if !ok {
		return "", nil
	}
	if locale == "" {
		locale = "en"
	}
	if value, ok := values[locale]; ok && value != "" {
		return value, nil
	}
	if idx := strings.Index(locale, "-"); idx > 0 {
		if value, ok := values[locale[:idx]]; ok && value != "" {
			return value, nil
		}
	}
	if value, ok := values["default"]; ok {
		return value, nil
	}
	return "", nil
}

var _ dashboard.TranslationService = (*demoTranslationService)(nil)

type staticThemeProvider struct {
	selection *dashboard.ThemeSelection
}

func (p *staticThemeProvider) SelectTheme(context.Context, dashboard.ThemeSelector) (*dashboard.ThemeSelection, error) {
	return p.selection, nil
}

func demoThemeSelection() *dashboard.ThemeSelection {
	return &dashboard.ThemeSelection{
		Name:    "ops-night",
		Variant: "dark",
		Tokens: map[string]string{
			"--dashboard-surface":    "#0b1220",
			"--dashboard-card":       "#111827",
			"--dashboard-foreground": "#e2e8f0",
			"--dashboard-muted":      "#94a3b8",
			"--dashboard-accent":     "#22d3ee",
		},
		Assets: dashboard.ThemeAssets{
			Values: map[string]string{
				"logo": "/assets/logo.svg",
			},
			Prefix: "https://cdn.goadmin.dev",
		},
		Templates: map[string]string{
			"dashboard.layout": "dashboard.html",
		},
	}
}

func main() {
	ctx := context.Background()

	translator := newDemoTranslationService()
	themeSelection := demoThemeSelection()
	themeProvider := &staticThemeProvider{selection: themeSelection}
	themeSelector := func(context.Context, dashboard.ViewerContext) dashboard.ThemeSelector {
		return dashboard.ThemeSelector{Name: themeSelection.Name, Variant: themeSelection.Variant}
	}
	service, registry, store := setupDemoDashboard(ctx, translator, themeProvider, themeSelector)

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

	server := router.NewFiberAdapter(func(app *fiber.App) *fiber.App {
		app = router.DefaultFiberOptions(app)
		if os.Getenv("GO_DASHBOARD_ENABLE_GZIP") != "" {
			app.Use(compress.New())
		}
		return app
	})
	appRouter := server.Router()
	if err := gorouter.Register(gorouter.Config[*fiber.App]{
		Router:     appRouter,
		Controller: controller,
		API:        executor,
		Broadcast:  hook,
		ViewerResolver: func(ctx router.Context) dashboard.ViewerContext {
			viewer := dashboard.ViewerContext{UserID: sampleViewerID, Roles: []string{"admin"}, Locale: "en"}
			if locale := ctx.Query("locale"); locale != "" {
				viewer.Locale = locale
			} else if header := ctx.Header("Accept-Language"); header != "" {
				viewer.Locale = strings.ToLower(strings.TrimSpace(strings.Split(header, ",")[0]))
			}
			return viewer
		},
	}); err != nil {
		log.Fatalf("register routes: %v", err)
	}

	admin, err := goadmin.New(goadmin.Config{
		EnableDashboard: true,
		Service: dashboardpkg.NewService(dashboard.Options{
			WidgetStore:   store,
			Providers:     registry,
			Translation:   translator,
			ThemeProvider: themeProvider,
			ThemeSelector: themeSelector,
		}),
		MenuBuilder: &loggingMenuBuilder{},
	})
	if err != nil {
		log.Fatalf("goadmin init: %v", err)
	}
	if err := admin.Bootstrap(ctx); err != nil {
		log.Fatalf("bootstrap: %v", err)
	}

	log.Printf("dashboard routes ready: http://localhost:9876/admin/dashboard")
	log.Printf("Try locale switching via http://localhost:9876/admin/dashboard?locale=es")
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

func (s *memoryWidgetStore) GetInstance(ctx context.Context, instanceID string) (dashboard.WidgetInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.instances[instanceID]
	if !ok {
		return dashboard.WidgetInstance{}, fmt.Errorf("instance %s not found", instanceID)
	}
	return inst, nil
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

func (s *memoryWidgetStore) UpdateInstance(ctx context.Context, input dashboard.UpdateWidgetInstanceInput) (dashboard.WidgetInstance, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	inst, ok := s.instances[input.InstanceID]
	if !ok {
		return dashboard.WidgetInstance{}, fmt.Errorf("instance %s not found", input.InstanceID)
	}
	if input.Configuration != nil {
		inst.Configuration = input.Configuration
	}
	if input.Metadata != nil {
		inst.Metadata = input.Metadata
	}
	s.instances[input.InstanceID] = inst
	return inst, nil
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
		"isType":        func(definition, code string) bool { return definition == code },
		"widgetTitle":   widgetTitle,
		"widgetHeading": widgetHeading,
		"widgetSpan":    widgetSpanMeta,
		"add":           func(a, b int) int { return a + b },
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
		"chartWidget": isChartDefinition,
		"chartMarkup": chartMarkup,
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
				"title": translateOrDefault(ctx, meta.Translator, meta.Viewer.Locale, "demo.widget.user_stats.title", "Account Health"),
				"values": map[string]any{
					"total":  16804,
					"active": 12034,
					"new":    482,
				},
			}, nil
		}),
		"admin.widget.recent_activity": dashboard.ProviderFunc(func(ctx context.Context, meta dashboard.WidgetContext) (dashboard.WidgetData, error) {
			return dashboard.WidgetData{
				"items": demoActivityFeed(ctx, time.Now(), meta.Translator, meta.Viewer.Locale),
			}, nil
		}),
		"admin.widget.quick_actions": dashboard.ProviderFunc(func(ctx context.Context, meta dashboard.WidgetContext) (dashboard.WidgetData, error) {
			return dashboard.WidgetData{
				"actions": demoQuickActions(ctx, meta.Translator, meta.Viewer.Locale),
			}, nil
		}),
		"admin.widget.system_status": dashboard.ProviderFunc(func(ctx context.Context, meta dashboard.WidgetContext) (dashboard.WidgetData, error) {
			return dashboard.WidgetData{
				"checks": demoStatusChecks(ctx, meta.Translator, meta.Viewer.Locale),
			}, nil
		}),
	}
	for code, provider := range providers {
		if err := reg.RegisterProvider(code, provider); err != nil {
			return err
		}
	}
	cdnHost := dashboard.DefaultEChartsAssetsHost()
	salesRenderer := dashboard.NewEChartsProvider(
		"line",
		dashboard.WithChartCache(dashboard.NewChartCache(10*time.Minute)),
		dashboard.WithChartAssetsHost(cdnHost),
	)
	if err := reg.RegisterProvider("admin.widget.sales_chart", dashboard.NewSalesChartProvider(demoSalesRepository{}, salesRenderer)); err != nil {
		return err
	}
	return nil
}

func setupDemoDashboard(ctx context.Context, translator dashboard.TranslationService, themeProvider dashboard.ThemeProvider, themeSelector dashboard.ThemeSelectorFunc) (*dashboard.Service, *dashboard.Registry, *memoryWidgetStore) {
	store := newMemoryWidgetStore()
	registry := dashboard.NewRegistry()

	customDefinition := dashboard.WidgetDefinition{
		Code:        "demo.widget.welcome",
		Name:        "Welcome Banner",
		Description: "Greets the signed-in administrator.",
		Category:    "demo",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"message": map[string]any{
					"oneOf": []map[string]any{
						{"type": "string"},
						{
							"type":                 "object",
							"additionalProperties": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}
	_, _ = store.EnsureDefinition(ctx, customDefinition)
	_ = registry.RegisterDefinition(customDefinition)
	_ = registry.RegisterProvider(customDefinition.Code, dashboard.ProviderFunc(func(ctx context.Context, meta dashboard.WidgetContext) (dashboard.WidgetData, error) {
		configMessage := meta.Instance.Configuration["message"]
		messageMap := toStringMap(configMessage)
		fallback := translateOrDefault(ctx, meta.Translator, meta.Viewer.Locale, "demo.widget.welcome.message", "Operations look steady. Use this space for runbook snippets or rotating notices.")
		defaultMessage := fallback
		if raw, ok := configMessage.(string); ok && raw != "" {
			defaultMessage = raw
		}
		message := defaultMessage
		if len(messageMap) > 0 {
			if defaultMessage == "" {
				defaultMessage = fallback
			}
			message = dashboard.ResolveLocalizedValue(messageMap, meta.Viewer.Locale, defaultMessage)
		} else if message == "" {
			message = fallback
		}
		return dashboard.WidgetData{
			"headline": translateOrDefault(ctx, meta.Translator, meta.Viewer.Locale, "demo.widget.welcome.headline", fmt.Sprintf("Hey %s 👋", meta.Viewer.UserID)),
			"message":  message,
		}, nil
	}))

	service := dashboard.NewService(dashboard.Options{
		WidgetStore:   store,
		Providers:     registry,
		Translation:   translator,
		ThemeProvider: themeProvider,
		ThemeSelector: themeSelector,
	})
	defaultViewer := dashboard.ViewerContext{UserID: sampleViewerID, Locale: "en"}

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
			"message": map[string]any{
				"en": "Operations look steady. Use this space for runbook snippets or rotating notices.",
				"es": "Las operaciones se ven estables. Usa este espacio para notas o recordatorios.",
			},
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
	addWidgetOrDie(ctx, service, "monthly sales chart", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.bar_chart",
		AreaCode:     "admin.dashboard.main",
		Position:     intPtr(4),
		Configuration: map[string]any{
			"title":    "Monthly Sales",
			"subtitle": "Revenue by region",
			"x_axis":   []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
			"series": []map[string]any{
				{"name": "North America", "data": []float64{120, 132, 101, 134, 90, 230}},
				{"name": "Europe", "data": []float64{220, 182, 191, 234, 290, 330}},
				{"name": "Asia Pacific", "data": []float64{150, 232, 201, 154, 190, 330}},
			},
		},
	})
	addWidgetOrDie(ctx, service, "user growth chart", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.line_chart",
		AreaCode:     "admin.dashboard.main",
		Position:     intPtr(5),
		Configuration: map[string]any{
			"title":  "Weekly User Growth",
			"x_axis": []string{"Week 1", "Week 2", "Week 3", "Week 4"},
			"series": []map[string]any{
				{"name": "Active Users", "data": []float64{1200, 1320, 1450, 1580}},
				{"name": "New Signups", "data": []float64{150, 180, 220, 250}},
			},
			"footer_note": "Data refreshed nightly · demo dataset",
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
	addWidgetOrDie(ctx, service, "traffic sources chart", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.pie_chart",
		AreaCode:     "admin.dashboard.sidebar",
		Position:     intPtr(2),
		Configuration: map[string]any{
			"title": "Traffic Sources",
			"series": []map[string]any{
				{
					"name": "Sources",
					"data": []map[string]any{
						{"name": "Direct", "value": 335},
						{"name": "Organic Search", "value": 310},
						{"name": "Social Media", "value": 234},
						{"name": "Email", "value": 135},
						{"name": "Referral", "value": 148},
					},
				},
			},
		},
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
	addWidgetOrDie(ctx, service, "scatter correlation", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.scatter_chart",
		AreaCode:     "admin.dashboard.main",
		Position:     intPtr(6),
		Configuration: map[string]any{
			"title": "Churn vs NPS",
			"series": []map[string]any{
				{
					"name": "Segments",
					"data": []map[string]any{
						{"name": "Enterprise", "x": 3.2, "y": 96},
						{"name": "Mid-market", "x": 5.1, "y": 88},
						{"name": "SMB", "x": 7.4, "y": 72},
					},
				},
			},
		},
	})
	addWidgetOrDie(ctx, service, "uptime gauge", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.gauge_chart",
		AreaCode:     "admin.dashboard.sidebar",
		Position:     intPtr(3),
		Configuration: map[string]any{
			"title": "Platform SLA",
			"series": []map[string]any{
				{"name": "SLA", "data": []float64{99.2}},
			},
			"theme": "wonderland",
		},
	})
	addWidgetOrDie(ctx, service, "sales pulse", dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.sales_chart",
		AreaCode:     "admin.dashboard.footer",
		Position:     intPtr(2),
		Configuration: map[string]any{
			"period":            "30d",
			"metric":            "revenue",
			"comparison_metric": "orders",
			"segment":           "enterprise",
			"dynamic":           true,
			"refresh_endpoint":  "/admin/api/sales/revenue",
		},
	})
	seedDefaultLayout(ctx, service, defaultViewer)
	return service, registry, store
}

func demoActivityFeed(ctx context.Context, now time.Time, translator dashboard.TranslationService, locale string) []map[string]any {
	entries := []struct {
		User          string
		ActionKey     string
		ActionDefault string
		Context       string
		When          time.Duration
	}{
		{"Candice Reed", "demo.activity.published_pricing", "published the spring pricing update", "Billing · Plan v3 rollout", 5 * time.Minute},
		{"Noah Patel", "demo.activity.invited_seats", "invited 24 enterprise seats", "Acme Industrial — Enterprise", 22 * time.Minute},
		{"Marcos Valle", "demo.activity.resolved_invoices", "resolved 14 aging invoices", "Finance · Treasury automation", 49 * time.Minute},
		{"Sara Ndlovu", "demo.activity.shipped_theme", "shipped a dashboard theme change", "Design System · Canary env", 2 * time.Hour},
		{"Elena Ibarra", "demo.activity.closed_incident", "closed incident #782", "Checkout API · On-call", 6 * time.Hour},
	}
	items := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		items = append(items, map[string]any{
			"user":   entry.User,
			"action": translateOrDefault(ctx, translator, locale, entry.ActionKey, entry.ActionDefault),
			"ago":    formatAgo(ctx, translator, now.Add(-entry.When), locale),
			"detail": entry.Context,
		})
	}
	return items
}

func demoQuickActions(ctx context.Context, translator dashboard.TranslationService, locale string) []map[string]any {
	type action struct {
		LabelKey      string
		LabelFallback string
		DescKey       string
		DescFallback  string
		Route         string
		Icon          string
	}
	actions := []action{
		{"demo.quick_action.invite.label", "Invite sales team", "demo.quick_action.invite.description", "Bulk import SDR seats", "/admin/users/invite", "users"},
		{"demo.quick_action.plan.label", "Plan change simulator", "demo.quick_action.plan.description", "Estimate ARR impact", "/admin/billing/simulator", "activity"},
		{"demo.quick_action.status.label", "Add status update", "demo.quick_action.status.description", "Publish to StatusPage", "/admin/incidents/new", "alert-circle"},
		{"demo.quick_action.automation.label", "Create automation", "demo.quick_action.automation.description", "Connect alerts to Zendesk", "/admin/workflows/new", "zap"},
	}
	items := make([]map[string]any, 0, len(actions))
	for _, act := range actions {
		items = append(items, map[string]any{
			"label":       translateOrDefault(ctx, translator, locale, act.LabelKey, act.LabelFallback),
			"description": translateOrDefault(ctx, translator, locale, act.DescKey, act.DescFallback),
			"route":       act.Route,
			"icon":        act.Icon,
		})
	}
	return items
}

func demoStatusChecks(ctx context.Context, translator dashboard.TranslationService, locale string) []map[string]any {
	checks := []struct {
		Key      string
		Fallback string
		Status   string
	}{
		{"demo.status.checkout", "Checkout API", "healthy"},
		{"demo.status.billing", "Billing jobs", "degraded"},
		{"demo.status.notifications", "Notifications", "healthy"},
		{"demo.status.sync", "Background sync", "investigating"},
	}
	items := make([]map[string]any, 0, len(checks))
	for _, check := range checks {
		items = append(items, map[string]any{
			"name":   translateOrDefault(ctx, translator, locale, check.Key, check.Fallback),
			"status": check.Status,
		})
	}
	return items
}

func seedDefaultLayout(ctx context.Context, svc *dashboard.Service, viewer dashboard.ViewerContext) {
	layout, err := svc.ConfigureLayout(ctx, viewer)
	if err != nil {
		return
	}
	mainArea := layout.Areas["admin.dashboard.main"]
	funnelID := widgetIDByDefinition(mainArea, "admin.widget.analytics_funnel")
	cohortID := widgetIDByDefinition(mainArea, "admin.widget.cohort_overview")
	if funnelID == "" || cohortID == "" {
		return
	}
	overrides := dashboard.LayoutOverrides{
		AreaRows: map[string][]dashboard.LayoutRow{
			"admin.dashboard.main": {
				{Widgets: []dashboard.WidgetSlot{
					{ID: funnelID, Width: 6},
					{ID: cohortID, Width: 6},
				}},
			},
		},
	}
	if err := svc.SavePreferences(ctx, viewer, overrides); err != nil {
		log.Printf("seed layout overrides: %v", err)
	}
}

func widgetIDByDefinition(widgets []dashboard.WidgetInstance, definition string) string {
	for _, w := range widgets {
		if w.DefinitionID == definition {
			return w.ID
		}
	}
	return ""
}

func formatAgo(ctx context.Context, translator dashboard.TranslationService, ts time.Time, locale string) string {
	diff := time.Since(ts)
	if diff < time.Minute {
		return translateOrDefault(ctx, translator, locale, "demo.time.just_now", "just now")
	}
	if diff < time.Hour {
		return fmt.Sprintf("%d%s", int(diff.Minutes()), translateOrDefault(ctx, translator, locale, "demo.time.minutes_suffix", "m"))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%d%s", int(diff.Hours()), translateOrDefault(ctx, translator, locale, "demo.time.hours_suffix", "h"))
	}
	days := int(diff.Hours()) / 24
	return fmt.Sprintf("%d%s", days, translateOrDefault(ctx, translator, locale, "demo.time.days_suffix", "d"))
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
	Metadata   map[string]any
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
		widget.Metadata = toAnyMap(widgetMap["metadata"])
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

func toAnyMap(value any) map[string]any {
	if value == nil {
		return nil
	}
	if typed, ok := value.(map[string]any); ok {
		return typed
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

func widgetHeading(widget widgetView) string {
	if widget.Data != nil {
		if title := stringOrDefault(widget.Data["title"], ""); title != "" {
			return title
		}
	}
	if widget.Config != nil {
		if title := stringOrDefault(widget.Config["title"], ""); title != "" {
			return title
		}
	}
	return widgetTitle(widget.Definition)
}

func chartMarkup(data map[string]any) template.HTML {
	if data == nil {
		return ""
	}
	if html, ok := data["chart_html"].(string); ok && html != "" {
		return template.HTML(html)
	}
	return ""
}

func isChartDefinition(def string) bool {
	switch def {
	case "admin.widget.bar_chart",
		"admin.widget.line_chart",
		"admin.widget.pie_chart",
		"admin.widget.scatter_chart",
		"admin.widget.gauge_chart",
		"admin.widget.sales_chart":
		return true
	default:
		return false
	}
}

func widgetSpanMeta(meta map[string]any) int {
	if meta == nil {
		return 12
	}
	layout, ok := meta["layout"].(map[string]any)
	if !ok {
		return 12
	}
	if width, ok := layout["width"]; ok {
		if val, ok := intFromAny(width); ok {
			return clampSpan(val)
		}
	}
	if cols, ok := layout["columns"]; ok {
		if val, ok := intFromAny(cols); ok {
			return clampSpan(val)
		}
	}
	return 12
}

func intFromAny(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case int8:
		return int(v), true
	case int16:
		return int(v), true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case uint:
		return int(v), true
	case uint8:
		return int(v), true
	case uint16:
		return int(v), true
	case uint32:
		return int(v), true
	case uint64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case string:
		if parsed, err := strconv.Atoi(v); err == nil {
			return parsed, true
		}
	}
	return 0, false
}

func clampSpan(width int) int {
	if width < 1 {
		return 1
	}
	if width > 12 {
		return 12
	}
	return width
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

func translateOrDefault(ctx context.Context, svc dashboard.TranslationService, locale, key, fallback string) string {
	if svc == nil {
		return fallback
	}
	value, err := svc.Translate(ctx, key, locale, nil)
	if err != nil || value == "" {
		return fallback
	}
	return value
}

func toStringMap(input any) map[string]string {
	switch value := input.(type) {
	case map[string]string:
		if len(value) == 0 {
			return nil
		}
		return value
	case map[string]any:
		out := make(map[string]string, len(value))
		for k, v := range value {
			if str, ok := v.(string); ok && str != "" {
				out[k] = str
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	default:
		return nil
	}
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

type demoSalesRepository struct{}

func (demoSalesRepository) FetchSalesSeries(_ context.Context, query dashboard.SalesSeriesQuery) ([]dashboard.SalesSeriesPoint, error) {
	values := []float64{11800, 12640, 13320, 14250, 15730, 16980}
	points := make([]dashboard.SalesSeriesPoint, len(values))
	now := time.Now().UTC()
	for i, value := range values {
		points[i] = dashboard.SalesSeriesPoint{
			Timestamp: now.AddDate(0, 0, -7*(len(values)-i)),
			Value:     value,
		}
	}
	return points, nil
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
      .widgets-grid {
        display: grid;
        grid-template-columns: repeat(12, minmax(0, 1fr));
        gap: 1rem;
        margin-top: 1rem;
      }
      .widgets-grid .widget {
        margin-top: 0;
        grid-column: span var(--span, 12);
      }
      .area-empty {
        grid-column: 1 / -1;
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
      .widget-chart {
        overflow: hidden;
        min-height: 320px;
      }
      .widget-chart > div {
        width: 100% !important;
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
      .widget__toolbar button[disabled] {
        opacity: 0.45;
        cursor: not-allowed;
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
          <section class="area area-main" data-area-code="{{ $area.Code }}">
            <h2>Main</h2>
            {{ template "widgets" $area }}
          </section>
        {{ else if eq $area.Code "admin.dashboard.sidebar" }}
          <section class="area area-sidebar" data-area-code="{{ $area.Code }}">
            <h2>Sidebar</h2>
            {{ template "widgets" $area }}
          </section>
        {{ else if eq $area.Code "admin.dashboard.footer" }}
          <section class="area area-footer" data-area-code="{{ $area.Code }}">
            <h2>Operations</h2>
            {{ template "widgets" $area }}
          </section>
        {{ end }}
      {{ end }}
    </div>
    <script>
      (function () {
        const grids = document.querySelectorAll("[data-area-grid]");
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

        grids.forEach(area => {
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

        document.querySelectorAll(".resize-widget").forEach(btn => {
          const widget = btn.closest(".widget");
          if (btn.disabled || !widget || widget.dataset.resizable !== "true") {
            return;
          }
          btn.addEventListener("click", () => {
            const current = parseInt(widget.dataset.span || "12", 10);
            const next = current === 12 ? 6 : 12;
            widget.dataset.span = next;
            widget.style.setProperty("--span", next);
            btn.textContent = next === 12 ? "Half Width" : "Full Width";
            saveLayout();
          });
          const initial = parseInt(widget.dataset.span || "12", 10);
          btn.textContent = initial === 12 ? "Half Width" : "Full Width";
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
          const payload = { area_order: {}, hidden_widget_ids: [], layout_rows: {} };
          grids.forEach(area => {
            const code = area.getAttribute("data-area-grid");
            const visibleWidgets = Array.from(area.querySelectorAll(".widget:not(.is-hidden)"));
            payload.area_order[code] = visibleWidgets.map(w => w.getAttribute("data-widget"));
            payload.layout_rows[code] = serializeRows(visibleWidgets);
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

        function serializeRows(widgets) {
          const rows = [];
          let current = [];
          let total = 0;
          widgets.forEach(widget => {
            const span = parseInt(widget.dataset.span || "12", 10);
            if (total + span > 12 && total > 0) {
              rows.push({ widgets: current });
              current = [];
              total = 0;
            }
            current.push({ id: widget.getAttribute("data-widget"), width: span });
            total += span;
            if (total >= 12) {
              rows.push({ widgets: current });
              current = [];
              total = 0;
            }
          });
          if (current.length) {
            rows.push({ widgets: current });
          }
          return rows;
        }
      })();
    </script>
  </body>
</html>

{{ define "widgets" }}
  {{ $areaCode := .Code }}
  {{ $resizable := or (eq $areaCode "admin.dashboard.main") (eq $areaCode "admin.dashboard.footer") }}
  <div class="widgets-grid" data-area-grid="{{ $areaCode }}">
    {{ if .Widgets }}
      {{ range .Widgets }}
        {{ $span := widgetSpan .Metadata }}
        <article class="widget" data-widget="{{ .ID }}" data-span="{{ $span }}" data-area-code="{{ $areaCode }}" data-resizable="{{ $resizable }}" style="--span: {{ $span }}">
          <div class="widget__toolbar">
            <button type="button" class="hide-widget">Toggle Hide</button>
            <button type="button" class="resize-widget" {{ if not $resizable }}disabled title="Resize only available in Main or Operations"{{ end }}>Half Width</button>
          </div>
          <h3>{{ widgetHeading . }}</h3>
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
          {{ else if chartWidget .Definition }}
            {{ $chart := chartMarkup .Data }}
            {{ if $chart }}
              <div class="widget-chart">
                {{ $chart }}
              </div>
            {{ else }}
              <p class="area-empty">No chart data available.</p>
            {{ end }}
          {{ else if isType .Definition "demo.widget.welcome" }}
            <p><strong>{{ index .Data "headline" }}</strong></p>
            <p>{{ index .Data "message" }}</p>
          {{ else }}
            <pre>{{ printf "%+v" .Data }}</pre>
          {{ end }}
        </article>
      {{ end }}
    {{ else }}
      <p class="area-empty">No widgets configured.</p>
    {{ end }}
  </div>
{{ end }}
`
