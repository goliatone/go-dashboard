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
	// DefaultShellAssetsPath is the local path used to serve embedded shell assets.
	DefaultShellAssetsPath = "/dashboard/assets/shell/"
	envShellAssetsCDN      = "GO_DASHBOARD_SHELL_ASSETS_CDN"
)

//go:embed assets/shell/shell.css assets/shell/shell.js
var embeddedShellAssets embed.FS

// ShellAssets returns the embedded shell CSS and JavaScript as an fs.FS.
func ShellAssets() fs.FS {
	sub, err := fs.Sub(embeddedShellAssets, "assets/shell")
	if err != nil {
		panic(fmt.Errorf("dashboard: failed to prepare embedded shell assets: %w", err))
	}
	return sub
}

// ShellAssetsFS exposes the embedded shell assets as an http.FileSystem.
func ShellAssetsFS() http.FileSystem {
	return http.FS(ShellAssets())
}

// ShellAssetsHandler returns an http.Handler that serves shell assets from prefix.
func ShellAssetsHandler(prefix string) http.Handler {
	if prefix == "" {
		prefix = DefaultShellAssetsPath
	}
	prefix = ensureTrailingSlash(prefix)
	return http.StripPrefix(prefix, http.FileServer(ShellAssetsFS()))
}

// DefaultShellAssetsHost returns the shell asset host, respecting GO_DASHBOARD_SHELL_ASSETS_CDN.
func DefaultShellAssetsHost() string {
	if host := strings.TrimSpace(os.Getenv(envShellAssetsCDN)); host != "" {
		return ensureTrailingSlash(host)
	}
	return DefaultShellAssetsPath
}

// ShellPageAssets returns the default shell CSS and JavaScript URLs.
func ShellPageAssets(host string) PageAssets {
	if host == "" {
		host = DefaultShellAssetsHost()
	}
	host = ensureTrailingSlash(host)
	return PageAssets{
		CSS: []string{host + "shell.css"},
		JS:  []string{host + "shell.js"},
	}
}

// AddShellAssets adds the shell CSS and JavaScript URLs to the page asset set.
func (assets *PageAssets) AddShellAssets(host string) {
	if assets == nil {
		return
	}
	shellAssets := ShellPageAssets(host)
	assets.AddCSS(shellAssets.CSS...)
	assets.AddJS(shellAssets.JS...)
}
