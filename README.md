# Go Dashboard

Go Dashboard is a modular toolkit for building server rendered admin dashboards in Go. It provides a layout service, HTTP controller, template renderer, router adapters, CLI tooling, and a provider system so you can compose dashboards from reusable widget definitions stored anywhere (database, CMS, or custom service).

### Key capabilities

- Layout orchestration with per-viewer preferences, telemetry hooks, and WebSocket refresh/broadcast helpers.
- Router-agnostic HTTP/JSON/WebSocket endpoints (first-class support for go-router) plus a thin façade in `pkg/dashboard`.
- Widget provider registry with analytics/chart helpers, schema validation, and manifest-driven discovery/CLI scaffolding.
- Localization helpers for templates and providers, plus translation-aware analytics/chart payloads.

## Packages

- `components/dashboard` – dashboard service, HTTP controllers, commands,
  queries, templates, and transport adapters.
- `components/dashboard/httpapi` – router-agnostic executor interface backed by shared commands.
- `components/dashboard/gorouter` – helpers that register dashboard routes (HTML/JSON/REST/WebSocket) on any `go-router` adapter.
- `pkg/dashboard` – thin façade exposing the service to consumers.
- `pkg/analytics` – helper clients/repositories for wiring real BI/observability data into analytics widgets.
- `pkg/goadmin` – helper utilities for wiring dashboards into go-admin
  (feature flags, menu seeding).

## Quick Start

1. Add go-dashboard to your module and implement `WidgetStore` (wrap go-cms, a custom DB, etc.).
2. Build a `dashboard.Service` with any optional dependencies you need
   (`Authorizer`, `PreferenceStore`, telemetry hooks, analytics providers, etc.).
3. Seed the dashboard (areas, definitions, default layout) once at bootstrap time.
4. Mount the go-router adapter (or wire your own `httpapi.Executor`) so the HTML, JSON, REST, and WebSocket routes are available.

```go
import (
    router "github.com/goliatone/go-router"

    "github.com/goliatone/go-dashboard/components/dashboard"
    "github.com/goliatone/go-dashboard/components/dashboard/commands"
    "github.com/goliatone/go-dashboard/components/dashboard/gorouter"
    "github.com/goliatone/go-dashboard/components/dashboard/httpapi"
)

service := dashboard.NewService(dashboard.Options{WidgetStore: myWidgetStore})
renderer, _ := dashboard.NewTemplateRenderer()
controller := dashboard.NewController(dashboard.ControllerOptions{
    Service:  service,
    Renderer: renderer,
})

executor := &httpapi.CommandExecutor{
    AssignCommander:  commands.NewAssignWidgetCommand(service, nil),
    RemoveCommander:  commands.NewRemoveWidgetCommand(service, nil),
    ReorderCommander: commands.NewReorderWidgetsCommand(service, nil),
    RefreshCommander: commands.NewRefreshWidgetCommand(service, nil),
}

hook := dashboard.NewBroadcastHook()
app := router.New()
_ = gorouter.Register(gorouter.Config[struct{}]{
    Router:     app,
    Controller: controller,
    API:        executor,
    Broadcast:  hook,
})
```

See `docs/TRANSPORTS.md` for details on the go-router adapter, executor interface,
and broadcast hook patterns. For a full working sample (including widget seeding,
custom providers, and menu bootstrapping) check `examples/goadmin/main.go`.

Each route defaults to `/admin/...`, but you can override individual paths (HTML,
layout JSON, CRUD endpoints, preferences, WebSocket) via `gorouter.RouteConfig`
while keeping the same controller/executor wiring.

## Advanced Analytics Widgets

`analytics_funnel`, `cohort_overview`, and `alert_trends` templates ship with DI-friendly providers so dashboards can surface BI/observability data without embedding transport logic. Configuration payloads are validated via JSON schema before widgets hit the store, and templates expose CSS hooks for fine-grained styling. Use `pkg/analytics` to wrap your HTTP BI or observability clients and pass the resulting repositories into `dashboard.New*AnalyticsProvider`. See `docs/ANALYTICS.md` for schemas, provider interfaces, and screenshots.

## Architecture

```
Admin Request (go-router)
        |
        v
  Controller (HTML/JSON)
        |
        v
  dashboard.Service  -----> PreferenceStore
        |                   Authorizer
        |                   Telemetry
        v
  WidgetStore (go-cms) <---- ProviderRegistry ----> Custom Providers
        |
        v
 RefreshHook / BroadcastHook ---> WebSocket / Notifications transports
```

## Personalized Layouts

