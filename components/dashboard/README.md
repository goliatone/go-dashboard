# Dashboard Components

This package hosts the core building blocks described in `DSH_TDD.md`:

- `service.go` – orchestrates go-cms widgets and provider execution.
- `bootstrap.go` – registers widget areas/definitions and seeds defaults.
- `provider.go` / `providers.go` – provider interfaces plus canonical widgets.
- `layout.go` – merges go-cms data with preference overrides + auth filters.
- `controller.go` – framework-agnostic controller invoked by transports.
- `commands/` – `go-command.Commander` implementations for seeding and CRUD.
- `queries/` – read-only helpers (layout resolution, area inspection).
- `httpapi/` – router-agnostic executor interface backed by the commands.
- `gorouter/` – go-router adapter that mounts HTML, JSON, CRUD, WebSocket routes.
- `templates/` – embedded go-template dashboard and widget partials.
- `refresh_broadcast.go` – in-process broadcast hook consumed by transports.

The directory now contains the production-ready implementation; refer to
`docs/TRANSPORTS.md` for diagrams and integration notes.

## Widget Registration Options

1. **Plugin hooks** – call `dashboard.RegisterWidgetHook(func(reg *dashboard.Registry) error { ... })`
   during `init()` to add definitions/providers at build time.
2. **Config manifests** – load JSON/YAML into `[]dashboard.WidgetManifest` and call
   `registry.LoadManifest(...)` so ops teams can toggle widgets without recompiling.
3. **DI services** – pass `*dashboard.Registry` (via interfaces) to modules during
   dependency injection; they can register widgets/providers programmatically.

All three feed the same `WidgetRegistry` implementation, ensuring maintenance
cost stays low while supporting apps of different sizes.
