package dashboard

import "encoding/json"

func cloneAnyValue(value any) any {
	switch typed := value.(type) {
	case nil:
		return nil
	case WidgetData:
		return map[string]any(cloneAnyMap(map[string]any(typed)))
	case map[string]any:
		return cloneAnyMap(typed)
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cloneAnyValue(item)
		}
		return out
	case []map[string]any:
		out := make([]map[string]any, len(typed))
		for i, item := range typed {
			out[i] = cloneAnyMap(item)
		}
		return out
	case []string:
		return append([]string{}, typed...)
	case map[string]string:
		out := make(map[string]string, len(typed))
		for key, item := range typed {
			out[key] = item
		}
		return out
	case json.RawMessage:
		return append(json.RawMessage(nil), typed...)
	default:
		return value
	}
}

func cloneWidgetAreaDefinition(area WidgetAreaDefinition) WidgetAreaDefinition {
	return area
}

func cloneWidgetAreaDefinitions(areas []WidgetAreaDefinition) []WidgetAreaDefinition {
	if len(areas) == 0 {
		return nil
	}
	out := make([]WidgetAreaDefinition, len(areas))
	for i, area := range areas {
		out[i] = cloneWidgetAreaDefinition(area)
	}
	return out
}

func cloneWidgetInstances(instances []WidgetInstance) []WidgetInstance {
	if len(instances) == 0 {
		return nil
	}
	out := make([]WidgetInstance, len(instances))
	for i, inst := range instances {
		out[i] = cloneWidgetInstance(inst)
	}
	return out
}

func cloneWidgetInstance(inst WidgetInstance) WidgetInstance {
	inst.Configuration = cloneAnyMap(inst.Configuration)
	inst.Metadata = cloneAnyMap(inst.Metadata)
	return inst
}

func cloneLayout(layout Layout) Layout {
	out := Layout{
		Areas: make(map[string][]WidgetInstance, len(layout.Areas)),
		Theme: cloneThemeSelection(layout.Theme),
	}
	for code, widgets := range layout.Areas {
		out.Areas[code] = cloneWidgetInstances(widgets)
	}
	return out
}

func cloneLayoutOverrides(overrides LayoutOverrides) LayoutOverrides {
	out := LayoutOverrides{
		Locale:        overrides.Locale,
		AreaOrder:     map[string][]string{},
		AreaRows:      map[string][]LayoutRow{},
		HiddenWidgets: map[string]bool{},
	}
	for area, ids := range overrides.AreaOrder {
		out.AreaOrder[area] = append([]string{}, ids...)
	}
	for area, rows := range overrides.AreaRows {
		out.AreaRows[area] = cloneLayoutRows(rows)
	}
	for id, hidden := range overrides.HiddenWidgets {
		out.HiddenWidgets[id] = hidden
	}
	return out
}

func cloneLayoutRows(rows []LayoutRow) []LayoutRow {
	if len(rows) == 0 {
		return nil
	}
	out := make([]LayoutRow, len(rows))
	for i, row := range rows {
		out[i] = LayoutRow{Widgets: cloneWidgetSlots(row.Widgets)}
	}
	return out
}

func cloneWidgetSlots(slots []WidgetSlot) []WidgetSlot {
	if len(slots) == 0 {
		return nil
	}
	out := make([]WidgetSlot, len(slots))
	copy(out, slots)
	return out
}

func clonePage(page Page) Page {
	out := Page{
		Title:       page.Title,
		Description: page.Description,
		Locale:      page.Locale,
		Areas:       clonePageAreas(page.Areas),
		Assets:      clonePageAssets(page.Assets),
		Theme:       cloneThemeSelection(page.Theme),
		State:       clonePageState(page.State),
		Meta:        clonePageMeta(page.Meta),
	}
	return out
}

func clonePageAreas(areas []PageArea) []PageArea {
	if len(areas) == 0 {
		return nil
	}
	out := make([]PageArea, len(areas))
	for i, area := range areas {
		out[i] = PageArea{
			Slot:    area.Slot,
			Code:    area.Code,
			Title:   area.Title,
			Order:   area.Order,
			Widgets: cloneWidgetFrames(area.Widgets),
		}
	}
	return out
}

func cloneWidgetFrames(widgets []WidgetFrame) []WidgetFrame {
	if len(widgets) == 0 {
		return nil
	}
	out := make([]WidgetFrame, len(widgets))
	for i, widget := range widgets {
		out[i] = WidgetFrame{
			ID:         widget.ID,
			Definition: widget.Definition,
			Name:       widget.Name,
			Template:   widget.Template,
			Area:       widget.Area,
			Span:       widget.Span,
			Hidden:     widget.Hidden,
			Config:     cloneAnyMap(widget.Config),
			Data:       cloneAnyValue(widget.Data),
			Meta:       cloneWidgetMeta(widget.Meta),
		}
	}
	return out
}

func cloneWidgetMeta(meta WidgetMeta) WidgetMeta {
	out := WidgetMeta{
		Order:         meta.Order,
		Layout:        cloneWidgetLayout(meta.Layout),
		Extensions:    cloneRawMessages(meta.Extensions),
		dataPresent:   meta.dataPresent,
		hiddenPresent: meta.hiddenPresent,
	}
	return out
}

func cloneWidgetLayout(layout *WidgetLayout) *WidgetLayout {
	if layout == nil {
		return nil
	}
	copy := *layout
	return &copy
}

func clonePageState(state *PageState) *PageState {
	if state == nil {
		return nil
	}
	return &PageState{
		Viewer:      state.Viewer,
		Preferences: cloneLayoutOverrides(state.Preferences),
	}
}

func clonePageMeta(meta *PageMeta) *PageMeta {
	if meta == nil {
		return nil
	}
	return &PageMeta{Extensions: cloneRawMessages(meta.Extensions)}
}

func clonePageAssets(assets *PageAssets) *PageAssets {
	if assets == nil || assets.Empty() {
		return nil
	}
	return &PageAssets{
		JS:  append([]string{}, assets.JS...),
		CSS: append([]string{}, assets.CSS...),
	}
}

func cloneRawMessages(in map[string]json.RawMessage) map[string]json.RawMessage {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]json.RawMessage, len(in))
	for key, value := range in {
		out[key] = append(json.RawMessage(nil), value...)
	}
	return out
}
