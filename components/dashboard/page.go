package dashboard

import "encoding/json"

// Page is the canonical typed dashboard presentation model used for rendering
// and JSON transport. Area ordering is preserved directly by the Areas slice.
type Page struct {
	Title       string          `json:"title,omitempty"`
	Description string          `json:"description,omitempty"`
	Locale      string          `json:"locale,omitempty"`
	Areas       []PageArea      `json:"areas,omitempty"`
	Theme       *ThemeSelection `json:"theme,omitempty"`
	State       *PageState      `json:"state,omitempty"`
	Meta        *PageMeta       `json:"meta,omitempty"`
}

// Area returns the first area registered for the provided slot.
func (page Page) Area(slot string) (PageArea, bool) {
	for _, area := range page.Areas {
		if area.Slot == slot {
			return area, true
		}
	}
	return PageArea{}, false
}

// LegacyPayload adapts the typed page contract to the historical payload-map
// shape used by existing transports and renderers during the migration.
func (page Page) LegacyPayload() map[string]any {
	theme := themePayload(page.Theme)
	areas := make(map[string]any, len(page.Areas))
	ordered := make([]map[string]any, 0, len(page.Areas))
	for _, area := range page.Areas {
		payload := area.legacyPayload(theme)
		areas[area.Slot] = payload
		ordered = append(ordered, payload)
	}

	response := map[string]any{
		"title":         page.Title,
		"description":   page.Description,
		"locale":        page.Locale,
		"areas":         areas,
		"ordered_areas": ordered,
	}
	if theme != nil {
		response["theme"] = theme
	}
	return response
}

// PageArea models a typed dashboard area/slot on the page.
type PageArea struct {
	Slot    string        `json:"slot,omitempty"`
	Code    string        `json:"code,omitempty"`
	Title   string        `json:"title,omitempty"`
	Order   int           `json:"order"`
	Widgets []WidgetFrame `json:"widgets,omitempty"`
}

// Widget returns the first widget in the area with the provided id.
func (area PageArea) Widget(id string) (WidgetFrame, bool) {
	for _, widget := range area.Widgets {
		if widget.ID == id {
			return widget, true
		}
	}
	return WidgetFrame{}, false
}

func (area PageArea) legacyPayload(theme map[string]any) map[string]any {
	widgets := make([]map[string]any, 0, len(area.Widgets))
	for _, widget := range area.Widgets {
		widgets = append(widgets, widget.legacyPayload(theme))
	}

	payload := map[string]any{
		"slot": area.Slot,
		"code": area.Code,
	}
	if area.Title != "" {
		payload["title"] = area.Title
	}
	if len(widgets) > 0 {
		payload["widgets"] = widgets
	} else {
		payload["widgets"] = nil
	}
	return payload
}

// WidgetFrame models the framework-owned widget fields exposed on the typed page.
// App-specific data remains in Data/Meta extensions and will be tightened in the
// typed widget authoring work that follows this baseline phase.
type WidgetFrame struct {
	ID         string         `json:"id,omitempty"`
	Definition string         `json:"definition,omitempty"`
	Name       string         `json:"name,omitempty"`
	Template   string         `json:"template,omitempty"`
	Area       string         `json:"area,omitempty"`
	Span       int            `json:"span,omitempty"`
	Hidden     bool           `json:"hidden,omitempty"`
	Config     map[string]any `json:"config,omitempty"`
	Data       any            `json:"data,omitempty"`
	Meta       WidgetMeta     `json:"meta"`
}

func (widget WidgetFrame) legacyPayload(theme map[string]any) map[string]any {
	payload := map[string]any{
		"id":         widget.ID,
		"definition": widget.Definition,
		"name":       widget.Name,
		"template":   widget.Template,
		"config":     widget.Config,
		"data":       normalizeWidgetFrameData(widget.Data),
		"area":       widget.Area,
		"area_code":  widget.Area,
		"span":       widget.Span,
		"hidden":     widget.Hidden,
		"metadata":   widget.legacyMetadata(),
		"theme":      theme,
	}
	if widget.Name == "" {
		delete(payload, "name")
	}
	return payload
}

func normalizeWidgetFrameData(data any) any {
	switch typed := data.(type) {
	case WidgetData:
		return map[string]any(typed)
	default:
		return data
	}
}

func (widget WidgetFrame) legacyMetadata() map[string]any {
	metadata := decodeWidgetExtensions(widget.Meta.Extensions)
	if widget.Meta.dataPresent {
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["data"] = normalizeWidgetFrameData(widget.Data)
	}
	if widget.Meta.Layout != nil {
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["layout"] = map[string]any{
			"row":     widget.Meta.Layout.Row,
			"column":  widget.Meta.Layout.Column,
			"width":   widget.Meta.Layout.Width,
			"columns": widget.Meta.Layout.Columns,
		}
	}
	if widget.Meta.hiddenPresent {
		if metadata == nil {
			metadata = map[string]any{}
		}
		metadata["hidden"] = widget.Hidden
	}
	return metadata
}

func decodeWidgetExtensions(extensions map[string]json.RawMessage) map[string]any {
	if len(extensions) == 0 {
		return nil
	}
	decoded := make(map[string]any, len(extensions))
	for key, raw := range extensions {
		if len(raw) == 0 {
			continue
		}
		var value any
		if err := json.Unmarshal(raw, &value); err != nil {
			continue
		}
		decoded[key] = value
	}
	if len(decoded) == 0 {
		return nil
	}
	return decoded
}

// WidgetMeta carries framework-managed widget metadata and a constrained
// extension bag for additional transport fields.
type WidgetMeta struct {
	Order         int                        `json:"order"`
	Layout        *WidgetLayout              `json:"layout,omitempty"`
	Extensions    map[string]json.RawMessage `json:"extensions,omitempty"`
	dataPresent   bool                       `json:"-"`
	hiddenPresent bool                       `json:"-"`
}

// WidgetLayout captures typed placement metadata guaranteed by the framework.
type WidgetLayout struct {
	Row     int `json:"row"`
	Column  int `json:"column"`
	Width   int `json:"width"`
	Columns int `json:"columns"`
}

// PageState captures viewer-scoped runtime state that may influence rendering
// and transport behavior.
type PageState struct {
	Viewer      ViewerContext   `json:"viewer"`
	Preferences LayoutOverrides `json:"preferences"`
}

// PageMeta carries framework-level extensions that apply to the whole page.
type PageMeta struct {
	Extensions map[string]json.RawMessage `json:"extensions,omitempty"`
}
