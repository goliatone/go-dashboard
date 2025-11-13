# Dashboard Troubleshooting Guide

Common issues encountered while wiring go-dashboard into go-admin deployments.

## Bootstrap Failures
- **Symptoms**: `RegisterAreas` or `RegisterDefinitions` returns an error on startup.
- **Checks**:
  1. Verify the configured `WidgetStore` has permission to create areas/definitions in go-cms.
  2. Confirm `commands.SeedDashboardCommand` is not running concurrently; wrap it with a mutex/once at bootstrap time.
  3. Ensure `dashboard.DefaultAreaDefinitions()` codes (`admin.dashboard.*`) are not blocked by your go-cms tenant configuration.
- **Fix**: Call `SeedDashboardCommand` once during deployment (migration/job) before exposing the dashboard routes.

## Permission / Visibility Issues
- **Symptoms**: Widgets disappear for users that should see them, or restricted widgets leak to unauthorized users.
- **Checks**:
  1. Make sure the `Authorizer` supplied in `dashboard.Options` enforces the same policies as go-auth/go-admin.
  2. Inspect the widget visibility metadata stored in go-cms (`WidgetVisibility.Roles`, `StartAt`, `EndAt`) to ensure it matches your expectations.
  3. Use the `queries.WidgetAreaQuery` with a controlled `ViewerContext` to reproduce the problem outside of transports.
- **Fix**: Implement a custom `Authorizer` that cross-checks both go-cms visibility settings and your application-specific permissions.

## Widget Provider Debugging
- **Symptoms**: Widget renders but data payloads are empty/stale.
- **Checks**:
  1. Enable the telemetry hook and look for `dashboard.widget.provider_error` events emitted in `Service.attachProviderData`.
  2. Ensure your provider function is registered via `ProviderRegistry.RegisterProvider` before `dashboard.Service` resolves layouts.
  3. Use the `examples/goadmin` sample as a reference for registering definitions and providers in lockstep.
- **Fix**: Wrap provider logic with richer logging/telemetry, and return detailed errors so transports can surface them in development environments.
