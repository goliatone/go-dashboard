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

## Typed Page Boundary

`dashboard.Page` is the canonical presentation contract for this package. It
preserves ordered areas directly as `[]PageArea`, carries framework-owned widget
frame metadata in typed fields, and reuses `*ThemeSelection` as the page theme
contract.

`dashboard.Renderer` now renders raw `dashboard.Page` values. The controller no
longer treats payload maps as the primary HTML rendering surface.

`Controller.LayoutPayload(...)` and other payload-map helpers remain available
only as migration adapters for existing integrations. New rendering and
transport work should derive from `dashboard.Page` rather than building or
mutating `map[string]any` payloads. When an older renderer still requires the
historical `Render(name, any, ...)` contract, wrap it explicitly with
`dashboard.AdaptLegacyRenderer(...)`.

## Discovery And Diagnostics

Operational consumers should use the typed discovery and diagnostics contracts
instead of reverse-engineering payload maps:

- `Registry.Catalog()` returns an ordered `CatalogSnapshot` containing registered
  areas, definitions, and provider discovery metadata.
- `Service.Diagnostics(...)` returns typed resolved layout state, viewer
  preferences, and theme information.
- `Controller.Diagnostics(...)` composes those diagnostics with the canonical
  typed `dashboard.Page`.

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

## Theming

- `dashboard.Service` accepts an optional go-theme-compatible `ThemeProvider`
  plus `ThemeSelectorFunc` on `dashboard.Options`. When provided, the resolved
  `ThemeSelection` is attached to the typed page contract, widget contexts, and
  template data (`theme.tokens`, `theme.css_vars_inline`, `theme.assets`,
  `theme.templates`).
- ECharts providers automatically derive a default chart theme from the selected
  variant (dark -> wonderland, light -> westeros); per-widget `theme` config and
  `WithChartTheme/WithChartThemeResolver` still win.
- Typed page rendering exposes `theme` so custom renderers can wire CSS
  variables or theme-specific partials; when no provider is configured,
  behavior is unchanged.
- go-dashboard does not require go-theme. Its safe projection contract is
  verified against the mirrored canonical fixture in
  `testdata/semantic-theme-projection.json`.

```go
themeProvider := loadThemeProviderSomehow() // e.g., go-theme registry adapter
svc := dashboard.NewService(dashboard.Options{
    WidgetStore:   store,
    Providers:     registry,
    ThemeProvider: themeProvider,
    ThemeSelector: func(ctx context.Context, viewer dashboard.ViewerContext) dashboard.ThemeSelector {
        return dashboard.ThemeSelector{Name: "admin", Variant: "dark"}
    },
})
```

### Semantic dashboard contract

Canonical semantic tokens use dotted names. Dashboard consumers resolve:

```text
dashboard component token -> portable token -> existing dashboard default
```

The dashboard extensions are:

- `dashboard.surface`
- `dashboard.card.{background,border,radius,shadow}`
- `dashboard.metric.{label,value,trend-positive,trend-negative}`
- `dashboard.chart.{axis,grid,tooltip-surface,tooltip-text}`

Portable fallbacks include the matching `color.*`, `font.*`, `space.*`,
`radius.*`, `shadow.*`, `motion.*`, and `chart.*` tokens. Existing safe legacy
keys and pre-prefixed CSS variables remain transport-compatible. Canonical
semantic chrome activates only when a valid shell/widget/state consumer token
is present, so omitted, unrelated, chart-only, and legacy tokens do not restyle
dashboard chrome. The stock template emits each semantic declaration
independently: a focus-only theme, for example, adds the focus rule without
resetting card, metric, header, or legacy host styles.

Use:

- `ThemeSelection.SemanticProjection()` for variables plus
  resolved/invalid/supported/unsupported diagnostics.
- `ThemeSelection.SemanticDashboardPlan(usage)` for render-aware stock-template
  styles and consumed/unused diagnostics. The typed page adapter derives this
  inventory automatically for widget dashboards and application shells.
- `ThemeSelection.DashboardConsumerDiagnostics()` only when no page inventory
  is available; it conservatively reports supported tokens as unused rather
  than claiming a renderer consumed them.