Per-user layout overrides are persisted through the new preferences endpoint:

```
POST /admin/dashboard/preferences
{
  "area_order": {
    "admin.dashboard.main": ["widget-2", "widget-1"]
  },
  "layout_rows": {
    "admin.dashboard.main": [
      {
        "widgets": [
          {"id": "widget-2", "width": 6},
          {"id": "widget-1", "width": 6}
        ]
      }
    ]
  },
  "hidden_widget_ids": ["widget-3"]
}
```

The route uses the authenticated viewer from go-router, so transports only need to send the desired ordering/hidden widgets (plus optional `layout_rows` to describe per-row widths). The data flows through `dashboard.SavePreferences` → `PreferenceStore`, and overrides are applied automatically during `ConfigureLayout`, which annotates each widget’s metadata with `layout.width`, `layout.row`, etc.

## Widget Discovery & CLI

Manifest-driven discovery lets third-party widgets register without touching Go
code. Author manifests (see `docs/DISCOVERY.md` and the samples under
`docs/manifests/`), then load them via
`registry.LoadManifestFile("docs/manifests/community.widgets.yaml")`. The
`cmd/widgetctl` binary (also exposed as `./taskfile dashboard:widgets:scaffold`)
generates manifest entries, JSON schemas, and provider stubs in one step:

```bash
./taskfile dashboard:widgets:scaffold \
  --code community.widget.pipeline_health \
  --name "Pipeline Health" \
  --description "Tracks CI/CD durations and failure counts." \
  --manifest docs/manifests/community.widgets.yaml
```

CI guardrails in Task 9.4 validate manifests for duplicates and schema issues so
broken widget packs never land in `main`.

## Localization

go-dashboard now mirrors go-cms localization flows:

- Router adapters (including go-router) propagate the locale discovered from URL
  prefixes, query params, or `Accept-Language` into `ViewerContext.Locale`.
- `dashboard.Options` accepts an optional `TranslationService`; the default
  renderer wires it via `dashboard.WithTranslationHelpers`, exposing a `T`
  function to templates (`{{ T("dashboard.widget.system_status.title", locale, "System Status") }}`).
- Providers receive the same translator through `WidgetContext.Translator` so
  server-side strings (quick action labels, system status names, etc.) can be
  localized alongside data.
- Widget definitions/manifests accept `name_localized` and
  `description_localized` maps. Use `dashboard.ResolveLocalizedValue` to pick
  the best translation with graceful fallback to the default string.
- The sample app (`examples/goadmin`) demonstrates locale switching via
  `?locale=es`, including localized quick actions, activity feed verbs, and
  welcome messages without changing transport code.

## Development Workflow

- `./taskfile dashboard:test` – run the focused dashboard test suite (same target used by CI).
- `./taskfile dashboard:lint` – execute Go vet across dashboard components.
- `./taskfile dev:test` – full repository test run, useful before releasing.

See `docs/TROUBLESHOOTING.md` if bootstrap, authorization, or provider issues appear during integration.

## Chart Widgets

Bar/line/pie widgets are rendered server-side through go-echarts. Check
`docs/ECHARTS_WIDGETS.md` for configuration payloads, CSP guidance, and
troubleshooting.

- Sample app: `go run ./examples/goadmin` creates chart widgets in the demo dashboard.
- Dynamic sales widgets are powered by `SalesChartProvider`, which can query any
  repository that satisfies `SalesSeriesRepository` and optionally cache render
  output via `dashboard.NewChartCache`.
- Inspect the JSON layout via:
  ```bash
  curl -s http://localhost:9876/admin/dashboard/_layout \
    | jq '.areas[].widgets[] | select(.definition|endswith("_chart"))'
  ```
- Design + roadmap live in `CHARTS_FEATURE.md` / `CHARTS_TSK.md`.

### Performance & CSP tips

- `./taskfile dashboard:serve:charts` enables Fiber’s compression middleware (`GO_DASHBOARD_ENABLE_GZIP=1`) and serves the bundled ECharts assets; set `GO_DASHBOARD_ECHARTS_CDN` to point at a CDN/self-hosted bucket instead.
- `./taskfile dashboard:assets:echarts` refreshes the embedded ECharts runtime + themes from the upstream go-echarts asset bundle.
- `./taskfile dashboard:bench:charts` runs the new `BenchmarkECharts*` targets to compare cached vs. uncached rendering cost.
- Set `dashboard.Options.ScriptNonce` when building the service to stamp CSP-approved nonces onto every inline `<script>` emitted by go-echarts.
