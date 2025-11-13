package dashboard

func applyOrderOverride(widgets []WidgetInstance, order []string) []WidgetInstance {
	if len(order) == 0 {
		return widgets
	}
	index := make(map[string]WidgetInstance, len(widgets))
	for _, w := range widgets {
		index[w.ID] = w
	}
	result := make([]WidgetInstance, 0, len(widgets))
	seen := make(map[string]struct{}, len(order))
	for _, id := range order {
		if w, ok := index[id]; ok {
			result = append(result, w)
			seen[id] = struct{}{}
		}
	}
	for _, w := range widgets {
		if _, ok := seen[w.ID]; !ok {
			result = append(result, w)
		}
	}
	return result
}

func applyHiddenFilter(widgets []WidgetInstance, hidden map[string]bool) []WidgetInstance {
	if len(widgets) == 0 || len(hidden) == 0 {
		return widgets
	}
	filtered := widgets[:0]
	for _, w := range widgets {
		if hidden[w.ID] {
			continue
		}
		filtered = append(filtered, w)
	}
	return filtered
}

func applyRowMetadata(widgets []WidgetInstance, rows []LayoutRow) []WidgetInstance {
	if len(widgets) == 0 || len(rows) == 0 {
		return widgets
	}
	index := make(map[string]int, len(widgets))
	for i, w := range widgets {
		index[w.ID] = i
	}
	for rowIdx, row := range rows {
		colIdx := 0
		for _, slot := range row.Widgets {
			i, ok := index[slot.ID]
			if !ok {
				continue
			}
			if widgets[i].Metadata == nil {
				widgets[i].Metadata = map[string]any{}
			}
			width := slot.Width
			if width <= 0 {
				width = 12
			}
			if width > 12 {
				width = 12
			}
			widgets[i].Metadata["layout"] = map[string]any{
				"row":     rowIdx,
				"column":  colIdx,
				"width":   width,
				"columns": width,
			}
			colIdx++
		}
	}
	return widgets
}
