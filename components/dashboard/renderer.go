package dashboard

import "io"

// Renderer describes the canonical typed page renderer contract needed by the
// controller.
type Renderer interface {
	RenderPage(name string, page Page, out ...io.Writer) (string, error)
}

// LegacyRenderer describes the historical renderer contract that accepted
// arbitrary payloads. It remains available only for migration adapters.
type LegacyRenderer interface {
	Render(name string, data any, out ...io.Writer) (string, error)
}

// RendererFunc adapts a function to the typed page renderer interface.
type RendererFunc func(name string, page Page, out ...io.Writer) (string, error)

// RenderPage implements Renderer.
func (fn RendererFunc) RenderPage(name string, page Page, out ...io.Writer) (string, error) {
	return fn(name, page, out...)
}

type legacyRendererAdapter struct {
	renderer LegacyRenderer
}

func (adapter legacyRendererAdapter) RenderPage(name string, page Page, out ...io.Writer) (string, error) {
	return adapter.renderer.Render(name, page.LegacyPayload(), out...)
}

// AdaptLegacyRenderer wraps a legacy renderer so the controller can keep using
// the typed page contract while older template implementations migrate.
func AdaptLegacyRenderer(renderer LegacyRenderer) Renderer {
	if renderer == nil {
		return nil
	}
	return legacyRendererAdapter{renderer: renderer}
}
