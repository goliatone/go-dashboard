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
	service          LayoutResolver
	renderer         Renderer
	template         string
	areas            []AreaSlot
	payloadDecorator PayloadDecorator
}

// PayloadDecorator mutates a controller payload after the canonical payload is built.
type PayloadDecorator func(ctx context.Context, viewer ViewerContext, payload map[string]any) (map[string]any, error)

// ControllerOptions configures the HTTP controller.
type ControllerOptions struct {
	Service          LayoutResolver
	Renderer         Renderer
	Template         string
	Areas            []AreaSlot
	PayloadDecorator PayloadDecorator
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
		service:          opts.Service,
		renderer:         opts.Renderer,
		template:         templateName,
		areas:            normalizeAreaSlots(opts.Areas),
		payloadDecorator: opts.PayloadDecorator,
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
	theme := themePayload(layout.Theme)
	areaMap := make(map[string]any, len(c.areas))
	ordered := make([]map[string]any, 0, len(c.areas))
	for _, section := range c.areas {
		payload := c.areaPayload(section.Code, layout.Areas[section.Code], theme)
		payload["slot"] = section.Slot
		areaMap[section.Slot] = payload
		ordered = append(ordered, payload)
	}

	response := map[string]any{
		"title":         "Dashboard",
		"description":   "Admin overview",
		"areas":         areaMap,
		"ordered_areas": ordered,
	}
	if theme != nil {
		response["theme"] = theme
	}
	return response
}

func (c *Controller) areaPayload(code string, widgets []WidgetInstance, theme map[string]any) map[string]any {
	return map[string]any{
		"code":    code,
		"widgets": c.widgetsPayload(widgets, theme),
	}
}

func (c *Controller) widgetsPayload(instances []WidgetInstance, theme map[string]any) []map[string]any {
	if len(instances) == 0 {
		return nil
	}
	widgets := make([]map[string]any, 0, len(instances))
	for _, inst := range instances {
		var data any
		if inst.Metadata != nil {
			data = normalizeWidgetPayloadData(inst.Metadata["data"])
		}
		widgets = append(widgets, map[string]any{
			"id":         inst.ID,
			"definition": inst.DefinitionID,
			"template":   templatePathFor(inst.DefinitionID),
			"config":     inst.Configuration,
			"data":       data,
			"area":       inst.AreaCode,
			"area_code":  inst.AreaCode,
			"span":       widgetSpan(inst.Metadata),
			"hidden":     widgetHidden(inst.Metadata),
			"metadata":   inst.Metadata,
			"theme":      theme,
		})
	}
	return widgets
}

func normalizeWidgetPayloadData(data any) any {
	switch typed := data.(type) {
	case WidgetData:
		return map[string]any(typed)
	default:
		return data
	}
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
	if c.payloadDecorator != nil {
		decorated, err := c.payloadDecorator(ctx, viewer, payload)
		if err != nil {
			return nil, err
		}
		if decorated != nil {
			payload = decorated
		}
	}
	return payload, nil
}

// LayoutPayload returns a JSON-ready payload with snake_case keys for the viewer.
func (c *Controller) LayoutPayload(ctx context.Context, viewer ViewerContext) (map[string]any, error) {
	return c.payloadForViewer(ctx, viewer)
}

func themePayload(selection *ThemeSelection) map[string]any {
	if selection == nil {
		return nil
	}
	payload := map[string]any{}
	if selection.Name != "" {
		payload["name"] = selection.Name
	}
	if selection.Variant != "" {
		payload["variant"] = selection.Variant
	}
	if len(selection.Tokens) > 0 {
		payload["tokens"] = selection.Tokens
	}
	if cssVars := selection.CSSVariables(); len(cssVars) > 0 {
		payload["css_vars"] = cssVars
	}
	if inline := selection.CSSVariablesInline(); inline != "" {
		payload["css_vars_inline"] = inline
	}
	if selection.Assets.Prefix != "" {
		payload["asset_prefix"] = selection.Assets.Prefix
	}
	if assets := selection.Assets.Resolved(); len(assets) > 0 {
		payload["assets"] = assets
	}
	if len(selection.Templates) > 0 {
		payload["templates"] = selection.Templates
	}
	if selection.ChartTheme != "" {
		payload["chart_theme"] = selection.ChartTheme
	}
	return payload
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

func widgetSpan(metadata map[string]any) int {
	if metadata == nil {
		return 12
	}
	if layout, ok := metadata["layout"].(map[string]any); ok {
		switch width := layout["width"].(type) {
		case int:
			if width > 0 {
				return width
			}
		case int32:
			if width > 0 {
				return int(width)
			}
		case int64:
			if width > 0 {
				return int(width)
			}
		case float64:
			if width > 0 {
				return int(width)
			}
		}
	}
	return 12
}

func widgetHidden(metadata map[string]any) bool {
	if metadata == nil {
		return false
	}
	hidden, _ := metadata["hidden"].(bool)
	return hidden
}
