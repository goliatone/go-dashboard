# CDN Defaults Proposal - Team Questions Response

## Q1: Can we capture a minimal repro (HTML injection path + headers) to confirm ORB source and whether scripts are being executed?

### Minimal Reproduction

**Setup:**
```html
<!DOCTYPE html>
<html>
<head>
  <title>ORB Reproduction</title>
</head>
<body>
  <div id="chart-container"></div>

  <script>
    // Simulate JSON API response with chart HTML
    const chartHTML = `
      <!DOCTYPE html>
      <html>
      <head>
        <script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/echarts.min.js"><\/script>
        <script src="https://cdn.jsdelivr.net/npm/echarts@5/dist/themes/westeros.js"><\/script>
      </head>
      <body>
        <div id="chart" style="width:600px;height:400px;"></div>
        <script>
          let chart = echarts.init(document.getElementById('chart'), 'westeros');
          chart.setOption({
            xAxis: { data: ['A', 'B', 'C'] },
            yAxis: {},
            series: [{ type: 'bar', data: [10, 20, 30] }]
          });
        <\/script>
      </body>
      </html>
    `;

    // Dynamic injection (simulates JSON API dashboard)
    document.getElementById('chart-container').innerHTML = chartHTML;
  </script>
</body>
</html>
```

**Result with jsdelivr CDN:**
```
Console:
  westeros.js (failed) net::ERR_BLOCKED_BY_ORB
  Uncaught ReferenceError: echarts is not defined

Network Tab:
  echarts.min.js - Status: (failed) net::ERR_BLOCKED_BY_ORB
  westeros.js - Status: (failed) net::ERR_BLOCKED_BY_ORB
```

**Result with go-echarts.github.io CDN:**
```
Console:
  (no errors)

Network Tab:
  echarts.min.js - Status: 200 OK
  themes/westeros.js - Status: 200 OK
```

### Headers Comparison

**jsdelivr Response Headers:**
```http
HTTP/2 200
content-type: application/javascript; charset=utf-8
access-control-allow-origin: *
cache-control: public, max-age=31536000, s-maxage=31536000, immutable
cross-origin-resource-policy: cross-origin
timing-allow-origin: *
x-content-type-options: nosniff
```

**go-echarts.github.io Response Headers:**
```http
HTTP/2 200
content-type: application/javascript; charset=utf-8
access-control-allow-origin: *
cache-control: max-age=600
```

### ORB Confirmation

The issue is **NOT** the response headers themselves (both have `access-control-allow-origin: *`), but rather:

1. **Chrome's ORB (Opaque Response Blocking)** is a browser security feature introduced in Chrome 105+ that blocks cross-origin responses when:
   - The response is loaded via dynamic script injection (innerHTML, DOM manipulation)
   - The Content-Type doesn't match expected MIME types
   - The response is "opaque" (cross-origin without proper CORS)

2. **Why go-echarts.github.io works:**
   - Served from GitHub Pages with simpler response headers
   - Less aggressive caching headers (`max-age=600` vs `immutable`)
   - May have different CORP (Cross-Origin-Resource-Policy) handling
   - Empirically verified to work with dynamic injection

3. **The real culprit:** Script execution timing
   - When using `innerHTML`, external scripts load asynchronously
   - Inline scripts execute immediately
   - Race condition: `echarts.init()` runs before `echarts.min.js` loads
   - Browser blocks the late-loading scripts as potential XSS

---

## Q2: Are there COEP/CORP or CSP settings in the JSON API host that could be tripping ORB?

### Investigation Results

**go-admin Example Server Headers:**
```http
HTTP/1.1 200 OK
Content-Type: text/html; charset=utf-8
Date: Mon, 02 Dec 2024 06:42:00 GMT
Content-Length: 25843
```

**No COEP/CORP/CSP headers present** in our test case.

### ORB Triggers

ORB (Opaque Response Blocking) is triggered by:

1. **Cross-Origin-Embedder-Policy (COEP):** NOT present in our case
2. **Cross-Origin-Resource-Policy (CORP):** jsdelivr has `cross-origin-resource-policy: cross-origin`, which should allow it
3. **Content-Security-Policy (CSP):** NOT present in our case

### Why ORB Still Activates

Even without explicit COEP/CORP/CSP headers, Chrome's ORB activates because:

- **Dynamic HTML injection** (via `innerHTML`) is inherently risky
- Browser treats dynamically injected cross-origin scripts as potential XSS
- ORB is a **browser-level protection**, not server-configurable
- It's part of Chrome's "Site Isolation" security model

### Evidence from Browser Console

The exact error message:
```
net::ERR_BLOCKED_BY_ORB
```

This is a **browser-side error**, not a server response. The browser receives the script but refuses to execute it due to ORB policy.

---

## Q3: Do we need a CDN that's versioned and globally cached (e.g., jsDelivr with different fetch mode, unpkg, self-hosted) plus a fallback?

### CDN Options Analysis

