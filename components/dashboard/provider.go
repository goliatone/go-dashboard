package dashboard

import "context"

// Provider fetches the data required to render a widget instance. Providers can
// call any downstream dependency (databases, APIs, services) as long as they
// respect the viewer context that is supplied in the WidgetContext payload.
type Provider interface {
	Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error)
}

// ProviderFunc adapts a function to the Provider interface.
type ProviderFunc func(ctx context.Context, meta WidgetContext) (WidgetData, error)

// Fetch implements Provider.
func (fn ProviderFunc) Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error) {
	return fn(ctx, meta)
}

// WidgetContext contains the metadata needed by providers to calculate widget data.
type WidgetContext struct {
	Instance   WidgetInstance
	Viewer     ViewerContext
	Options    map[string]any
	Translator TranslationService
}

// WidgetData is an opaque payload passed to templates.
type WidgetData map[string]any
