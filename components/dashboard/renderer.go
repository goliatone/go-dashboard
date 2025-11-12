package dashboard

import "io"

// Renderer describes the template renderer contract needed by the controller.
type Renderer interface {
	Render(name string, data any, out ...io.Writer) (string, error)
}
