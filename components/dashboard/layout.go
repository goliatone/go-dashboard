package dashboard

import "context"

// ViewerContext captures the active user/locale information needed to render dashboards.
type ViewerContext struct {
	UserID string
	Roles  []string
	Locale string
}

// Layout describes the resolved widget instances per dashboard area.
type Layout struct {
	Areas map[string][]WidgetInstance
}

// WidgetInstance is a lightweight placeholder struct until go-cms integration lands.
type WidgetInstance struct {
	ID           string
	DefinitionID string
	Config       map[string]any
}

// ConfigureLayout resolves the final widget arrangement for a user.
func (s *Service) ConfigureLayout(ctx context.Context, viewer ViewerContext) (Layout, error) {
	return Layout{Areas: map[string][]WidgetInstance{}}, nil
}
