# Dashboard Components

This package hosts the core building blocks described in `DSH_TDD.md`:

- `service.go` – public API for orchestrating go-cms widgets.
- `bootstrap.go` – helpers that register widget areas/definitions.
- `provider.go` – provider and registry interfaces for widget data.
- `layout.go` – runtime layout resolution and preference plumbing.
- `controller.go` – HTTP/glue helpers for routing dashboards.
- `commands/` – `go-command.Commander` implementations for mutations.
- `queries/` – read-only query objects for layout/widget inspection.
- `templates/` – default go-template views + widget partials.

All implementation files start as placeholders so we can iterate without
blocking Phase 1 tasks. As functionality lands, expand this README with
more detailed guidance and diagrams.
