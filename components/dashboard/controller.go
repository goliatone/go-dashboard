package dashboard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// LayoutResolver fetches layouts for a viewer.
type LayoutResolver interface {
	ConfigureLayout(ctx context.Context, viewer ViewerContext) (Layout, error)
}

// Controller orchestrates HTTP handlers/routes for the admin dashboard.
type Controller struct {
	service  LayoutResolver
	renderer Renderer
	template string
}

// ControllerOptions configures the HTTP controller.
type ControllerOptions struct {
	Service  LayoutResolver
	Renderer Renderer
	Template string
}

// NewController wires the service and renderer into a controller.
func NewController(opts ControllerOptions) *Controller {
	templateName := opts.Template
	if templateName == "" {
		templateName = "dashboard.html"
	}
	return &Controller{
		service:  opts.Service,
		renderer: opts.Renderer,
		template: templateName,
	}
}

// Render resolves the layout for a viewer and returns it to the caller.
func (c *Controller) Render(ctx context.Context, viewer ViewerContext) (Layout, error) {
	if c == nil || c.service == nil {
		return Layout{}, fmt.Errorf("dashboard: controller missing service")
	}
	return c.service.ConfigureLayout(ctx, viewer)
}

// HandleDashboard renders the HTML dashboard page using the configured renderer.
func (c *Controller) HandleDashboard(w http.ResponseWriter, r *http.Request, viewer ViewerContext) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.RenderTemplate(r.Context(), viewer, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// HandleDashboardJSON returns the dashboard layout as JSON (useful for SPAs/tests).
func (c *Controller) HandleDashboardJSON(w http.ResponseWriter, r *http.Request, viewer ViewerContext) {
	layout, err := c.Render(r.Context(), viewer)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(layout); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (c *Controller) payloadFromLayout(layout Layout) map[string]any {
	return map[string]any{
		"title":       "Dashboard",
		"description": "Admin overview",
		"areas": map[string]any{
			"main":    c.areaPayload("admin.dashboard.main", layout.Areas["admin.dashboard.main"]),
			"sidebar": c.areaPayload("admin.dashboard.sidebar", layout.Areas["admin.dashboard.sidebar"]),
			"footer":  c.areaPayload("admin.dashboard.footer", layout.Areas["admin.dashboard.footer"]),
		},
	}
}

func (c *Controller) areaPayload(code string, widgets []WidgetInstance) map[string]any {
	return map[string]any{
		"code":    code,
		"widgets": c.widgetsPayload(widgets),
	}
}

func (c *Controller) widgetsPayload(instances []WidgetInstance) []map[string]any {
	if len(instances) == 0 {
		return nil
	}
	widgets := make([]map[string]any, 0, len(instances))
	for _, inst := range instances {
		var data any
		if inst.Metadata != nil {
			data = inst.Metadata["data"]
		}
		widgets = append(widgets, map[string]any{
			"id":        inst.ID,
			"template":  fmt.Sprintf("widgets/%s.html", inst.DefinitionID),
			"config":    inst.Configuration,
			"data":      data,
			"area_code": inst.AreaCode,
		})
	}
	return widgets
}

func (c *Controller) templatePath() string {
	return c.template
}

// RenderTemplate renders the dashboard HTML into the provided writer.
func (c *Controller) RenderTemplate(ctx context.Context, viewer ViewerContext, out io.Writer) error {
	if c.renderer == nil {
		return fmt.Errorf("dashboard: renderer not configured")
	}
	payload, err := c.payloadForViewer(ctx, viewer)
	if err != nil {
		return err
	}
	_, err = c.renderer.Render(c.templatePath(), payload, out)
	return err
}

func (c *Controller) payloadForViewer(ctx context.Context, viewer ViewerContext) (map[string]any, error) {
	layout, err := c.Render(ctx, viewer)
	if err != nil {
		return nil, err
	}
	return c.payloadFromLayout(layout), nil
}
