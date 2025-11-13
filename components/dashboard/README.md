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
  It now accepts per-endpoint overrides via `RouteConfig` so transports can keep
  the shared controller/commands but expose them under any URL scheme.
- `templates/` – embedded go-template dashboard and widget partials.
- `refresh_broadcast.go` – in-process broadcast hook consumed by transports.

The directory now contains the production-ready implementation; refer to
`docs/TRANSPORTS.md` for diagrams and integration notes.

## Widget Registration Options

1. **Plugin hooks** – call `dashboard.RegisterWidgetHook(func(reg *dashboard.Registry) error { ... })`
   during `init()` to add definitions/providers at build time.
2. **Config manifests** – ship YAML/JSON manifests (see `docs/DISCOVERY.md`) and
   call `registry.LoadManifestFile("docs/manifests/community.widgets.yaml")`.
3. **DI services** – pass `*dashboard.Registry` (via interfaces) to modules during
   dependency injection; they can register widgets/providers programmatically.

All three feed the same `WidgetRegistry` implementation, ensuring maintenance
cost stays low while supporting apps of different sizes.

Use `cmd/widgetctl` (or `./taskfile dashboard:widgets:scaffold`) to generate
manifest entries and provider stubs when building new widget packs.

## Analytics Templates & Styling Hooks

The Phase 8 widgets ship with embedded partials that expose predictable class
names so host applications can theme them without editing Go code:

- `.widget--funnel`, `.funnel-steps__bar`, `.funnel-steps__label`
- `.widget--cohort`, `.cohort-list__row`, `.cohort__cell`
- `.widget--alerts`, `.alert-trends__badge`, `.alert-trends__bar--{critical|warning|info}`

Override these selectors from your admin stylesheet or provide an alternative
renderer via `ControllerOptions.Template` if you need a completely different
layout.
