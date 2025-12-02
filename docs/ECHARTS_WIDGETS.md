# ECharts Widgets

Server-rendered charts (bar, line, pie) are available through the go-echarts integration. This guide explains how to configure widgets, satisfy
CSP requirements, and verify that transports render the generated HTML.

---

## Quick Start

1. Ensure your dashboard service was built after **Phase 1** of `CHARTS_TSK.md`
   (new definitions + providers).
2. Add a widget instance via command or API:

```json
POST /admin/dashboard/widgets
{
  "definition_id": "admin.widget.bar_chart",
  "area_code": "admin.dashboard.main",
  "configuration": {
    "title": "Quarterly Revenue",
    "x_axis": ["Q1", "Q2", "Q3", "Q4"],
    "series": [
      {"name": "North America", "data": [120, 132, 101, 134]},
      {"name": "EMEA", "data": [220, 182, 191, 234]}
    ],
    "footer_note": "Data refreshed nightly"
  }
}
```

3. Run the sample app for a full walkthrough:

```bash
go run ./examples/goadmin
open http://localhost:9876/admin/dashboard
```

Use `curl -s http://localhost:9876/admin/dashboard/_layout | jq '.areas[].widgets[] | select(.definition=="admin.widget.bar_chart")'`
to inspect the server response that includes `chart_html`.

---

## Configuration Reference

| Field         | Type            | Notes |
|---------------|-----------------|-------|
| `title`       | string          | Display title; translated via `TranslationService` when available. |
| `subtitle`    | string          | Optional subtitle rendered under the title. |
| `x_axis`      | array[string]   | Labels for bar/line/scatter charts. When omitted the provider infers labels from the first series. |
| `series`      | array<object>   | **Required.** Each series needs a `name` and `data`. `data` accepts simple numbers (e.g., `[1,2,3]`) or `{ "name": "Slice A", "value": 42 }` objects. |
| `footer_note` | string          | Optional footnote rendered in the widget footer. |
| `theme`       | string          | Optional per-widget theme override (`westeros`, `walden`, `wonderland`, `dark`). |
| `dynamic`     | boolean         | Marks widgets as real-time capable; transports can use this to wire WebSocket/SSE refreshes. |
| `refresh_endpoint` | string     | Optional HTTP endpoint used by transports when `dynamic` is true. |
| `show_chart_title` | boolean    | Defaults to `false`. When `true`, go-echarts also renders the title/subtitle inside the chart canvas (useful if you hide the widget header). |

Pie/gauge charts only use the first `series` entry because the underlying ECharts API
expects a single dataset.

**Scatter data:** supply `{ "x": <number>, "y": <number> }` objects (optional `name`) to plot value pairs.

---

## CSP & Security

- go-echarts emits inline `<script>` blocks. Configure your web server with a
  script nonce (preferred) or allowlist the ECharts CDN origin when using
  `opts.Initialization{AssetsHost: ...}`. Provide `dashboard.Options.ScriptNonce`
  when constructing the service so every inline script automatically receives
  the nonce attribute.
- Widget configuration (titles, subtitles, axis labels, series names) is parsed
  and HTML-escaped on the server before go-echarts receives it. Avoid
  concatenating raw config strings directly into templates.
- Providers run with the standard `ViewerContext`. Continue to enforce
  viewer/role-based access in custom providers before fetching sensitive data.

See `CHARTS_FEATURE.md` "Challenge 5: Security, CSP, and Data Isolation" for
deployment recommendations.

### Assets & Dynamic Injection

- **Bundled by default:** go-dashboard now ships the ECharts runtime + themes
  under `/dashboard/assets/echarts/` and serves them via the go-router adapter.
  The generated chart HTML points here out of the box.
- **Override when needed:** set `GO_DASHBOARD_ECHARTS_CDN` or pass
  `dashboard.WithChartAssetsHost(...)` to point to a CDN/self-hosted bucket.
- **Non go-router hosts:** mount `dashboard.EChartsAssetsHandler` (or
  `EChartsAssetsFS` with your framework’s static middleware) at
  `/dashboard/assets/echarts/` so the default URLs resolve.
- **JSON API / innerHTML:** if you inject chart HTML client-side, either rely on
  the bundled same-origin assets or preload the runtime in your page shell before
  calling `innerHTML`. Programmatic script loading (creating `<script>` nodes and
  awaiting `onload`) also avoids `echarts is not defined` races.
