package gorouter

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	router "github.com/goliatone/go-router"

	"github.com/goliatone/go-dashboard/components/dashboard"
	"github.com/goliatone/go-dashboard/components/dashboard/commands"
)

func TestRegisterValidatesConfig(t *testing.T) {
	err := Register(Config[struct{}]{})
	if err == nil {
		t.Fatalf("expected error when router/controller missing")
	}
}

func TestRegisterHTMLRoute(t *testing.T) {
	server := router.NewFiberAdapter()
	appRouter := server.Router()
	layout := dashboard.Layout{
		Areas: map[string][]dashboard.WidgetInstance{
			"admin.dashboard.main": {
				{ID: "w1", DefinitionID: "admin.widget.user_stats"},
			},
		},
	}
	service := &stubLayoutResolver{layout: layout}
	renderer := &stubRenderer{}
	controller := dashboard.NewController(dashboard.ControllerOptions{
		Service:  service,
		Renderer: renderer,
	})

	cfg := Config[*fiber.App]{
		Router:     appRouter,
		Controller: controller,
		API:        noopExecutor{},
		ViewerResolver: func(router.Context) dashboard.ViewerContext {
			return dashboard.ViewerContext{UserID: "tester"}
		},
	}
	if err := Register(cfg); err != nil {
		t.Fatalf("register returned error: %v", err)
	}

	fiberAdapter, ok := server.(interface {
		WrappedRouter() *fiber.App
	})
	if !ok {
		t.Fatalf("adapter does not expose wrapped router")
	}
	resp, err := fiberAdapter.WrappedRouter().Test(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/dashboard", nil))
	if err != nil {
		t.Fatalf("fiber app test: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if renderer.calls == 0 {
		t.Fatalf("renderer not invoked")
	}

	prefReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/dashboard/preferences", bytes.NewBufferString(`{"area_order":{"admin.dashboard.main":["w1"]}}`))
	prefResp, err := fiberAdapter.WrappedRouter().Test(prefReq)
	if err != nil {
		t.Fatalf("preferences request failed: %v", err)
	}
	defer func() {
		if closeErr := prefResp.Body.Close(); closeErr != nil {
			t.Errorf("close preferences response body: %v", closeErr)
		}
	}()
	if prefResp.StatusCode != http.StatusOK {
		t.Fatalf("expected preferences 200, got %d", prefResp.StatusCode)
	}
}

func TestAssetsRouteServesEmbeddedFiles(t *testing.T) {
	server := router.NewFiberAdapter()
	appRouter := server.Router()
	layout := dashboard.Layout{
		Areas: map[string][]dashboard.WidgetInstance{
			"admin.dashboard.main": nil,
		},
	}
	service := &stubLayoutResolver{layout: layout}
	controller := dashboard.NewController(dashboard.ControllerOptions{
		Service:  service,
		Renderer: &stubRenderer{},
	})
	cfg := Config[*fiber.App]{
		Router:     appRouter,
		Controller: controller,
		API:        noopExecutor{},
	}
	if err := Register(cfg); err != nil {
		t.Fatalf("register returned error: %v", err)
	}
	fiberAdapter, ok := server.(interface {
		WrappedRouter() *fiber.App
	})
	if !ok {
		t.Fatalf("adapter does not expose wrapped router")
	}
	resp, err := fiberAdapter.WrappedRouter().Test(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/dashboard/assets/echarts/echarts.min.js", nil))
	if err != nil {
		t.Fatalf("asset request failed: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close asset response body: %v", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected embedded assets to be served, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct == "" {
		t.Fatalf("expected content type for asset response")
	}

	shellResp, err := fiberAdapter.WrappedRouter().Test(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/dashboard/assets/shell/shell.css", nil))
	if err != nil {
		t.Fatalf("shell asset request failed: %v", err)
	}
	defer func() {
		if closeErr := shellResp.Body.Close(); closeErr != nil {
			t.Errorf("close shell asset response body: %v", closeErr)
		}
	}()
	if shellResp.StatusCode != http.StatusOK {
		t.Fatalf("expected embedded shell assets to be served, got %d", shellResp.StatusCode)
	}
	if shellResp.Header.Get("Content-Type") == "" {
		t.Fatalf("expected content type for shell asset response")
	}
}

func TestRegisterAllowsExternallyManagedAssets(t *testing.T) {
	server := router.NewFiberAdapter()
	controller := dashboard.NewController(dashboard.ControllerOptions{
		Service: &stubLayoutResolver{layout: dashboard.Layout{
			Areas: map[string][]dashboard.WidgetInstance{"admin.dashboard.main": nil},
		}},
		Renderer: &stubRenderer{},
	})

	if err := Register(Config[*fiber.App]{
		Router:            server.Router(),
		Controller:        controller,
		API:               noopExecutor{},
		AssetRegistration: AssetRegistrationModeExternal,
	}); err != nil {
		t.Fatalf("register returned error: %v", err)
	}

	fiberAdapter, ok := server.(interface{ WrappedRouter() *fiber.App })
	if !ok {
		t.Fatalf("adapter does not expose wrapped router")
	}
	app := fiberAdapter.WrappedRouter()

	dashboardResp, err := app.Test(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/dashboard", nil))
	if err != nil {
		t.Fatalf("dashboard request failed: %v", err)
	}
	defer func() {
		if closeErr := dashboardResp.Body.Close(); closeErr != nil {
			t.Errorf("close dashboard response body: %v", closeErr)
		}
	}()
	if dashboardResp.StatusCode != http.StatusOK {
		t.Fatalf("expected dashboard route to remain registered, got %d", dashboardResp.StatusCode)
	}

	for _, assetPath := range []string{
		"/dashboard/assets/echarts/echarts.min.js",
		"/dashboard/assets/shell/shell.css",
	} {
		resp, err := app.Test(httptest.NewRequestWithContext(t.Context(), http.MethodGet, assetPath, nil))
		if err != nil {
			t.Fatalf("asset request %s failed: %v", assetPath, err)
		}
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Errorf("close asset response body: %v", closeErr)
			}
		}()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected externally managed asset %s to remain unmounted, got %d", assetPath, resp.StatusCode)
		}
	}
}

func TestRegisterCanUseSeparateAssetRouter(t *testing.T) {
	server := router.NewFiberAdapter()
	rootRouter := server.Router()
	controller := dashboard.NewController(dashboard.ControllerOptions{
		Service: &stubLayoutResolver{layout: dashboard.Layout{
			Areas: map[string][]dashboard.WidgetInstance{"admin.dashboard.main": nil},
		}},
		Renderer: &stubRenderer{},
	})

	if err := Register(Config[*fiber.App]{
		Router:      rootRouter.Group("/admin"),
		AssetRouter: rootRouter,
		Controller:  controller,
		API:         noopExecutor{},
		BasePath:    "/",
	}); err != nil {
		t.Fatalf("register returned error: %v", err)
	}

	fiberAdapter, ok := server.(interface{ WrappedRouter() *fiber.App })
	if !ok {
		t.Fatalf("adapter does not expose wrapped router")
	}
	app := fiberAdapter.WrappedRouter()

	for target, wantStatus := range map[string]int{
		"/admin/dashboard":                         http.StatusOK,
		"/dashboard/assets/echarts/echarts.min.js": http.StatusOK,
		"/dashboard/assets/shell/shell.css":        http.StatusOK,
		"/admin/dashboard/assets/shell/shell.css":  http.StatusNotFound,
	} {
		resp, err := app.Test(httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, nil))
		if err != nil {
			t.Fatalf("request %s failed: %v", target, err)
		}
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				t.Errorf("close response body: %v", closeErr)
			}
		}()
		if resp.StatusCode != wantStatus {
			t.Fatalf("expected %s to return %d, got %d", target, wantStatus, resp.StatusCode)
		}
	}
}

func TestRegisterRejectsUnknownAssetRegistrationMode(t *testing.T) {
	server := router.NewFiberAdapter()
	controller := dashboard.NewController(dashboard.ControllerOptions{
		Service:  &stubLayoutResolver{layout: dashboard.Layout{Areas: map[string][]dashboard.WidgetInstance{}}},
		Renderer: &stubRenderer{},
	})

	err := Register(Config[*fiber.App]{
		Router:            server.Router(),
		Controller:        controller,
		AssetRegistration: AssetRegistrationMode(255),
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported asset registration mode") {
		t.Fatalf("expected unsupported asset registration error, got %v", err)
	}
}

func TestDefaultViewerResolverUsesAcceptLanguage(t *testing.T) {
	server := router.NewFiberAdapter()
	appRouter := server.Router()
	layout := dashboard.Layout{
		Areas: map[string][]dashboard.WidgetInstance{
			"admin.dashboard.main": nil,
		},
	}
	service := &stubLayoutResolver{layout: layout}
	controller := dashboard.NewController(dashboard.ControllerOptions{
		Service:  service,
		Renderer: &stubRenderer{},
	})
	cfg := Config[*fiber.App]{
		Router:     appRouter,
		Controller: controller,
		API:        noopExecutor{},
	}
	if err := Register(cfg); err != nil {
		t.Fatalf("register returned error: %v", err)
	}
	fiberAdapter, ok := server.(interface {
		WrappedRouter() *fiber.App
	})
	if !ok {
		t.Fatalf("adapter does not expose wrapped router")
	}
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/dashboard/_layout", nil)
	req.Header.Set("Accept-Language", "es-MX,es;q=0.9,en;q=0.8")
	resp, err := fiberAdapter.WrappedRouter().Test(req)
	if err != nil {
		t.Fatalf("layout request failed: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close layout response body: %v", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected layout 200, got %d", resp.StatusCode)
	}
	if service.lastViewer.Locale != "es-mx" {
		t.Fatalf("expected locale inferred from Accept-Language, got %q", service.lastViewer.Locale)
	}

	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode layout response: %v", err)
	}
	if _, hasOrderedAreas := payload["ordered_areas"]; hasOrderedAreas {
		t.Fatalf("expected JSON route to return typed page payload, got %+v", payload)
	}
	areas, ok := payload["areas"].([]any)
	if !ok || len(areas) == 0 {
		t.Fatalf("expected typed page areas array, got %+v", payload["areas"])
	}
}

func TestRegisterWithCustomRoutes(t *testing.T) {
	server := router.NewFiberAdapter()
	appRouter := server.Router()
	layout := dashboard.Layout{
		Areas: map[string][]dashboard.WidgetInstance{
			"admin.dashboard.main": {
				{ID: "w1", DefinitionID: "admin.widget.user_stats"},
			},
		},
	}
	service := &stubLayoutResolver{layout: layout}
	controller := dashboard.NewController(dashboard.ControllerOptions{
		Service:  service,
		Renderer: &stubRenderer{},
	})

	cfg := Config[*fiber.App]{
		Router:     appRouter,
		Controller: controller,
		API:        noopExecutor{},
		BasePath:   "/console",
		Routes: RouteConfig{
			HTML:        "/home",
			Layout:      "/home/layout.json",
			Widgets:     "/widgets",
			WidgetID:    "/widgets/:id",
			Reorder:     "/widgets/order",
			Refresh:     "/widgets/refresh",
			Preferences: "/prefs",
			WebSocket:   "/ws/live",
			ShellAssets: "/assets/shell/",
		},
	}
	if err := Register(cfg); err != nil {
		t.Fatalf("register returned error: %v", err)
	}

	fiberAdapter, ok := server.(interface {
		WrappedRouter() *fiber.App
	})
	if !ok {
		t.Fatalf("adapter does not expose wrapped router")
	}

	resp, err := fiberAdapter.WrappedRouter().Test(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/console/home", nil))
	if err != nil {
		t.Fatalf("fiber app test: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close response body: %v", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for custom HTML route, got %d", resp.StatusCode)
	}

	prefReq := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/console/prefs", bytes.NewBufferString(`{"area_order":{"admin.dashboard.main":["w1"]}}`))
	prefResp, err := fiberAdapter.WrappedRouter().Test(prefReq)
	if err != nil {
		t.Fatalf("preferences request failed: %v", err)
	}
	defer func() {
		if closeErr := prefResp.Body.Close(); closeErr != nil {
			t.Errorf("close preferences response body: %v", closeErr)
		}
	}()
	if prefResp.StatusCode != http.StatusOK {
		t.Fatalf("expected custom preferences route to return 200, got %d", prefResp.StatusCode)
	}

	legacyResp, err := fiberAdapter.WrappedRouter().Test(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/admin/dashboard", nil))
	if err != nil {
		t.Fatalf("legacy route request failed: %v", err)
	}
	defer func() {
		if closeErr := legacyResp.Body.Close(); closeErr != nil {
			t.Errorf("close legacy response body: %v", closeErr)
		}
	}()
	if legacyResp.StatusCode == http.StatusOK {
		t.Fatalf("expected default route to be unmapped when custom routes used")
	}

	shellResp, err := fiberAdapter.WrappedRouter().Test(httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/assets/shell/shell.js", nil))
	if err != nil {
		t.Fatalf("custom shell asset request failed: %v", err)
	}
	defer func() {
		if closeErr := shellResp.Body.Close(); closeErr != nil {
			t.Errorf("close shell response body: %v", closeErr)
		}
	}()
	if shellResp.StatusCode != http.StatusOK {
		t.Fatalf("expected custom shell assets route to return 200, got %d", shellResp.StatusCode)
	}
}

func TestPreferencesRouteAcceptsLegacyLayoutCompatibilityPayload(t *testing.T) {
	server := router.NewFiberAdapter()
	appRouter := server.Router()
	service := &stubLayoutResolver{layout: dashboard.Layout{Areas: map[string][]dashboard.WidgetInstance{}}}
	controller := dashboard.NewController(dashboard.ControllerOptions{
		Service:  service,
		Renderer: &stubRenderer{},
	})

	cfg := Config[*fiber.App]{
		Router:     appRouter,
		Controller: controller,
		API:        noopExecutor{},
	}
	if err := Register(cfg); err != nil {
		t.Fatalf("register returned error: %v", err)
	}

	fiberAdapter, ok := server.(interface {
		WrappedRouter() *fiber.App
	})
	if !ok {
		t.Fatalf("adapter does not expose wrapped router")
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/admin/dashboard/preferences", bytes.NewBufferString(`{"layout":[{"id":"w1","area":"admin.dashboard.main","position":0,"span":6}]}`))
	resp, err := fiberAdapter.WrappedRouter().Test(req)
	if err != nil {
		t.Fatalf("preferences request failed: %v", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			t.Errorf("close preferences response body: %v", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected legacy compatibility payload to return 200, got %d", resp.StatusCode)
	}
}

type stubLayoutResolver struct {
	layout     dashboard.Layout
	err        error
	lastViewer dashboard.ViewerContext
}

func (s *stubLayoutResolver) ConfigureLayout(ctx context.Context, viewer dashboard.ViewerContext) (dashboard.Layout, error) {
	s.lastViewer = viewer
	return s.layout, s.err
}

type stubRenderer struct {
	calls int
}

func (s *stubRenderer) RenderPage(name string, page dashboard.Page, out ...io.Writer) (string, error) {
	s.calls++
	if len(out) > 0 && out[0] != nil {
		if _, err := out[0].Write([]byte("ok")); err != nil {
			return "", err
		}
	}
	return "ok", nil
}

type noopExecutor struct{}

func (noopExecutor) Assign(context.Context, dashboard.AddWidgetRequest) error    { return nil }
func (noopExecutor) Remove(context.Context, commands.RemoveWidgetInput) error    { return nil }
func (noopExecutor) Reorder(context.Context, commands.ReorderWidgetsInput) error { return nil }
func (noopExecutor) Refresh(context.Context, commands.RefreshWidgetInput) error  { return nil }
func (noopExecutor) Preferences(context.Context, commands.SaveLayoutPreferencesInput) error {
	return nil
}
