# Dashboard Commands

This package contains the `go-command.Commander` implementations that wrap every
state-changing workflow exposed by go-dashboard:

- `SeedDashboardCommand` bootstraps widget areas, definitions, and optional layouts.
- `AssignWidgetCommand`, `RemoveWidgetCommand`, and `ReorderWidgetsCommand`
  encapsulate CRUD operations so HTTP/WebSocket transports stay thin.
- `UpdateWidgetCommand` mutates widget configuration/metadata without reassigning it.
- `RefreshWidgetCommand` fans out events via the configured `RefreshHook`.

Each command records telemetry and depends only on interfaces (`dashboard.Service`,
`WidgetStore`, etc.), which keeps them reusable by REST handlers, background jobs,
or CLIs. Tests under this directory validate wiring without hitting go-cms.