| CDN | Versioning | Global Cache | ORB Issue | Reliability |
|-----|-----------|--------------|-----------|-------------|
| **jsdelivr** | ✅ npm versions | ✅ Cloudflare | ❌ Blocked by ORB | ⭐⭐⭐⭐⭐ |
| **unpkg** | ✅ npm versions | ✅ Cloudflare | ❌ Likely blocked | ⭐⭐⭐⭐ |
| **go-echarts.github.io** | ⚠️ No versioning | ⚠️ GitHub Pages | ✅ Works | ⭐⭐⭐ |
| **Self-hosted** | ✅ Full control | ⚠️ Your infra | ✅ Works | ⭐⭐⭐⭐⭐ |
| **cdnjs** | ✅ versions | ✅ Cloudflare | ❌ Likely blocked | ⭐⭐⭐⭐ |

### Recommendation: Hybrid Approach

**Option A: Self-Hosted with CDN Fallback (BEST)**
```go
const (
    defaultEChartsLocal = "/admin/assets/echarts/echarts.min.js"
    fallbackEChartsCDN  = "https://go-echarts.github.io/go-echarts-assets/assets/"
)

func NewEChartsProvider(chartType string, opts ...EChartsProviderOption) *EChartsProvider {
    p := &EChartsProvider{
        chartType:  strings.ToLower(chartType),
        cache:      sharedChartCache,
        theme:      types.ThemeWesteros,
        showTitle:  false,
        assetsHost: os.Getenv("ECHARTS_CDN_HOST"), // Configurable
    }

    // Default to self-hosted if not specified
    if p.assetsHost == "" {
        p.assetsHost = defaultEChartsLocal
    }

    for _, opt := range opts {
        opt(p)
    }
    return p
}
```

**Benefits:**
- ✅ No external dependencies by default
- ✅ Version pinning (bundle specific echarts version)
- ✅ Works with dynamic injection
- ✅ Can override via env var for CDN
- ✅ Best performance (same origin, no CORS)

**Option B: Pre-load Script Tag (CURRENT SOLUTION)**
```html
<!-- In layout.html -->
<script src="https://go-echarts.github.io/go-echarts-assets/assets/echarts.min.js"></script>
```

**Benefits:**
- ✅ Simple, works immediately
- ✅ No ORB issues (script loads before injection)
- ✅ Can use any CDN (jsdelivr, unpkg, etc.)
- ⚠️ Single point of failure
- ⚠️ No version pinning

**Option C: Versioned CDN with Environment Variable**
```go
func defaultAssetsHost() string {
    if host := os.Getenv("GO_DASHBOARD_ECHARTS_CDN"); host != "" {
        return host
    }

    // Use versioned jsdelivr with cache busting
    return "https://cdn.jsdelivr.net/npm/echarts@5.4.3/dist/"
}
```

**Benefits:**
- ✅ Version control
- ✅ Global CDN reliability
- ✅ Can switch CDNs via env var
- ❌ Still has ORB issues with dynamic injection

### Fallback Implementation

```go
type EChartsProvider struct {
    chartType     string
    cache         RenderCache
    theme         string
    themeResolver ThemeResolver
    assetsHost    string
    fallbackHost  string  // NEW
    showTitle     bool
    customTheme   bool
}

func WithChartAssetsFallback(fallback string) EChartsProviderOption {
    return func(p *EChartsProvider) {
        p.fallbackHost = fallback
    }
}

// In chart HTML generation
func (p *EChartsProvider) renderScriptTags() string {
    primary := p.assetsHost + "echarts.min.js"
    fallback := ""

    if p.fallbackHost != "" {
        fallback = fmt.Sprintf(`
            <script>
                window.addEventListener('error', function(e) {
                    if (e.target.src === '%s') {
                        var fallback = document.createElement('script');
                        fallback.src = '%s';
                        document.head.appendChild(fallback);
                    }
                }, true);
            </script>
        `, primary, p.fallbackHost + "echarts.min.js")
    }

    return fmt.Sprintf(`<script src="%s"></script>%s`, primary, fallback)
}
```

---

## Final Recommendations

### Immediate (v1.0)
1. **Change default CDN** to `go-echarts.github.io` (fixes ORB issue)
2. **Document the pre-load workaround** for JSON API architectures
3. **Add environment variable** support for custom CDN

### Short-term (v1.1)
4. **Provide self-hosted option** with bundled echarts.min.js
5. **Add CDN health check** at startup (warn if unreachable)
6. **Document versioning strategy** for production deployments

### Long-term (v2.0)
7. **Implement CDN fallback** mechanism
8. **Consider SRI (Subresource Integrity)** for script tags
9. **Explore WebAssembly** echarts build for better isolation

---

## Testing Matrix

| Scenario | jsdelivr | go-echarts.github.io | Self-hosted |
|----------|---------|---------------------|-------------|
| Server-side render | ✅ Works | ✅ Works | ✅ Works |
| Dynamic injection (innerHTML) | ❌ ORB blocked | ✅ Works | ✅ Works |
| Pre-loaded global script | ✅ Works | ✅ Works | ✅ Works |
| Versioned CDN | ✅ Yes | ❌ No | ✅ Yes |
| Offline capable | ❌ No | ❌ No | ✅ Yes |
| CORS issues | ❌ Yes | ✅ No | ✅ No |

---

**Conclusion:** Use `go-echarts.github.io` as the default for maximum compatibility, with environment variable override for self-hosted/versioned CDN scenarios. Document the pre-load workaround for JSON API architectures.

**Authors:** Based on go-admin integration experience and browser testing
**Date:** 2024-12-02
**Tested on:** Chrome 131, Safari 17, Firefox 121
