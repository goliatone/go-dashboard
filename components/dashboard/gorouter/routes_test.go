package gorouter

import (
	"context"
	"encoding/json"
	"io"
	"testing"

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
	mock := newMockRouter()
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

	cfg := Config[struct{}]{
		Router:     mock,
		Controller: controller,
		API:        noopExecutor{},
	}
	if err := Register(cfg); err != nil {
		t.Fatalf("register returned error: %v", err)
	}

	handlerKey := "GET:/admin/dashboard"
	h, ok := mock.routes[handlerKey]
	if !ok {
		t.Fatalf("expected dashboard route to be registered")
	}

	ctx := newMockContext()
	if err := h(ctx); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if len(ctx.body) == 0 {
		t.Fatalf("expected response body")
	}
	if renderer.calls == 0 {
		t.Fatalf("renderer not invoked")
	}
}

// --- Test helpers ---

type mockRouter struct {
	prefix string
	routes map[string]router.HandlerFunc
	ws     map[string]func(router.WebSocketContext) error
}

func newMockRouter() *mockRouter {
	return &mockRouter{
		routes: map[string]router.HandlerFunc{},
		ws:     map[string]func(router.WebSocketContext) error{},
	}
}

func (m *mockRouter) Group(prefix string) router.Router[struct{}] {
	return &mockRouter{
		prefix: m.prefix + prefix,
		routes: m.routes,
		ws:     m.ws,
	}
}

func (m *mockRouter) record(method, path string, handler router.HandlerFunc) {
	full := m.prefix + path
	m.routes[method+":"+full] = handler
}

func (m *mockRouter) Get(path string, handler router.HandlerFunc, mw ...router.MiddlewareFunc) router.RouteInfo {
	m.record(string(router.GET), path, handler)
	return mockRouteInfo{}
}

func (m *mockRouter) Post(path string, handler router.HandlerFunc, mw ...router.MiddlewareFunc) router.RouteInfo {
	m.record(string(router.POST), path, handler)
	return mockRouteInfo{}
}

func (m *mockRouter) Delete(path string, handler router.HandlerFunc, mw ...router.MiddlewareFunc) router.RouteInfo {
	m.record(string(router.DELETE), path, handler)
	return mockRouteInfo{}
}

func (m *mockRouter) WebSocket(path string, cfg router.WebSocketConfig, handler func(router.WebSocketContext) error) router.RouteInfo {
	full := m.prefix + path
	m.ws[full] = handler
	return mockRouteInfo{}
}

type mockRouteInfo struct{}

func (mockRouteInfo) SetName(string) router.RouteInfo { return mockRouteInfo{} }

type mockContext struct {
	ctx     context.Context
	headers map[string]string
	body    []byte
	locals  map[any]any
	params  map[string]string
	status  int
}

func newMockContext() *mockContext {
	return &mockContext{
		ctx:     context.Background(),
		headers: map[string]string{},
		locals:  map[any]any{},
		params:  map[string]string{},
	}
}

func (m *mockContext) Context() context.Context {
	return m.ctx
}

func (m *mockContext) SetHeader(k, v string) router.Context {
	m.headers[k] = v
	return m
}

func (m *mockContext) Send(b []byte) error {
	m.body = append([]byte{}, b...)
	return nil
}

func (m *mockContext) JSON(code int, v any) error {
	m.status = code
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	m.body = data
	return nil
}

func (m *mockContext) Body() []byte { return m.body }

func (m *mockContext) Param(name string, defaultValue ...string) string {
	if v, ok := m.params[name]; ok {
		return v
	}
	if len(defaultValue) > 0 {
		return defaultValue[0]
	}
	return ""
}

func (m *mockContext) Locals(key any, value ...any) any {
	if len(value) == 0 {
		return m.locals[key]
	}
	m.locals[key] = value[0]
	return value[0]
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

func (noopExecutor) Assign(context.Context, dashboard.AddWidgetRequest) error            { return nil }
func (noopExecutor) Remove(context.Context, commands.RemoveWidgetInput) error           { return nil }
func (noopExecutor) Reorder(context.Context, commands.ReorderWidgetsInput) error        { return nil }
func (noopExecutor) Refresh(context.Context, commands.RefreshWidgetInput) error         { return nil }
