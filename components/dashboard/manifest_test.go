package dashboard

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeManifest(t *testing.T) {
	const payload = `
version: 1
name: community-pack
widgets:
  - definition:
      code: community.widget.metrics
      name: Community Metrics
      description: Shows metrics pushed by the community pack.
      category: community
      schema:
        type: object
        properties:
          range:
            type: string
    provider:
      name: Community Provider
      summary: Calls the community metrics API.
      entry: github.com/example/community.Provider
      package: github.com/example/community
      docs_url: https://example.com/widgets/metrics
      capabilities: ["html","json"]
`
	doc, err := DecodeManifest(strings.NewReader(payload))
	require.NoError(t, err)
	require.Len(t, doc.Widgets, 1)

	widget := doc.Widgets[0]
	assert.Equal(t, "community.widget.metrics", widget.Definition.Code)
	assert.Equal(t, "Community Metrics", widget.Definition.Name)
	assert.Equal(t, "Community Provider", widget.Provider.Name)
	assert.Equal(t, "github.com/example/community.Provider", widget.Provider.Entry)
	assert.Equal(t, "community", widget.Definition.Category)
}

func TestRegistryLoadManifestDocument(t *testing.T) {
	doc := &WidgetManifestDocument{
		Version: manifestVersionV1,
		Widgets: []ManifestWidget{
			{
				Definition: WidgetDefinition{
					Code: "acme.widget.inventory",
					Name: "Inventory",
				},
				Provider: ManifestProvider{
					Name:    "Inventory Provider",
					Summary: "Fetches inventory counts",
					Entry:   "github.com/acme/widgets.NewInventoryProvider",
				},
			},
		},
	}
	reg := NewRegistry()

	err := reg.LoadManifestDocument(doc)
	require.NoError(t, err)

	def, ok := reg.Definition("acme.widget.inventory")
	require.True(t, ok)
	assert.Equal(t, "Inventory", def.Name)

	meta, ok := reg.ProviderMetadata("acme.widget.inventory")
	require.True(t, ok)
	assert.Equal(t, "Inventory Provider", meta.Name)
	assert.Equal(t, "github.com/acme/widgets.NewInventoryProvider", meta.Entry)
}

func TestManifestDuplicateCodes(t *testing.T) {
	const payload = `
widgets:
  - definition:
      code: dup.widget
      name: First
  - definition:
      code: dup.widget
      name: Second
`
	_, err := DecodeManifest(strings.NewReader(payload))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicates widget code")
}

func TestDocsManifestsAreValid(t *testing.T) {
	dir := filepath.Join("..", "..", "docs", "manifests")
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)

	codes := map[string]string{}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		doc, err := ReadManifest(path)
		require.NoErrorf(t, err, "manifest %s should parse", path)
		for _, widget := range doc.Widgets {
			if prev, exists := codes[widget.Definition.Code]; exists {
				t.Fatalf("widget code %s defined in both %s and %s", widget.Definition.Code, prev, path)
			}
			codes[widget.Definition.Code] = path
		}
	}
}
