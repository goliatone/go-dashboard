# Dashboard Transport Patterns

go-dashboard exposes several integration points so applications can choose
the transport strategy that best fits their stack.

## HTML Controller
- `components/dashboard.Controller` resolves layouts via `Service.ConfigureLayout`
  and renders go-template views. Use `Render` for data-only responses or
  `RenderTemplate` to stream HTML into any writer.
- The go-router adapter already wires this controller to `/admin/dashboard` and
  `/admin/dashboard/_layout`, so most apps only need to provide a renderer and
  viewer resolver.

## go-router Adapter
- `components/dashboard/gorouter` exposes `Register` which mounts HTML, JSON,
  CRUD, and WebSocket routes on any go-router implementation.
- Supply `Config.API` with an `httpapi.Executor` (see below), the dashboard
  controller, broadcast hook, and a viewer resolver that extracts user/role
  metadata from the router context.
- Localization is automatic: if your `ViewerResolver` omits `Locale`, the
  adapter falls back to query params, URL prefixes, or `Accept-Language` so
  `ViewerContext.Locale` is always populated for downstream providers/templates.
- Customize URLs by setting `Config.BasePath` (changes the prefix) or providing
  a `RouteConfig` with per-endpoint paths (HTML, `_layout`, CRUD, preferences,
  WebSocket) while reusing the same controller/command wiring.

## Router-Agnostic Command Executor
- `components/dashboard/httpapi` defines the `Executor` interface. It exposes
  strongly typed methods (`Assign`, `Remove`, `Reorder`, `Refresh`) backed by
  the shared commands.
- `CommandExecutor` is the default implementation that wraps
  `go-command.Commander[T]` instances. Transports only need to parse JSON (or
  their protocol of choice) into the relevant structs before calling the executor.

## Layout Preferences API
- `POST /dashboard/preferences` accepts a payload containing `area_order` and
  `hidden_widget_ids`. Viewer information is inferred from the router context.
- The go-router adapter invokes `SaveLayoutPreferencesCommand`, which persists
  overrides via the configured `PreferenceStore`. `ConfigureLayout` then applies
  user-specific ordering and hides the requested widgets.

## WebSocket Broadcast
- `dashboard.NewBroadcastHook()` implements `RefreshHook` and fans out widget
  events to in-process subscribers.
- The go-router adapter subscribes to this hook and streams events over
  WebSockets, keeping all protocol-specific code in one place.
- For alternative transports (notifications, SSE, etc.), subscribe to the hook
  and forward the events with your own adapters.

## Notifications
- `NotificationsHook` plugs into go-notifications (or similar systems) through a
  minimal client interface. Whenever widgets update, events are forwarded to the
  configured channel.

These adapters let you mix and match HTML rendering, REST/command execution, and
WebSocket or notification transports without duplicating business logic.
