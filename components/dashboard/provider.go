package dashboard

import "context"

// Provider fetches data required to render a widget instance.
type Provider interface {
	Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error)
}

// ProviderFunc adapts a function to the Provider interface.
type ProviderFunc func(ctx context.Context, meta WidgetContext) (WidgetData, error)

// Fetch implements Provider.
func (fn ProviderFunc) Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error) {
	return fn(ctx, meta)
}

// WidgetContext contains the metadata needed by providers.
type WidgetContext struct {
	Instance WidgetInstance
	Viewer   ViewerContext
	Options  map[string]any
}

// WidgetData is an opaque payload passed to templates.
type WidgetData map[string]any
