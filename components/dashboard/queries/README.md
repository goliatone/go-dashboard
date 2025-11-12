# Dashboard Queries

Encapsulates read-only operations so transports/test suites can reuse common
logic without depending on the entire dashboard service:

- `LayoutQuery` resolves the full dashboard layout for a viewer.
- `WidgetAreaQuery` fetches a single area (`admin.dashboard.main`, etc.) with
  the correct audience, locale, and provider metadata applied.

The queries wrap `dashboard.Service` interfaces and satisfy
`go-command.Querier`, making them easy to plug into schedulers, GraphQL
resolvers, or other read-focused transports.
