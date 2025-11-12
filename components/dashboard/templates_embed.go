package dashboard

import (
	"embed"

	template "github.com/goliatone/go-template"
)

//go:embed templates/*.html templates/**/*.html
var embeddedTemplates embed.FS

// NewTemplateRenderer creates a go-template renderer backed by the embedded templates.
func NewTemplateRenderer() (Renderer, error) {
	return template.NewRenderer(
		template.WithFS(embeddedTemplates),
		template.WithBaseDir("templates"),
		template.WithExtension(".html"),
	)
}
