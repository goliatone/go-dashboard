package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alecthomas/kong"
	"github.com/ettle/strcase"
	"gopkg.in/yaml.v3"

	"github.com/goliatone/go-dashboard/components/dashboard"
)

type cli struct {
	Scaffold scaffoldCmd `cmd:"" help:"Scaffold a widget definition, provider stub, and manifest entry."`
}

type scaffoldCmd struct {
	Code            string   `required:"" help:"Fully-qualified widget code (e.g. acme.widget.stats)."`
	Name            string   `required:"" help:"Display name for the widget."`
	Description     string   `required:"" help:"One-line description used in manifests."`
	Category        string   `default:"custom" help:"Widget category (analytics, stats, etc.)."`
	ManifestPath    string   `required:"" type:"path" help:"Path to the widget manifest YAML/JSON file to update."`
	SchemaPath      string   `type:"path" help:"Optional path to a JSON schema file for the widget configuration."`
	Tag             []string `help:"Optional tags to include in the manifest (use multiple --tag flags)."`
	Maintainer      []string `help:"Maintainers to record in the manifest."`
	Capabilities    []string `help:"Provider capability labels (html,json,sse,...)."`
	DocsURL         string   `help:"Link to provider documentation."`
	Channel         string   `help:"Distribution channel label (community, partner, internal)."`
	ProviderPackage string   `default:"github.com/goliatone/go-dashboard/components/dashboard" help:"Go package where the provider factory lives."`
	ProviderEntry   string   `help:"Factory identifier recorded in the manifest (defaults to New<Widget>Provider)."`
	ProviderOut     string   `help:"File path for the generated provider stub (defaults to components/dashboard/providers/<code>_provider.go)."`
	Overwrite       bool     `help:"Overwrite existing provider stub / manifest entry if present."`
	SkipProvider    bool     `name:"skip-provider" help:"Skip provider stub generation."`
}

func main() {
	ctx := kong.Parse(&cli{},
		kong.Description("Widget scaffolding utility for go-dashboard manifests."),
		kong.UsageOnError(),
	)
	err := ctx.Run(context.Background())
	ctx.FatalIfErrorf(err)
}

func (cmd *scaffoldCmd) Run(_ context.Context) error {
	if err := cmd.validate(); err != nil {
		return err
	}
	manifestPath, err := filepath.Abs(cmd.ManifestPath)
	if err != nil {
		return fmt.Errorf("widgetctl: resolve manifest path: %w", err)
	}
	doc, err := loadOrInitManifest(manifestPath)
	if err != nil {
		return err
	}
	if !cmd.Overwrite {
		for _, widget := range doc.Widgets {
			if widget.Definition.Code == cmd.Code {
				return fmt.Errorf("widgetctl: manifest already defines widget %s (use --overwrite to replace)", cmd.Code)
			}
		}
	}

	schema, err := cmd.loadSchema()
	if err != nil {
		return err
	}

	baseName := deriveBaseName(cmd.Code)
	providerType := baseName + "Provider"
	providerEntry := cmd.ProviderEntry
	if providerEntry == "" {
		providerEntry = fmt.Sprintf("%s.New%s", cmd.ProviderPackage, providerType)
	}

	entry := dashboard.ManifestWidget{
		Definition: dashboard.WidgetDefinition{
			Code:        cmd.Code,
			Name:        cmd.Name,
			Description: cmd.Description,
			Category:    cmd.Category,
			Schema:      schema,
		},
		Provider: dashboard.ManifestProvider{
			Name:         fmt.Sprintf("%s Provider", cmd.Name),
			Summary:      cmd.Description,
			Entry:        providerEntry,
			Package:      cmd.ProviderPackage,
			DocsURL:      cmd.DocsURL,
			Capabilities: cmd.Capabilities,
			Channel:      cmd.Channel,
		},
		Maintainers: cmd.Maintainer,
		Tags:        cmd.Tag,
	}

	if cmd.Overwrite {
		replaced := false
		for idx := range doc.Widgets {
			if doc.Widgets[idx].Definition.Code == cmd.Code {
				doc.Widgets[idx] = entry
				replaced = true
				break
			}
		}
		if !replaced {
			doc.Widgets = append(doc.Widgets, entry)
		}
	} else {
		doc.Widgets = append(doc.Widgets, entry)
	}

	sort.Slice(doc.Widgets, func(i, j int) bool {
		return doc.Widgets[i].Definition.Code < doc.Widgets[j].Definition.Code
	})

	if err := writeManifest(manifestPath, doc); err != nil {
		return err
	}

	if cmd.SkipProvider {
		fmt.Fprintf(os.Stdout, "✓ Added %s to %s (provider entry recorded as %s)\n", cmd.Code, manifestPath, providerEntry)
		return nil
	}

	providerPath := cmd.ProviderOut
	if providerPath == "" {
		providerPath = filepath.Join("components", "dashboard", "providers", fmt.Sprintf("%s_provider.go", sanitizeFileName(cmd.Code)))
	}
	if err := writeProviderStub(providerPath, providerType, cmd.Code, cmd.Overwrite); err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "✓ Added %s to %s and generated %s\n", cmd.Code, manifestPath, providerPath)
	return nil
}