- `ThemeSelection.SemanticChartPalette()` for the typed eight-series
  ECharts palette and generic presentation diagnostics.

Widget frames accept `WidgetPresentationState` values `ready`, `loading`,
`empty`, and `error`. Templates emit `data-dashboard-state`; loading also emits
`aria-busy="true"`. The empty state remains represented by
`.dashboard-area--empty`. Controller adaptation rejects present metadata state
values with the wrong type or an unknown name instead of silently rendering
them as ready.

ECharts keeps the selected named/custom `ChartTheme`. When semantic series
tokens are present, validated colors are applied to typed series/data items.
Missing positions retain their current positional default until the first
valid semantic color, then rotate through previously supplied semantic colors.
Axis, grid, tooltip surface, and tooltip text use typed go-echarts options and
its configuration-visitor extension point. Concrete provider diagnostics and
cache identity include only applied presentation: axis and grid are consumed
for bar, line, and scatter charts, while pie and gauge report them unused. The
render cache includes the resolved named theme and applied semantic palette, so
variants cannot reuse stale chart markup.

## Application Shell

`dashboard.Shell` is an opt-in application/workbench shell for modules that need
side rails, splitters, collapsible regions, and focus mode. `go-dashboard` owns
the shell mechanics; consuming modules own the region content and behavior.
Existing widget dashboards keep using `Page.Areas` and are unchanged unless
`Page.Shell` is set.

Module content is supplied through `ShellRegion.Content`. Use `Text` for escaped
plain text, or `HTML` when the caller has already produced trusted markup. Shell
assets can be added with:

```go
assets := dashboard.PageAssets{}
assets.AddShellAssets("")
page.Assets = &assets
```

Default asset URLs are:

- CSS: `/dashboard/assets/shell/shell.css`
- JS: `/dashboard/assets/shell/shell.js`

The go-router adapter serves those files separately from ECharts assets. Override
`gorouter.RouteConfig.ShellAssets` when an app needs a different local prefix.
By default, `gorouter.Register` mounts both embedded asset families on
`Config.Router`. Set `Config.AssetRouter` when dashboard endpoints use a grouped
router but assets must remain on canonical root paths. Hosts that already mount
the same assets on a dedicated static surface should set
`AssetRegistrationModeExternal` instead.

The declarative runtime contract is:

- Root: `data-dashboard-shell`, `data-dashboard-shell-surface`,
  `data-dashboard-shell-version`
- Region: `data-shell-region`, `data-shell-role`, `data-shell-placement`
- Rail controls: `data-shell-toggle`, `data-shell-resize`
- Focus controls: `data-shell-focus-toggle`, `data-shell-focus-exit`
- Optional `ShellAction` controls render with `data-shell-action` and map known
  kinds to the same toggle/focus attributes: `toggle-region`, `focus`,
  `exit-focus`, or plain `button`.

Resize handles render `role="separator"` with vertical orientation and
`aria-valuemin/max/now`. The runtime also supports keyboard resizing with arrow
keys plus Home/End.

Theme tokens are ordinary `ThemeSelection` CSS variables. The shell CSS reads
the canonical dashboard and portable semantic variables first, then preserves
the legacy `--dashboard-*` variables and original literals as final fallbacks.
This keeps existing hosts unchanged while allowing the same semantic selection
to style shell surfaces, cards, text, focus, radius, and shadow.

Browser state is scoped as:

```text
go-dashboard:shell:v<version>:<surface>[:module:<module>]:viewer:<viewer|anonymous>
```

When viewer identity is not available client-side, the runtime intentionally uses
`viewer:anonymous`. Server-backed shell preferences remain deferred.

Validation commands:

```sh
go test ./...
node --test components/dashboard/assets/shell/shell.test.mjs
```

The follow-up `go-admin` adoption spec can migrate local pane controllers once
Content Types and Block Library are represented by the generic `navigation`,
`main`, `palette`, and `inspector` region roles; their collapse, resize, focus,
storage-restore, and keyboard resize behavior should remain intact.
