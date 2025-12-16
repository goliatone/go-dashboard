package gorouter

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	router "github.com/goliatone/go-router"

	"github.com/goliatone/go-dashboard/components/dashboard"
	"github.com/goliatone/go-dashboard/components/dashboard/commands"
	"github.com/goliatone/go-dashboard/components/dashboard/httpapi"
)

// ViewerResolver converts a router.Context into a dashboard.ViewerContext.
type ViewerResolver func(router.Context) dashboard.ViewerContext

// Config wires go-router with go-dashboard controllers, APIs, and hooks.
type Config[T any] struct {
	Router         router.Router[T]
	Controller     *dashboard.Controller
	API            httpapi.Executor
	Broadcast      *dashboard.BroadcastHook
	ViewerResolver ViewerResolver
	BasePath       string
	Routes         RouteConfig
}

// RouteConfig customizes the relative paths used for dashboard endpoints.
type RouteConfig struct {
	HTML        string
	Layout      string
	Widgets     string
	WidgetID    string
	Reorder     string
	Refresh     string
	Preferences string
	WebSocket   string
	Assets      string
}

// Register mounts dashboard routes (HTML, JSON, REST, WebSocket) on a go-router router.
func Register[T any](cfg Config[T]) error {
	if cfg.Router == nil {
		return errors.New("gorouter: router is required")
	}
	if cfg.Controller == nil {
		return errors.New("gorouter: controller is required")
	}
	routes := cfg.routes()
	base := cfg.BasePath
	if base == "" {
		base = "/admin"
	}
	viewerResolver := cfg.ViewerResolver
	if viewerResolver == nil {
		viewerResolver = defaultViewerResolver
	}

	if routes.Assets != "" {
		cfg.Router.Static(routes.Assets, ".", router.Static{
			FS:     dashboard.EChartsAssets(),
			Root:   ".",
			MaxAge: 86400,
		})
	}

	group := cfg.Router.Group(base)

	group.Get(routes.HTML, router.WrapHandler(func(ctx router.Context) error {
		viewer := viewerResolver(ctx)
		var buf bytes.Buffer
		if err := cfg.Controller.RenderTemplate(ctx.Context(), viewer, &buf); err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		ctx.SetHeader("Content-Type", "text/html; charset=utf-8")
		return ctx.Send(buf.Bytes())
	}))

	group.Get(routes.Layout, router.WrapHandler(func(ctx router.Context) error {
		viewer := viewerResolver(ctx)
		payload, err := cfg.Controller.LayoutPayload(ctx.Context(), viewer)
		if err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		return ctx.JSON(http.StatusOK, payload)
	}))

	if cfg.API != nil {
		registerAPI(group, cfg.API, viewerResolver, routes)
	}

	if cfg.Broadcast != nil {
		registerWebSocket(group, cfg.Broadcast, routes.WebSocket)
	}

	return nil
}

func registerAPI[T any](r router.Router[T], api httpapi.Executor, resolver ViewerResolver, routes RouteConfig) {
	r.Post(routes.Widgets, router.WrapHandler(func(ctx router.Context) error {
		var payload dashboard.AddWidgetRequest
		if err := json.Unmarshal(ctx.Body(), &payload); err != nil {
			return respondError(ctx, http.StatusBadRequest, err)
		}
		if err := api.Assign(ctx.Context(), payload); err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		return ctx.JSON(http.StatusCreated, map[string]string{"status": "created"})
	}))

	r.Delete(routes.WidgetID, router.WrapHandler(func(ctx router.Context) error {
		id := ctx.Param("id")
		if id == "" {
			return respondError(ctx, http.StatusBadRequest, errors.New("widget id is required"))
		}
		if err := api.Remove(ctx.Context(), commands.RemoveWidgetInput{WidgetID: id}); err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		return ctx.JSON(http.StatusNoContent, map[string]string{"status": "removed"})
	}))

	r.Post(routes.Reorder, router.WrapHandler(func(ctx router.Context) error {
		var payload commands.ReorderWidgetsInput
		if err := json.Unmarshal(ctx.Body(), &payload); err != nil {
			return respondError(ctx, http.StatusBadRequest, err)
		}
		if err := api.Reorder(ctx.Context(), payload); err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		return ctx.JSON(http.StatusOK, map[string]string{"status": "reordered"})
	}))

	r.Post(routes.Refresh, router.WrapHandler(func(ctx router.Context) error {
		var payload commands.RefreshWidgetInput
		if err := json.Unmarshal(ctx.Body(), &payload); err != nil {
			return respondError(ctx, http.StatusBadRequest, err)
		}
		if err := api.Refresh(ctx.Context(), payload); err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		return ctx.JSON(http.StatusAccepted, map[string]string{"status": "queued"})
	}))

	r.Post(routes.Preferences, router.WrapHandler(func(ctx router.Context) error {
		var payload commands.SaveLayoutPreferencesInput
		if err := json.Unmarshal(ctx.Body(), &payload); err != nil {
			return respondError(ctx, http.StatusBadRequest, err)
		}
		payload.Viewer = resolver(ctx)
		if err := api.Preferences(ctx.Context(), payload); err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		return ctx.JSON(http.StatusOK, map[string]string{"status": "saved"})
	}))
}

