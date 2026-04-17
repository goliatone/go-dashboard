package dashboard

import (
	"context"
	"encoding/json"
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
	pageDecorator    PageDecorator
	payloadDecorator PayloadDecorator
}

// PageDecorator mutates the canonical typed page before transport-specific
// adapters run.
type PageDecorator func(ctx context.Context, viewer ViewerContext, page Page) (Page, error)

// PayloadDecorator mutates a controller payload after the canonical typed page
// is built and adapted. It is a temporary migration hook only.
type PayloadDecorator func(ctx context.Context, viewer ViewerContext, payload map[string]any) (map[string]any, error)

// ControllerOptions configures the HTTP controller.
type ControllerOptions struct {
	Service          LayoutResolver
	Renderer         Renderer
	Template         string
	Areas            []AreaSlot
	PageDecorator    PageDecorator
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
		pageDecorator:    opts.PageDecorator,
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

func (c *Controller) pageFromLayout(layout Layout, viewer ViewerContext) (Page, error) {
	page := Page{
		Title:       "Dashboard",
		Description: "Admin overview",
		Locale:      viewer.Locale,
		Theme:       layout.Theme,
		Areas:       make([]PageArea, 0, len(c.areas)),
	}
	assets := PageAssets{}
	for idx, section := range c.areas {
		widgets, widgetAssets, err := c.widgetFrames(section.Code, layout.Areas[section.Code])
		if err != nil {
			return Page{}, err
		}
		assets.AddJS(widgetAssets.JS...)
		assets.AddCSS(widgetAssets.CSS...)
		page.Areas = append(page.Areas, PageArea{
			Slot:    section.Slot,
			Code:    section.Code,
			Order:   idx + 1,
			Widgets: widgets,
		})
	}
	if !assets.Empty() {
		page.Assets = &assets
	}
	return page, nil
}

func (c *Controller) widgetFrames(code string, instances []WidgetInstance) ([]WidgetFrame, PageAssets, error) {
	if len(instances) == 0 {
		return nil, PageAssets{}, nil
	}
	widgets := make([]WidgetFrame, 0, len(instances))
	assets := PageAssets{}
	for idx, inst := range instances {
		data, dataPresent, err := resolveWidgetFrameData(inst.Metadata)
		if err != nil {
			return nil, PageAssets{}, err
		}
		data, widgetAssets, err := extractWidgetPageAssets(data)
		if err != nil {
			return nil, PageAssets{}, err
		}
		assets.AddJS(widgetAssets.JS...)
		assets.AddCSS(widgetAssets.CSS...)
		areaCode := inst.AreaCode
		if areaCode == "" {
			areaCode = code
		}
		widgets = append(widgets, WidgetFrame{
			ID:         inst.ID,
			Definition: inst.DefinitionID,
			Template:   templatePathFor(inst.DefinitionID),
			Config:     inst.Configuration,
			Data:       data,
			Area:       areaCode,
			Span:       widgetSpan(inst.Metadata),
			Hidden:     widgetHidden(inst.Metadata),
			Meta: WidgetMeta{
				Order:         idx + 1,
				Layout:        widgetLayout(inst.Metadata),
				Extensions:    widgetExtensions(inst.Metadata),
				dataPresent:   dataPresent,
				hiddenPresent: hasWidgetMetadataKey(inst.Metadata, "hidden"),
			},
		})
	}
	return widgets, assets, nil
}

func (c *Controller) templatePath() string {
	return c.template
}

// RenderTemplate renders the dashboard HTML into the provided writer through the
// canonical typed page renderer boundary.
func (c *Controller) RenderTemplate(ctx context.Context, viewer ViewerContext, out io.Writer) error {
	return c.RenderPage(ctx, viewer, out)
}

func (c *Controller) payloadForViewer(ctx context.Context, viewer ViewerContext) (map[string]any, error) {
	page, err := c.Page(ctx, viewer)
	if err != nil {
		return nil, err
	}
	payload := page.LegacyPayload()
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

// LayoutPayload returns the legacy JSON-ready payload adapter derived from the
// canonical typed page.
func (c *Controller) LayoutPayload(ctx context.Context, viewer ViewerContext) (map[string]any, error) {
	return c.payloadForViewer(ctx, viewer)
}

// Page resolves and decorates the canonical typed page for the current viewer.
func (c *Controller) Page(ctx context.Context, viewer ViewerContext) (Page, error) {
	layout, err := c.Render(ctx, viewer)
	if err != nil {
		return Page{}, err
	}
	page, err := c.pageFromLayout(layout, viewer)
	if err != nil {
		return Page{}, err
	}
	return c.decoratePage(ctx, viewer, page)
}

// RenderPage resolves and renders the typed dashboard page directly.
func (c *Controller) RenderPage(ctx context.Context, viewer ViewerContext, out io.Writer) error {
	if c.renderer == nil {
		return fmt.Errorf("dashboard: renderer not configured")
	}
	page, err := c.Page(ctx, viewer)
	if err != nil {
		return err
	}
	_, err = c.renderer.RenderPage(c.templatePath(), page, out)
	return err
}

func (c *Controller) decoratePage(ctx context.Context, viewer ViewerContext, page Page) (Page, error) {
	if c.pageDecorator == nil {
		return page, nil
	}
	decorated, err := c.pageDecorator(ctx, viewer, page)
	if err != nil {
		return Page{}, err
	}
	return decorated, nil
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

func widgetLayout(metadata map[string]any) *WidgetLayout {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata["layout"].(map[string]any)
	if !ok {
		return nil
	}
	layout := &WidgetLayout{
		Width:   widgetSpan(metadata),
		Columns: widgetSpan(metadata),
	}
	layout.Row = intValue(raw["row"])
	layout.Column = intValue(raw["column"])
	if width := intValue(raw["width"]); width > 0 {
		layout.Width = width
	}
	if columns := intValue(raw["columns"]); columns > 0 {
		layout.Columns = columns
	} else {
		layout.Columns = layout.Width
	}
	return layout
}

func widgetExtensions(metadata map[string]any) map[string]json.RawMessage {
	if len(metadata) == 0 {
		return nil
	}
	extensions := map[string]json.RawMessage{}
	for key, value := range metadata {
		switch key {
		case "data", "layout", "hidden":
			continue
		case widgetViewModelMetadataKey:
			continue
		}
		raw, err := json.Marshal(value)
		if err != nil {
			continue
		}
		extensions[key] = raw
	}
	if len(extensions) == 0 {
		return nil
	}
	return extensions
}

func intValue(value any) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int8:
		return int(typed)
	case int16:
		return int(typed)
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float32:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return 0
	}
}

func hasWidgetMetadataKey(metadata map[string]any, key string) bool {
	if len(metadata) == 0 {
		return false
	}
	_, ok := metadata[key]
	return ok
}

func (c *Controller) pageForViewer(ctx context.Context, viewer ViewerContext) (Page, error) {
	return c.Page(ctx, viewer)
}

func resolveWidgetFrameData(metadata map[string]any) (any, bool, error) {
	if len(metadata) == 0 {
		return nil, false, nil
	}
	if rawView, ok := metadata[widgetViewModelMetadataKey]; ok {
		if view, ok := rawView.(WidgetViewModel); ok {
			serialized, err := view.Serialize()
			if err != nil {
				return nil, true, err
			}
			return normalizeWidgetFrameData(serialized), true, nil
		}
		return normalizeWidgetFrameData(rawView), true, nil
	}
	if rawData, ok := metadata["data"]; ok {
		return normalizeWidgetFrameData(rawData), true, nil
	}
	return nil, false, nil
}

func extractWidgetPageAssets(data any) (any, PageAssets, error) {
	if data == nil {
		return nil, PageAssets{}, nil
	}
	normalized, err := normalizeStructuredValue(data)
	if err != nil {
		return data, PageAssets{}, err
	}
	payload, ok := normalized.(map[string]any)
	if !ok {
		return data, PageAssets{}, nil
	}
	assets := PageAssets{
		JS:  stringSliceFromValue(payload["js_assets"]),
		CSS: stringSliceFromValue(payload["css_assets"]),
	}
	if assets.Empty() {
		return data, PageAssets{}, nil
	}
	cleaned := cloneAnyMap(payload)
	delete(cleaned, "js_assets")
	delete(cleaned, "css_assets")
	return cleaned, assets, nil
}

func stringSliceFromValue(value any) []string {
	switch typed := value.(type) {
	case nil:
		return nil
	case []string:
		return append([]string{}, typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if s, ok := item.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