func (cmd *scaffoldCmd) validate() error {
	if !strings.Contains(cmd.Code, ".") {
		return fmt.Errorf("widgetctl: widget code %s must contain at least one '.' segment", cmd.Code)
	}
	return nil
}

func (cmd *scaffoldCmd) loadSchema() (map[string]any, error) {
	if cmd.SchemaPath == "" {
		return map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		}, nil
	}
	data, err := os.ReadFile(cmd.SchemaPath)
	if err != nil {
		return nil, fmt.Errorf("widgetctl: read schema file: %w", err)
	}
	var schema map[string]any
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("widgetctl: parse schema JSON: %w", err)
	}
	return schema, nil
}

func loadOrInitManifest(path string) (*dashboard.WidgetManifestDocument, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			doc := &dashboard.WidgetManifestDocument{
				Version: dashboard.ManifestVersion,
				Widgets: []dashboard.ManifestWidget{},
				Source:  path,
			}
			return doc, nil
		}
		return nil, fmt.Errorf("widgetctl: stat manifest: %w", err)
	}
	doc, err := dashboard.ReadManifest(path)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func writeManifest(path string, doc *dashboard.WidgetManifestDocument) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("widgetctl: mkdir %s: %w", filepath.Dir(path), err)
	}
	tmpDoc := *doc
	tmpDoc.Source = ""

	file, err := os.Create(path) //nolint:gosec
	if err != nil {
		return fmt.Errorf("widgetctl: create manifest %s: %w", path, err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	defer encoder.Close()
	if err := encoder.Encode(tmpDoc); err != nil {
		return fmt.Errorf("widgetctl: write manifest: %w", err)
	}
	return nil
}

func writeProviderStub(path, providerType, code string, overwrite bool) error {
	if _, err := os.Stat(path); err == nil && !overwrite {
		return fmt.Errorf("widgetctl: provider stub %s already exists (use --overwrite or --provider-out)", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("widgetctl: mkdir provider dir: %w", err)
	}
	content := fmt.Sprintf(`package dashboard

import (
	"context"
)

// %s fetches data for %s widgets.
type %s struct{}

// New%s wires the provider into the dashboard registry.
func New%s() Provider {
	return &%s{}
}

// Fetch retrieves the widget payload. Replace with your implementation.
func (p *%s) Fetch(ctx context.Context, meta WidgetContext) (WidgetData, error) {
	_ = meta // TODO: use viewer context / configuration
	return WidgetData{
		"message": "replace with real data",
	}, nil
}
`, providerType, code, providerType, providerType, providerType, providerType, providerType)

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("widgetctl: write provider stub: %w", err)
	}
	return nil
}

func deriveBaseName(code string) string {
	parts := strings.Split(code, ".")
	slug := parts[len(parts)-1]
	slug = strings.TrimSpace(slug)
	if slug == "" {
		slug = code
	}
	return strcase.ToCamel(slug)
}

func sanitizeFileName(code string) string {
	replacer := strings.NewReplacer(".", "_", "-", "_", "/", "_", " ", "_")
	return strings.ToLower(replacer.Replace(code))
}