func registerWebSocket[T any](r router.Router[T], hook *dashboard.BroadcastHook, path string) {
	cfg := router.DefaultWebSocketConfig()
	r.WebSocket(path, cfg, func(ws router.WebSocketContext) error {
		events, cancel := hook.Subscribe()
		defer cancel()
		for {
			select {
			case event, ok := <-events:
				if !ok {
					return nil
				}
				if err := ws.WriteJSON(event); err != nil {
					return err
				}
			case <-ws.Context().Done():
				return ws.Close()
			}
		}
	})
}

func defaultViewerResolver(ctx router.Context) dashboard.ViewerContext {
	var viewer dashboard.ViewerContext
	if v, ok := ctx.Locals("user_id").(string); ok {
		viewer.UserID = v
	}
	if roles, ok := ctx.Locals("roles").([]string); ok {
		viewer.Roles = roles
	}
	viewer.Locale = inferLocale(ctx)
	return viewer
}

func inferLocale(ctx router.Context) string {
	if locale, ok := ctx.Locals("locale").(string); ok && locale != "" {
		return locale
	}
	if locale := strings.TrimSpace(ctx.Param("locale")); locale != "" {
		return strings.ToLower(locale)
	}
	if locale := strings.TrimSpace(ctx.Query("locale")); locale != "" {
		return strings.ToLower(locale)
	}
	if header := ctx.Header("Accept-Language"); header != "" {
		if lang := parseAcceptLanguage(header); lang != "" {
			return lang
		}
	}
	return ""
}

func parseAcceptLanguage(header string) string {
	for _, token := range strings.Split(header, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if idx := strings.Index(token, ";"); idx >= 0 {
			token = token[:idx]
		}
		if token != "" {
			return strings.ToLower(token)
		}
	}
	return ""
}

func respondError(ctx router.Context, status int, err error) error {
	return ctx.JSON(status, map[string]string{"error": err.Error()})
}

func (cfg Config[T]) routes() RouteConfig {
	routes := defaultRouteConfig(cfg.Routes)
	return routes
}

func defaultRouteConfig(routes RouteConfig) RouteConfig {
	if routes.HTML == "" {
		routes.HTML = "/dashboard"
	}
	if routes.Layout == "" {
		routes.Layout = "/dashboard/_layout"
	}
	if routes.Widgets == "" {
		routes.Widgets = "/dashboard/widgets"
	}
	if routes.WidgetID == "" {
		routes.WidgetID = "/dashboard/widgets/:id"
	}
	if routes.Reorder == "" {
		routes.Reorder = "/dashboard/widgets/reorder"
	}
	if routes.Refresh == "" {
		routes.Refresh = "/dashboard/widgets/refresh"
	}
	if routes.Preferences == "" {
		routes.Preferences = "/dashboard/preferences"
	}
	if routes.WebSocket == "" {
		routes.WebSocket = "/dashboard/ws"
	}
	if routes.Assets == "" {
		routes.Assets = dashboard.DefaultEChartsAssetsPath
	}
	return routes
}
