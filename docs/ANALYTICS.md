# Advanced Analytics Widgets

Phase 8 introduces three high-value analytics widgets that showcase how
dashboard providers can consume external BI/reporting services while
remaining transport-neutral.

![Analytics widgets preview](images/analytics-demo.svg)

## Widget Definitions & Schemas

| Code | Purpose | Config Schema Highlights |
| --- | --- | --- |
| `admin.widget.analytics_funnel` | Visualizes conversion drop-off across funnel stages. | `range` (`7d`/`14d`/`30d`/`90d`/`180d`), optional `segment` label, `goal` (0-100%) used for alerting. |
| `admin.widget.cohort_overview` | Shows cohort retention/activation tables. | `interval` (`weekly` or `monthly`), `periods` (4-12), `metric` (`active`, `retained`, `upgraded`). |
| `admin.widget.alert_trends` | Highlights alert volume by severity over time. | `lookback_days` (7-90), `severity` (multi-select array), optional `service` filter. |

Schemas are embedded in `components/dashboard/defaults.go` and enforced at
runtime via the new `ConfigValidator`. Calls to `Service.AddWidget` now fail
fast if the payload does not satisfy the JSON schema, preventing malformed
dashboard entries from reaching go-cms.

## Provider Interfaces

Each widget accepts a DI-friendly repository so applications can plug in their
own analytics sources:

```go
type FunnelReportRepository interface {
    FetchFunnelReport(ctx context.Context, query dashboard.FunnelQuery) (dashboard.FunnelReport, error)
}

type CohortReportRepository interface {
    FetchCohortReport(ctx context.Context, query dashboard.CohortQuery) (dashboard.CohortReport, error)
}

type AlertTrendsRepository interface {
    FetchAlertTrends(ctx context.Context, query dashboard.AlertTrendQuery) (dashboard.AlertTrendsReport, error)
}
```

Constructor helpers (`NewFunnelAnalyticsProvider`, `NewCohortAnalyticsProvider`,
`NewAlertTrendsProvider`) wire these repositories into the provider registry.
The default build ships with `Demo*Repository` implementations so examples and
tests render realistic data without external dependencies.

### Live Clients

Use `pkg/analytics` when you want to connect the widgets to real services:

```go
client, _ := analytics.NewHTTPClient(analytics.HTTPConfig{
    BaseURL: os.Getenv("ANALYTICS_API_URL"),
    APIKey:  os.Getenv("ANALYTICS_API_TOKEN"),
})

funnelRepo := analytics.NewFunnelRepository(client)
cohortRepo := analytics.NewCohortRepository(client)
alertRepo := analytics.NewAlertRepository(client)

_ = registry.RegisterProvider("admin.widget.analytics_funnel",
    dashboard.NewFunnelAnalyticsProvider(funnelRepo))
_ = registry.RegisterProvider("admin.widget.cohort_overview",
    dashboard.NewCohortAnalyticsProvider(cohortRepo))
_ = registry.RegisterProvider("admin.widget.alert_trends",
    dashboard.NewAlertTrendsProvider(alertRepo))
```

For local demos or tests, `analytics.NewMockClient` provides deterministic
fixtures that can also be registered through the same helper constructors.

### Styling Hooks & Templates

Each partial exposes predictable CSS hooks so integrators can override styles
without editing the embedded templates:

- `widgets/analytics_funnel.html` &rarr; `.widget--funnel`, `.funnel-steps__bar`,
  `.funnel-steps__label`
- `widgets/cohort_overview.html` &rarr; `.widget--cohort`, `.cohort-list__row`,
  `.cohort__cell` (uses a CSS variable `--rate` to color retention pills)
- `widgets/alert_trends.html` &rarr; `.widget--alerts`, `.alert-trends__badge`,
  `.alert-trends__bar--{critical|warning|info}`

Override these classes (or extend them in your host stylesheet) to match the
rest of your admin UI.

## Plugging Real Data Sources

1. Implement the repository interface that matches your data source (BI API,
   warehouse, observability backend, etc.).
2. Register the provider during DI:

```go
repo := analytics.NewFunnelRepository(client)
registry.RegisterProvider("admin.widget.analytics_funnel",
    dashboard.NewFunnelAnalyticsProvider(repo))
```

3. When you assign the widget (`Service.AddWidget`), pass the schema-compliant
   configuration (e.g., `{"range":"30d","segment":"enterprise"}`).
4. Transports render the same partials regardless of whether the data arrived
   from the demo repos or your production services.

The sample go-admin app (`examples/goadmin`) now seeds the dashboard with the
funnel widget so you can see the end-to-end flow before connecting live data.
