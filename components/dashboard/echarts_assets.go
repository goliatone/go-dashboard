package dashboard

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"
)

const (
	// DefaultEChartsAssetsPath is the local path used to serve embedded ECharts assets.
	DefaultEChartsAssetsPath = "/dashboard/assets/echarts/"
	// envEChartsCDN overrides the default assets host (e.g., to point at a CDN or self-hosted bucket).
	envEChartsCDN = "GO_DASHBOARD_ECHARTS_CDN"
)

//go:embed assets/echarts/* assets/echarts/themes/*
var embeddedEChartsAssets embed.FS

// EChartsAssetsFS exposes the embedded ECharts runtime and themes.
func EChartsAssetsFS() http.FileSystem {
	sub, err := fs.Sub(embeddedEChartsAssets, "assets/echarts")
	if err != nil {
		// This should never happen because the directory is embedded at build time.
		panic(fmt.Errorf("dashboard: failed to prepare embedded ECharts assets: %w", err))
	}
	return http.FS(sub)
}

// EChartsAssetsHandler returns an http.Handler that serves the embedded assets from the given prefix.
func EChartsAssetsHandler(prefix string) http.Handler {
	if prefix == "" {
		prefix = DefaultEChartsAssetsPath
	}
	prefix = ensureTrailingSlash(prefix)
	return http.StripPrefix(prefix, http.FileServer(EChartsAssetsFS()))
}

// DefaultEChartsAssetsHost returns the default assets host, respecting GO_DASHBOARD_ECHARTS_CDN if set.
func DefaultEChartsAssetsHost() string {
	if host := strings.TrimSpace(os.Getenv(envEChartsCDN)); host != "" {
		return ensureTrailingSlash(host)
	}
	return DefaultEChartsAssetsPath
}

func ensureTrailingSlash(value string) string {
	if value == "" {
		return ""
	}
	if strings.HasSuffix(value, "/") {
		return value
	}
	return value + "/"
}
