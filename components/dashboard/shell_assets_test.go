package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestShellPageAssetsAddsDefaultsOnce(t *testing.T) {
	assets := PageAssets{}
	assets.AddShellAssets("")
	assets.AddShellAssets("")

	if len(assets.CSS) != 1 || assets.CSS[0] != DefaultShellAssetsPath+"shell.css" {
		t.Fatalf("expected shell css once, got %+v", assets.CSS)
	}
	if len(assets.JS) != 1 || assets.JS[0] != DefaultShellAssetsPath+"shell.js" {
		t.Fatalf("expected shell js once, got %+v", assets.JS)
	}
}

func TestShellAssetsHandlerServesEmbeddedRuntime(t *testing.T) {
	handler := ShellAssetsHandler(DefaultShellAssetsPath)
	req := httptest.NewRequest(http.MethodGet, DefaultShellAssetsPath+"shell.js", nil)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected shell asset 200, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "DashboardShell") {
		t.Fatalf("expected shell runtime response, got %q", resp.Body.String())
	}
}
