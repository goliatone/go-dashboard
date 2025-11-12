package dashboard

import (
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	manifestVersionV1 = "1"
	// ManifestVersion exposes the current manifest format version for tooling.
	ManifestVersion = manifestVersionV1
)

// WidgetManifestDocument models a YAML/JSON manifest describing widgets/providers.
type WidgetManifestDocument struct {
	Version  string           `json:"version" yaml:"version"`
	Name     string           `json:"name,omitempty" yaml:"name,omitempty"`
	Package  string           `json:"package,omitempty" yaml:"package,omitempty"`
	Homepage string           `json:"homepage,omitempty" yaml:"homepage,omitempty"`
	Widgets  []ManifestWidget `json:"widgets" yaml:"widgets"`
	Source   string           `json:"-" yaml:"-"`
}

// ManifestWidget describes a single widget entry within a manifest.
type ManifestWidget struct {
	Definition  WidgetDefinition `json:"definition" yaml:"definition"`
	Provider    ManifestProvider `json:"provider,omitempty" yaml:"provider,omitempty"`
	Maintainers []string         `json:"maintainers,omitempty" yaml:"maintainers,omitempty"`
	Tags        []string         `json:"tags,omitempty" yaml:"tags,omitempty"`
}

// ManifestProvider captures discovery metadata about a provider implementation.
type ManifestProvider struct {
	Name         string   `json:"name,omitempty" yaml:"name,omitempty"`
	Summary      string   `json:"summary,omitempty" yaml:"summary,omitempty"`
	Entry        string   `json:"entry,omitempty" yaml:"entry,omitempty"`
	Package      string   `json:"package,omitempty" yaml:"package,omitempty"`
	DocsURL      string   `json:"docs_url,omitempty" yaml:"docs_url,omitempty"`
	Capabilities []string `json:"capabilities,omitempty" yaml:"capabilities,omitempty"`
	Channel      string   `json:"channel,omitempty" yaml:"channel,omitempty"`
}

// LoadManifestFile reads a manifest from disk, registers it against the registry, and returns the document.
func (r *Registry) LoadManifestFile(path string) (*WidgetManifestDocument, error) {
	doc, err := ReadManifest(path)
	if err != nil {
		return nil, err
	}
	if err := r.LoadManifestDocument(doc); err != nil {
		return nil, err
	}
	return doc, nil
}

// LoadManifestDocument registers definitions and provider metadata from a decoded manifest.
func (r *Registry) LoadManifestDocument(doc *WidgetManifestDocument) error {
	if doc == nil {
		return fmt.Errorf("dashboard: manifest document is nil")
	}
	for _, widget := range doc.Widgets {
		if err := r.RegisterDefinition(widget.Definition); err != nil {
			return fmt.Errorf("dashboard: register widget %s from %s: %w", widget.Definition.Code, doc.Source, err)
		}
		r.recordProviderMetadata(widget.Definition.Code, widget.Provider)
	}
	return nil
}

// ReadManifest loads a manifest file from disk without registering it.
func ReadManifest(path string) (*WidgetManifestDocument, error) {
	f, err := os.Open(path) //nolint:gosec
	if err != nil {
		return nil, fmt.Errorf("dashboard: open manifest %s: %w", path, err)
	}
	defer f.Close()
	doc, err := DecodeManifest(f)
	if err != nil {
		return nil, fmt.Errorf("dashboard: decode manifest %s: %w", path, err)
	}
	doc.Source = path
	return doc, nil
}

// DecodeManifest reads a manifest from any reader.
func DecodeManifest(r io.Reader) (*WidgetManifestDocument, error) {
	decoder := yaml.NewDecoder(r)
	decoder.KnownFields(true)
	var doc WidgetManifestDocument
	if err := decoder.Decode(&doc); err != nil {
		if err == io.EOF {
			return nil, fmt.Errorf("dashboard: manifest is empty")
		}
		return nil, fmt.Errorf("dashboard: parse manifest: %w", err)
	}
	doc.applyDefaults()
	if err := doc.Validate(); err != nil {
		return nil, err
	}
	return &doc, nil
}

// Validate ensures the manifest satisfies required fields.
func (doc *WidgetManifestDocument) Validate() error {
	if doc.Version != manifestVersionV1 {
		return fmt.Errorf("dashboard: unsupported manifest version %q", doc.Version)
	}
	seen := make(map[string]struct{}, len(doc.Widgets))
	for idx, widget := range doc.Widgets {
		if widget.Definition.Code == "" {
			return fmt.Errorf("dashboard: manifest widget at index %d is missing definition.code", idx)
		}
		if widget.Definition.Name == "" {
			return fmt.Errorf("dashboard: manifest widget %s missing definition.name", widget.Definition.Code)
		}
		if _, exists := seen[widget.Definition.Code]; exists {
			return fmt.Errorf("dashboard: manifest duplicates widget code %s", widget.Definition.Code)
		}
		seen[widget.Definition.Code] = struct{}{}
	}
	return nil
}

func (doc *WidgetManifestDocument) applyDefaults() {
	if doc.Version == "" {
		doc.Version = manifestVersionV1
	}
}

func (p ManifestProvider) isZero() bool {
	return p.Name == "" &&
		p.Summary == "" &&
		p.Entry == "" &&
		p.Package == "" &&
		p.DocsURL == "" &&
		len(p.Capabilities) == 0 &&
		p.Channel == ""
}
