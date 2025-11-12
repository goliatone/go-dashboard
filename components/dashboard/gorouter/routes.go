package gorouter

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"

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
}

// Register mounts dashboard routes (HTML, JSON, REST, WebSocket) on a go-router router.
func Register[T any](cfg Config[T]) error {
	if cfg.Router == nil {
		return errors.New("gorouter: router is required")
	}
	if cfg.Controller == nil {
		return errors.New("gorouter: controller is required")
	}
	base := cfg.BasePath
	if base == "" {
		base = "/admin"
	}
	viewerResolver := cfg.ViewerResolver
	if viewerResolver == nil {
		viewerResolver = defaultViewerResolver
	}

	group := cfg.Router.Group(base)

	group.Get("/dashboard", router.WrapHandler(func(ctx router.Context) error {
		viewer := viewerResolver(ctx)
		var buf bytes.Buffer
		if err := cfg.Controller.RenderTemplate(ctx.Context(), viewer, &buf); err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		ctx.SetHeader("Content-Type", "text/html; charset=utf-8")
		return ctx.Send(buf.Bytes())
	}))

	group.Get("/dashboard/_layout", router.WrapHandler(func(ctx router.Context) error {
		viewer := viewerResolver(ctx)
		layout, err := cfg.Controller.Render(ctx.Context(), viewer)
		if err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		return ctx.JSON(http.StatusOK, layout)
	}))

	if cfg.API != nil {
		registerAPI(group, cfg.API)
	}

	if cfg.Broadcast != nil {
		registerWebSocket(group, cfg.Broadcast)
	}

	return nil
}

func registerAPI[T any](r router.Router[T], api httpapi.Executor) {
	r.Post("/dashboard/widgets", router.WrapHandler(func(ctx router.Context) error {
		var payload dashboard.AddWidgetRequest
		if err := json.Unmarshal(ctx.Body(), &payload); err != nil {
			return respondError(ctx, http.StatusBadRequest, err)
		}
		if err := api.Assign(ctx.Context(), payload); err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		return ctx.JSON(http.StatusCreated, map[string]string{"status": "created"})
	}))

	r.Delete("/dashboard/widgets/:id", router.WrapHandler(func(ctx router.Context) error {
		id := ctx.Param("id")
		if id == "" {
			return respondError(ctx, http.StatusBadRequest, errors.New("widget id is required"))
		}
		if err := api.Remove(ctx.Context(), commands.RemoveWidgetInput{WidgetID: id}); err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		return ctx.JSON(http.StatusNoContent, map[string]string{"status": "removed"})
	}))

	r.Post("/dashboard/widgets/reorder", router.WrapHandler(func(ctx router.Context) error {
		var payload commands.ReorderWidgetsInput
		if err := json.Unmarshal(ctx.Body(), &payload); err != nil {
			return respondError(ctx, http.StatusBadRequest, err)
		}
		if err := api.Reorder(ctx.Context(), payload); err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		return ctx.JSON(http.StatusOK, map[string]string{"status": "reordered"})
	}))

	r.Post("/dashboard/widgets/refresh", router.WrapHandler(func(ctx router.Context) error {
		var payload commands.RefreshWidgetInput
		if err := json.Unmarshal(ctx.Body(), &payload); err != nil {
			return respondError(ctx, http.StatusBadRequest, err)
		}
		if err := api.Refresh(ctx.Context(), payload); err != nil {
			return respondError(ctx, http.StatusInternalServerError, err)
		}
		return ctx.JSON(http.StatusAccepted, map[string]string{"status": "queued"})
	}))
}

func registerWebSocket[T any](r router.Router[T], hook *dashboard.BroadcastHook) {
	cfg := router.DefaultWebSocketConfig()
	r.WebSocket("/dashboard/ws", cfg, func(ws router.WebSocketContext) error {
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
	if locale, ok := ctx.Locals("locale").(string); ok {
		viewer.Locale = locale
	}
	return viewer
}

func respondError(ctx router.Context, status int, err error) error {
	return ctx.JSON(status, map[string]string{"error": err.Error()})
}
