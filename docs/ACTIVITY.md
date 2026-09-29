# go-dashboard Activity Hooks

This module mirrors the activity pattern used in go-cms, go-notifications, and go-options. The core contracts live in `pkg/activity` and are compatible with `go-users/pkg/types.ActivitySink`.

## Wiring

- Provide hooks + config when constructing the service:
  ```go
  import (
    activity "github.com/goliatone/go-dashboard/pkg/activity"
    usersink "github.com/goliatone/go-dashboard/pkg/activity/usersink"
  )

  svc := dashboard.NewService(dashboard.Options{
    WidgetStore:    store,
    Providers:      registry,
    ActivityConfig: activity.Config{Enabled: true, Channel: "dashboard"},
    ActivityHooks: activity.Hooks{
      usersink.Hook{Sink: myActivitySink}, // forwards to go-users
      activity.HookFunc(func(ctx context.Context, evt activity.Event) error {
        // metrics/logging example
        return nil
      }),
    },
    ActivityFeed: myActivityFeed, // optional; drives recent-activity widget
  })
  ```
- Leave hooks empty to disable emissions; channel defaults to `dashboard`.
- `ActivityFeed` is optional; the recent-activity widget falls back to a static demo feed if none is supplied.

## Emission Points

- `dashboard.widget.add|update|remove|reorder`
- `dashboard.widget.event` (refresh/notify)
- `dashboard.preferences.save`

Metadata includes widget IDs, definition IDs, area codes, locale, and counts where applicable. Actor/User/Tenant IDs are pulled from request context or command payloads when provided.

## Adapters

- `pkg/activity/usersink.Hook` maps events into `types.ActivityRecord` (UUID parsing, metadata passthrough for `definition_code`/`recipients`).
- `pkg/activity/admininterop` adapts generic admin-style records (`actor`, `action`, `object`, `channel`, `metadata`, `occurred_at`) into `pkg/activity.Event` + hook emission.
- `pkg/activity.CaptureHook` is available for tests.

## Admin Interop

Use `admininterop` when another system emits activity entries shaped like `actor/action/object`.

```go
import (
  "context"

  dashboardactivity "github.com/goliatone/go-dashboard/pkg/activity"
  "github.com/goliatone/go-dashboard/pkg/activity/admininterop"
)

hooks := dashboardactivity.Hooks{
  dashboardactivity.HookFunc(func(ctx context.Context, evt dashboardactivity.Event) error {
    // metrics, notifications, audit forwarding...
    return nil
  }),
}

sink := admininterop.NewSink(
  hooks,
  dashboardactivity.Config{Enabled: true, Channel: "dashboard"},
  // Optional override; default is "admin"
  admininterop.WithDefaultChannel("admin"),
)

_ = sink.Record(context.Background(), admininterop.Record{
  Actor:   "user-123",
  Action:  "update",
  Object:  "page:42",
  Metadata: map[string]any{
    "locale": "en",
  },
})
```

Channel precedence:
- `record.Channel` if set
- adapter default channel (`admin` unless overridden)
- emitter default channel (`activity.Config.Channel`, default `dashboard`)

## Composite Object Parsing

`activity.ParseCompositeObject(input)` is the canonical parser for `type:id` object identifiers:

- No colon: returns `(trimmedInput, "", false)`.
- Multiple colons: splits at the first colon; remainder is `objectID`.
- Empty parts (`":id"` or `"type:"`): returns trimmed parts with `ok=false`.

## go-admin/go-auth Style Integration

`go-dashboard` does not import `go-admin` types to avoid dependency cycles. Instead, map your external event shape into `admininterop.Record`.

```go
func onGoAuthEvent(ctx context.Context, sink admininterop.Sink, e AuthEvent) error {
  object := "user:" + e.UserID
  return sink.Record(ctx, admininterop.Record{
    Actor:      e.ActorID,
    Action:     string(e.EventType),
    Object:     object,
    Channel:    "auth",
    Metadata:   e.Metadata,
    OccurredAt: e.OccurredAt,
  })
}
```

See `ACTIVITY_TDD.md` for the design context and `ACTIVITY_TSK.md` for the implementation checklist.
