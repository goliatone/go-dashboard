# Proposal: Improve ECharts CDN Defaults for JSON API Dashboards

## Problem Statement

When using go-dashboard with JSON API transports (where chart HTML is dynamically injected via JavaScript), charts fail to render with `ERR_BLOCKED_BY_ORB` (Opaque Response Blocking) errors in the browser console.

### Root Cause

1. **go-echarts default CDN**: Uses `https://cdn.jsdelivr.net/npm/echarts@5/dist/` (from go-echarts library defaults)
2. **Browser security**: When chart HTML containing `<script src="https://cdn.jsdelivr.net/...">` is dynamically injected via JavaScript (`innerHTML`, DOM manipulation), browsers block the external scripts as a security measure
3. **CORS/ORB**: jsdelivr CDN doesn't provide adequate CORS headers for dynamically injected scripts

### Impact

- **Server-side rendered dashboards** (like `examples/goadmin`) work fine
- **JSON API dashboards** (like `go-admin` integration) fail silently with empty chart widgets
- Users must manually configure CDN in every chart provider registration

## Solution Options

### Option 1: Change Default CDN (RECOMMENDED)

Change the default CDN in `NewEChartsProvider` to use `go-echarts.github.io`:

```go
// providers_echarts.go
const defaultEChartsAssetsHost = "https://go-echarts.github.io/go-echarts-assets/assets/"

func NewEChartsProvider(chartType string, opts ...EChartsProviderOption) *EChartsProvider {
	p := &EChartsProvider{
		chartType:  strings.ToLower(chartType),
		cache:      sharedChartCache,
		theme:      types.ThemeWesteros,
		showTitle:  false,
		assetsHost: defaultEChartsAssetsHost, // ADD THIS
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}
```

**Pros:**
- Works for both server-side and client-side rendering
- No breaking changes (can still override with `WithChartAssetsHost`)
- Matches the working example in `examples/goadmin`

**Cons:**
- Slight behavior change (but fixes broken use case)
- Adds dependency on `go-echarts.github.io` availability

### Option 2: Environment Variable Default

Allow configuration via environment variable:

```go
func defaultAssetsHost() string {
	if host := os.Getenv("GO_DASHBOARD_ECHARTS_CDN"); host != "" {
		return host
	}
	return "https://go-echarts.github.io/go-echarts-assets/assets/"
}
```

**Pros:**
- Flexible for different deployment scenarios
- Can switch to local/self-hosted CDN easily

**Cons:**
- More complex
- Still requires setting env var for out-of-box experience

### Option 3: Document Only (Status Quo)

Keep current behavior, improve documentation.

**Pros:**
- No code changes

**Cons:**
- Users hit this issue repeatedly
- Poor out-of-box experience for JSON API dashboards

## Recommendation

**Implement Option 1** with the following changes:

1. Add default CDN constant in `providers_echarts.go`
2. Update documentation to explain the choice
3. Add troubleshooting section for custom CDN scenarios

### Migration Path

No migration needed - this is a fix for a broken use case. Users who explicitly set `WithChartAssetsHost` are unaffected.

## Testing

- Verify `examples/goadmin` still works (server-side rendering)
- Verify go-admin integration works (JSON API + dynamic injection)
- Add test case for CDN override behavior

## Documentation Updates

- [x] Updated `docs/ECHARTS_WIDGETS.md` with CORS/ORB section
- [ ] Add note to README about CDN configuration
- [ ] Update `examples/goadmin` comments to explain CDN choice

## Related Issues

This issue was discovered while integrating go-dashboard with go-admin's JSON API architecture. The problem manifests as:

```
westeros.js (failed) net::ERR_BLOCKED_BY_ORB
wonderland.js (failed) net::ERR_BLOCKED_BY_ORB
Uncaught ReferenceError: echarts is not defined
```

## Implementation Checklist

- [ ] Add `defaultEChartsAssetsHost` constant
- [ ] Set default in `NewEChartsProvider`
- [ ] Add test for default CDN behavior
- [ ] Add test for CDN override
- [ ] Update README with CDN section
- [ ] Update examples/goadmin comments
- [ ] Consider environment variable option for v2

## Open Questions

1. Should we support fallback CDN if primary fails?
2. Should we provide a way to disable CDN entirely (inline ECharts)?
3. Should we validate CDN reachability at startup?

---

**Author:** Analysis based on go-admin integration experience
**Date:** 2025-12-01
**Status:** Proposed
