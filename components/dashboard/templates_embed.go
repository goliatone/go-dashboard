package dashboard

import (
	"context"
	"embed"
	"io"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"sync"

	template "github.com/goliatone/go-template"
)

//go:embed templates/*.html templates/**/*.html templates/**/**/*.html
var embeddedTemplates embed.FS

var (
	embeddedTemplateRootOnce sync.Once
	embeddedTemplateRootDir  string
	embeddedTemplateRootErr  error
)

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

// NewTemplateRenderer creates a typed dashboard page renderer backed by the
// embedded templates. The underlying template engine still consumes template
// data maps, but that adaptation is contained within the renderer boundary.
func NewTemplateRenderer(options ...TemplateRendererOption) (Renderer, error) {
	cfg := templateRendererConfig{}
	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(&cfg)
	}
	funcMap := map[string]any{
		"T":        makeTemplateTranslationFunc(cfg.translator),
		"coalesce": templateCoalesce,
	}
	maps.Copy(funcMap, cfg.funcs)
	baseDir, err := embeddedTemplateBaseDir()
	if err != nil {
		return nil, err
	}
	opts := []template.Option{
		template.WithBaseDir(baseDir),
		template.WithExtension(".html"),
		template.WithTemplateFunc(funcMap),
	}
	engine, err := template.NewRenderer(opts...)
	if err != nil {
		return nil, err
	}
	return templatePageRenderer{renderer: engine, rootDir: baseDir}, nil
}

func embeddedTemplateBaseDir() (string, error) {
	embeddedTemplateRootOnce.Do(func() {
		dir, err := os.MkdirTemp("", "go-dashboard-templates-")
		if err != nil {
			embeddedTemplateRootErr = err
			return
		}
		if err := copyEmbeddedTemplates(dir); err != nil {
			_ = os.RemoveAll(dir)
			embeddedTemplateRootErr = err
			return
		}
		embeddedTemplateRootDir = dir
	})
	return embeddedTemplateRootDir, embeddedTemplateRootErr
}

func copyEmbeddedTemplates(root string) error {
	return fs.WalkDir(embeddedTemplates, "templates", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel("templates", path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(root, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(embeddedTemplates, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
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

func templateCoalesce(values ...any) any {
	for _, value := range values {
		if !templateIsEmpty(value) {
			return value
		}
	}
	return nil
}

func templateIsEmpty(value any) bool {
	if value == nil {
		return true
	}
	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Bool:
		return !rv.Bool()
	case reflect.String:
		return rv.Len() == 0
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return rv.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return rv.IsNil()
	case reflect.Slice, reflect.Array, reflect.Map:
		return rv.Len() == 0
	default:
		return false
	}
}

type templatePageRenderer struct {
	renderer LegacyRenderer
	rootDir  string
}

func (renderer templatePageRenderer) RenderPage(name string, page Page, out ...io.Writer) (string, error) {
	payload := page.LegacyPayload()
	renderer.normalizeWidgetTemplates(payload)
	return renderer.renderer.Render(name, payload, out...)
}

func (renderer templatePageRenderer) normalizeWidgetTemplates(payload map[string]any) {
	if renderer.rootDir == "" || len(payload) == 0 {
		return
	}
	areas, _ := payload["areas"].(map[string]any)
	for _, rawArea := range areas {
		area, ok := rawArea.(map[string]any)
		if !ok {
			continue
		}
		widgets, ok := area["widgets"].([]map[string]any)
		if !ok {
			continue
		}
		for _, widget := range widgets {
			templateName, _ := widget["template"].(string)
			if templateName == "" || filepath.IsAbs(templateName) {
				continue
			}
			widget["template"] = filepath.Join(renderer.rootDir, path.Clean(templateName))
		}
	}
}
