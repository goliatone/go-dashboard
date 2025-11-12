package dashboard

import "context"

// Provider fetches data required to render a widget instance.
type Provider interface {
	Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error)
}

// WidgetContext contains the metadata needed by providers.
type WidgetContext struct {
	Instance WidgetInstance
	Viewer   ViewerContext
}

// WidgetData is an opaque payload passed to templates.
type WidgetData map[string]any