- **Update assets:** run `./taskfile dashboard:assets:echarts` to refresh the
  embedded runtime/themes from the upstream go-echarts assets bundle.

---

## Theming & Localization

- Titles/subtitles call `TranslationService` using the key pattern
  `dashboard.widget.<definition>.title`. Provide translations through your
  existing translation backend for localized dashboards.
- Customize themes per-viewer by supplying `dashboard.WithChartThemeResolver`
  when constructing `EChartsProvider`, or per-widget via the `theme` config
  field. CSS overrides remain available via `.widget--echarts` classes.
- When the dashboard service is configured with a go-theme `ThemeProvider` +
  selector, ECharts providers derive a default theme from the selected variant
  (e.g., dark -> wonderland, light -> westeros). Per-widget `theme` overrides
  and `WithChartTheme/WithChartThemeResolver` still take precedence.

---

## Dynamic Data Providers

- `components/dashboard/providers_sales_chart.go` contains `SalesChartProvider`,
  which consumes a `SalesSeriesRepository` and delegates rendering to
  `EChartsProvider`. The repository receives viewer/segment metadata so you can
  enforce authorization and fetch tenant-specific metrics.
- Add concurrency by fetching comparison metrics (see the `comparison_metric`
  config in `admin.widget.sales_chart`). The provider will call the repository
  twice and render side-by-side series.
- Mark widgets as `dynamic: true` and set `refresh_endpoint` so transports know
  they can stream incremental updates over WebSocket/SSE without rerendering
  full HTML.

### Sales Chart Configuration

The built in `admin.widget.sales_chart` widget uses the same
`widgets/echarts_chart.html` template as the other chart definitions—there is no
Chart.js fallback or dual-rendering layer to maintain.

| Field | Type | Notes |
|-------|------|-------|
| `period` | string | Enum: `7d`, `14d`, `30d`, `60d`, `90d`, `180d`. Defaults to `30d`. |
| `metric` | string | Enum: `revenue`, `orders`, `customers`, `seats`, `pipeline`. Defaults to `revenue`. |
| `comparison_metric` | string | Optional metric from the same enum to plot side-by-side with `metric`. |
| `segment` | string | Free-form descriptor rendered in the title (e.g., `enterprise`, `north america`). Defaults to `all customers`. |
| `dynamic` | boolean | Enables SSE/WebSocket refresh hints; pass-through to the rendered widget metadata. |
| `refresh_endpoint` | string | Optional URL transports can poll/hit when `dynamic` is true. |
| `theme` | string | Optional theme override (`westeros`, `walden`, `wonderland`, `chalk`). |
| `footer_note` | string | Optional footnote rendered under the chart. |
| `show_chart_title` | boolean | Same as above; defaults to `false` so the widget header owns the title. |

`SalesChartProvider` derives the title/subtitle from the selected metric, period,
and segment, so you generally do not supply chart `series` manually.

---

## Performance & Caching

- `dashboard.NewChartCache(ttl)` memoizes rendered HTML. Pass it via
  `dashboard.WithChartCache` when constructing providers (the shared default
  cache uses a 5 minute TTL).
- Use `WithChartAssetsHost("https://cdn.jsdelivr.net/npm/echarts@5/dist/")` to
  reference cached CDN copies of the ECharts runtime instead of embedding the
  script tag every time, or set `GO_DASHBOARD_ECHARTS_CDN` before running the
  sample Task so it points to your CDN bucket.
- Sample app (`examples/goadmin`) demonstrates both cache usage and a dynamic
  sales chart seeded via `SalesChartProvider`.
- Run `./taskfile dashboard:serve:charts` to boot the demo with Fiber’s gzip
  middleware (`GO_DASHBOARD_ENABLE_GZIP=1`), and
  `./taskfile dashboard:bench:charts` to capture cached vs. uncached benchmark
  numbers.

---

## Troubleshooting

- **Blank widget:** ensure `series` is non-empty and contains numeric data.
- **Large payloads:** enable gzip/deflate in your HTTP server and consider
  caching chart HTML (Phase 3 introduces `ChartCache`).
- **CSP violations:** confirm that script nonces propagate through your reverse
  proxy or relax `script-src` to include the nonce + CDN origin used by
  go-echarts.
- **Layout overflow:** charts inherit the dashboard grid width; use layout
  preferences or `footer_note` to communicate wide datasets and encourage
  full-width placement when needed.

For more examples, consult `CHARTS_FEATURE.md` and the seeding logic inside
`examples/goadmin/main.go`.
