# Dashboard Components

This package hosts the core building blocks described in `DSH_TDD.md`:

- `service.go` – public API for orchestrating go-cms widgets.
- `bootstrap.go` – helpers that register widget areas/definitions.
- `provider.go` – provider and registry interfaces for widget data.
- `layout.go` – runtime layout resolution and preference plumbing.
- `controller.go` – HTTP/glue helpers for routing dashboards.
- `commands/` – `go-command.Commander` implementations for mutations.
- `queries/` – read-only query objects for layout/widget inspection.
- `httpapi/` – REST handlers that wrap command executions.
- `templates/` – default go-template views + widget partials.
- `refresh_broadcast.go` – optional broadcast + WebSocket/SSE hooks.

All implementation files start as placeholders so we can iterate without
blocking Phase 1 tasks. As functionality lands, expand this README with
more detailed guidance and diagrams.

## Widget Registration Options

1. **Plugin hooks** – call `dashboard.RegisterWidgetHook(func(reg *dashboard.Registry) error { ... })`
   during `init()` to add definitions/providers at build time.
2. **Config manifests** – load JSON/YAML into `[]dashboard.WidgetManifest` and call
   `registry.LoadManifest(...)` so ops teams can toggle widgets without recompiling.
3. **DI services** – pass `*dashboard.Registry` (via interfaces) to modules during
   dependency injection; they can register widgets/providers programmatically.

All three feed the same `WidgetRegistry` implementation, ensuring maintenance
cost stays low while supporting apps of different sizes.
