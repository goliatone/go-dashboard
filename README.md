# Go Dashboard

Core building blocks for go-admin dashboards backed by go-cms widgets.

## Packages

- `components/dashboard` – dashboard service, HTTP controllers, commands,
  queries, templates, and transport adapters.
- `components/dashboard/httpapi` – router-agnostic executor interface backed by shared commands.
- `components/dashboard/gorouter` – helpers that register dashboard routes (HTML/JSON/REST/WebSocket) on any `go-router` adapter.
- `pkg/dashboard` – thin façade exposing the service to consumers.
- `pkg/goadmin` – helper utilities for wiring dashboards into go-admin
  (feature flags, menu seeding).

## Quick Start

1. Add go-dashboard alongside go-cms/go-admin in your module and provide a `WidgetStore`
   implementation (typically the go-cms widgets service).
2. Build a `dashboard.Service` with any optional dependencies you need
   (`Authorizer`, `PreferenceStore`, telemetry hooks, etc.).
3. Seed the dashboard (areas, definitions, default layout) once at bootstrap time.
4. Mount the go-router adapter so the HTML, JSON, REST, and WebSocket routes are available.

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

## Development Workflow

- `./taskfile dashboard:test` – run the focused dashboard test suite (same target used by CI).
- `./taskfile dashboard:lint` – execute Go vet across dashboard components.
- `./taskfile dev:test` – full repository test run, useful before releasing.

See `docs/TROUBLESHOOTING.md` if bootstrap, authorization, or provider issues appear during integration.
