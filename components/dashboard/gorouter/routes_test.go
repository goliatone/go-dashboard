package gorouter

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
	resp, err := fiberAdapter.WrappedRouter().Test(httptest.NewRequest(http.MethodGet, "/admin/dashboard", nil))
	if err != nil {
		t.Fatalf("fiber app test: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if renderer.calls == 0 {
		t.Fatalf("renderer not invoked")
	}

	prefReq := httptest.NewRequest(http.MethodPost, "/admin/dashboard/preferences", bytes.NewBufferString(`{"area_order":{"admin.dashboard.main":["w1"]}}`))
	resp, err = fiberAdapter.WrappedRouter().Test(prefReq)
	if err != nil {
		t.Fatalf("preferences request failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected preferences 200, got %d", resp.StatusCode)
	}
}

type stubLayoutResolver struct {
	layout dashboard.Layout
	err    error
}

func (s *stubLayoutResolver) ConfigureLayout(ctx context.Context, viewer dashboard.ViewerContext) (dashboard.Layout, error) {
	return s.layout, s.err
}

type stubRenderer struct {
	calls int
}

func (s *stubRenderer) Render(name string, data any, out ...io.Writer) (string, error) {
	s.calls++
	if len(out) > 0 && out[0] != nil {
		out[0].Write([]byte("ok"))
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
