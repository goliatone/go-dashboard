package dashboard

import (
	"context"
	"fmt"
	"io"
	"strings"
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
	areas    []AreaSlot
}

// ControllerOptions configures the HTTP controller.
type ControllerOptions struct {
	Service  LayoutResolver
	Renderer Renderer
	Template string
	Areas    []AreaSlot
}

// AreaSlot describes the mapping between a payload slot (main/sidebar/etc.)
// and the underlying widget area code stored in go-cms.
type AreaSlot struct {
	Slot string
	Code string
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
		areas:    normalizeAreaSlots(opts.Areas),
	}
}

// Render resolves the layout for a viewer and returns it to the caller.
func (c *Controller) Render(ctx context.Context, viewer ViewerContext) (Layout, error) {
	if c == nil || c.service == nil {
		return Layout{}, fmt.Errorf("dashboard: controller missing service")
	}
	return c.service.ConfigureLayout(ctx, viewer)
}

func (c *Controller) payloadFromLayout(layout Layout) map[string]any {
	areaMap := make(map[string]any, len(c.areas))
	ordered := make([]map[string]any, 0, len(c.areas))
	for _, section := range c.areas {
		payload := c.areaPayload(section.Code, layout.Areas[section.Code])
		payload["slot"] = section.Slot
		areaMap[section.Slot] = payload
		ordered = append(ordered, payload)
	}

	return map[string]any{
		"title":         "Dashboard",
		"description":   "Admin overview",
		"areas":         areaMap,
		"ordered_areas": ordered,
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
			"id":         inst.ID,
			"definition": inst.DefinitionID,
			"template":   templatePathFor(inst.DefinitionID),
			"config":     inst.Configuration,
			"data":       data,
			"area_code":  inst.AreaCode,
			"metadata":   inst.Metadata,
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
	payload := c.payloadFromLayout(layout)
	if viewer.Locale != "" {
		payload["locale"] = viewer.Locale
	} else {
		payload["locale"] = ""
	}
	return payload, nil
}

// LayoutPayload returns a JSON-ready payload with snake_case keys for the viewer.
func (c *Controller) LayoutPayload(ctx context.Context, viewer ViewerContext) (map[string]any, error) {
	return c.payloadForViewer(ctx, viewer)
}

func templatePathFor(definition string) string {
	if definition == "" {
		return "widgets/unknown.html"
	}
	parts := strings.Split(definition, ".")
	name := parts[len(parts)-1]
	return fmt.Sprintf("widgets/%s.html", name)
}

func normalizeAreaSlots(slots []AreaSlot) []AreaSlot {
	if len(slots) == 0 {
		return []AreaSlot{
			{Slot: "main", Code: "admin.dashboard.main"},
			{Slot: "sidebar", Code: "admin.dashboard.sidebar"},
			{Slot: "footer", Code: "admin.dashboard.footer"},
		}
	}
	result := make([]AreaSlot, 0, len(slots))
	seen := map[string]bool{}
	for _, slot := range slots {
		if slot.Slot == "" || slot.Code == "" {
			continue
		}
		if seen[slot.Slot] {
			continue
		}
		seen[slot.Slot] = true
		result = append(result, slot)
	}
	return result
}
