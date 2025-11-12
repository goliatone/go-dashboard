package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	router "github.com/goliatone/go-router"

	"github.com/goliatone/go-dashboard/components/dashboard"
	"github.com/goliatone/go-dashboard/components/dashboard/commands"
	"github.com/goliatone/go-dashboard/components/dashboard/gorouter"
	"github.com/goliatone/go-dashboard/components/dashboard/httpapi"
	dashboardpkg "github.com/goliatone/go-dashboard/pkg/dashboard"
	"github.com/goliatone/go-dashboard/pkg/goadmin"
)

func main() {
	ctx := context.Background()

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
	if err := seed.Execute(ctx, commands.SeedDashboardInput{SeedLayout: true}); err != nil {
		log.Fatalf("seed dashboard: %v", err)
	}

	if err := service.AddWidget(ctx, dashboard.AddWidgetRequest{
		DefinitionID: customDefinition.Code,
		AreaCode:     "admin.dashboard.main",
		Configuration: map[string]any{
			"message": "All systems are running smoothly. Customize this provider to show data from your own services.",
		},
	}); err != nil {
		log.Fatalf("add welcome widget: %v", err)
	}
	funnelPos := 0
	if err := service.AddWidget(ctx, dashboard.AddWidgetRequest{
		DefinitionID: "admin.widget.analytics_funnel",
		AreaCode:     "admin.dashboard.main",
		Position:     &funnelPos,
		Configuration: map[string]any{
			"range":   "30d",
			"segment": "enterprise",
			"goal":    52,
		},
	}); err != nil {
		log.Fatalf("add analytics widget: %v", err)
	}

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
	}).Parse(dashboardTemplate))
	return sampleRenderer{tmpl: tmpl}
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
	}
	if areas, ok := raw["areas"].(map[string]any); ok {
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
		if dataMap, ok := widgetMap["data"].(map[string]any); ok {
			widget.Data = dataMap
		} else if widgetMap["data"] != nil {
			widget.Data = map[string]any{"value": widgetMap["data"]}
		}
		area.Widgets = append(area.Widgets, widget)
	}
	return area
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

const dashboardTemplate = `<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <title>{{ .Title }}</title>
    <style>
      body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; background: #f4f5f7; margin: 0; }
      header { background: #1f2937; color: white; padding: 1.5rem 2rem; }
      header p { margin: 0.25rem 0 0; color: #d1d5db; }
      .dashboard { padding: 2rem; display: grid; gap: 1.5rem; grid-template-columns: 2fr 1fr; }
      .area-footer { grid-column: 1 / -1; }
      section.area { background: white; border-radius: 12px; box-shadow: 0 2px 8px rgba(31,41,55,0.12); padding: 1.5rem; }
      section.area h2 { margin-top: 0; font-size: 1.1rem; text-transform: uppercase; letter-spacing: 0.08em; color: #6b7280; }
      .widget { border: 1px solid #e5e7eb; border-radius: 10px; padding: 1rem; margin-top: 1rem; background: #fafafa; cursor: grab; }
      .widget.dragging { opacity: 0.5; }
      .widget.is-hidden { opacity: 0.35; }
      .widget h3 { margin: 0 0 0.5rem; }
      .metrics { display: flex; gap: 1rem; }
      .metric { flex: 1; background: white; border-radius: 8px; padding: 0.75rem; text-align: center; }
      .metric span { display: block; font-size: 1.4rem; font-weight: bold; }
      .activity { list-style: none; padding: 0; margin: 0; }
      .activity li { padding: 0.5rem 0; border-bottom: 1px solid #e5e7eb; }
      .actions { display: flex; flex-wrap: wrap; gap: 0.5rem; }
      .actions a { text-decoration: none; background: #2563eb; color: white; padding: 0.5rem 1rem; border-radius: 6px; font-size: 0.9rem; }
      .status { list-style: none; padding: 0; margin: 0; }
      .status li { display: flex; justify-content: space-between; padding: 0.35rem 0; }
      .widget__toolbar { display: flex; justify-content: flex-end; gap: 0.5rem; margin-bottom: 0.5rem; }
      .widget__toolbar button { border: none; background: #e5e7eb; border-radius: 4px; padding: 0.2rem 0.6rem; cursor: pointer; font-size: 0.8rem; }
      #save-status { margin: 1rem 2rem 0; color: #2563eb; font-size: 0.9rem; }
    </style>
  </head>
  <body>
    <header>
      <h1>{{ .Title }}</h1>
      {{ if .Description }}<p>{{ .Description }}</p>{{ end }}
    </header>
    <p id="save-status">Drag widgets between areas or hit "Toggle Hide" to personalize this dashboard. Changes autosave.</p>
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
            <h2>Footer</h2>
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
              <span>{{ $value }}</span>
            </div>
          {{ end }}
        </div>
      {{ else if isType .Definition "admin.widget.recent_activity" }}
        <ul class="activity">
          {{ range $item := index .Data "items" }}
            <li><strong>{{ index $item "user" }}</strong> {{ index $item "action" }} · {{ index $item "ago" }}</li>
          {{ end }}
        </ul>
      {{ else if isType .Definition "admin.widget.quick_actions" }}
        <div class="actions">
          {{ range $action := index .Data "actions" }}
            <a href="{{ index $action "route" }}">{{ index $action "label" }}</a>
          {{ end }}
        </div>
      {{ else if isType .Definition "admin.widget.system_status" }}
        <ul class="status">
          {{ range $check := index .Data "checks" }}
            <li>
              <span>{{ index $check "name" }}</span>
              <strong>{{ index $check "status" }}</strong>
            </li>
          {{ end }}
        </ul>
      {{ else if isType .Definition "admin.widget.analytics_funnel" }}
        <p><strong>{{ index .Data "conversion_rate" }}%</strong> conversion (goal {{ index .Data "goal" }}%)</p>
        <ul class="activity">
          {{ range $step := index .Data "steps" }}
            <li>
              <strong>{{ index $step "label" }}</strong>
              {{ index $step "value" }} · {{ printf "%.1f%%" (index $step "percent") }} of top
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
