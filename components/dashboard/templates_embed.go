package dashboard

import (
	"context"
	"embed"

	template "github.com/goliatone/go-template"
)

//go:embed templates/*.html templates/**/*.html
var embeddedTemplates embed.FS

// TemplateRendererOption customizes the embedded renderer (function map, translation helpers, etc.).
type TemplateRendererOption func(*templateRendererConfig)

type templateRendererConfig struct {
	funcs      map[string]any
	translator TranslationService
}

// WithTemplateFuncMap merges custom template helper functions into the renderer.
func WithTemplateFuncMap(funcs map[string]any) TemplateRendererOption {
	return func(cfg *templateRendererConfig) {
		if len(funcs) == 0 {
			return
		}
		if cfg.funcs == nil {
			cfg.funcs = map[string]any{}
		}
		for key, fn := range funcs {
			if key == "" || fn == nil {
				continue
			}
			cfg.funcs[key] = fn
		}
	}
}

// WithTranslationHelpers wires a TranslationService so templates can call {{ T "key" locale }}.
func WithTranslationHelpers(svc TranslationService) TemplateRendererOption {
	return func(cfg *templateRendererConfig) {
		cfg.translator = svc
	}
}

// NewTemplateRenderer creates a go-template renderer backed by the embedded templates.
func NewTemplateRenderer(options ...TemplateRendererOption) (Renderer, error) {
	cfg := templateRendererConfig{}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(&cfg)
	}
	funcMap := map[string]any{
		"T": makeTemplateTranslationFunc(cfg.translator),
	}
	for key, fn := range cfg.funcs {
		funcMap[key] = fn
	}
	opts := []template.Option{
		template.WithFS(embeddedTemplates),
		template.WithBaseDir("templates"),
		template.WithExtension(".html"),
		template.WithTemplateFunc(funcMap),
	}
	return template.NewRenderer(opts...)
}

func makeTemplateTranslationFunc(svc TranslationService) func(string, string, ...any) string {
	return func(key, locale string, extras ...any) string {
		fallback, params := translationArgsFromExtras(extras...)
		return translateOrFallback(context.Background(), svc, key, locale, fallback, params)
	}
}

func translationArgsFromExtras(extras ...any) (string, map[string]any) {
	var fallback string
	var params map[string]any
	for _, arg := range extras {
		switch value := arg.(type) {
		case string:
			if fallback == "" && value != "" {
				fallback = value
			}
		case map[string]any:
			params = value
		}
	}
	return fallback, params
}
