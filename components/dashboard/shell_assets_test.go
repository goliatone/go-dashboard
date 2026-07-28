package dashboard

import (
	"context"
	"io"
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
	req := httptest.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		DefaultShellAssetsPath+"shell.js",
		nil,
	)
	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected shell asset 200, got %d", resp.Code)
	}
	if !strings.Contains(resp.Body.String(), "DashboardShell") {
		t.Fatalf("expected shell runtime response, got %q", resp.Body.String())
	}
}

func TestShellCSSUsesSemanticFallbacksAndPreservesDefaults(t *testing.T) {
	file, err := ShellAssets().Open("shell.css")
	if err != nil {
		t.Fatalf("open embedded shell CSS: %v", err)
	}
	t.Cleanup(func() {
		if closeErr := file.Close(); closeErr != nil {
			t.Errorf("close embedded shell CSS: %v", closeErr)
		}
	})
	data, err := io.ReadAll(file)
	if err != nil {
		t.Fatalf("read embedded shell CSS: %v", err)
	}
	css := string(data)
	for _, want := range []string{
		`var(--dashboard-surface, var(--color-surface-canvas, #f8fafc))`,
		`var(--dashboard-card-background, var(--color-surface-raised, var(--dashboard-card, #ffffff)))`,
		`var(--dashboard-card-border, var(--color-border-default, var(--dashboard-border, #d8dee8)))`,
		`var(--dashboard-card-radius, var(--radius-surface, var(--dashboard-radius, 6px)))`,
		`var(--dashboard-card-shadow, var(--shadow-surface, var(--dashboard-shadow, 0 1px 2px rgba(15, 23, 42, 0.08))))`,
		`@media (max-width: 760px)`,
	} {
		if !strings.Contains(css, want) {
			t.Fatalf("embedded shell CSS missing semantic fallback %q", want)
		}
	}
}
