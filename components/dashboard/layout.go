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
