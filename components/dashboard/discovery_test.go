package dashboard

import (
	"context"
	"testing"
)

func TestRegistryCatalogSnapshotIsStableAndOrdered(t *testing.T) {
	reg := &Registry{
		areas:           map[string]WidgetAreaDefinition{},
		definitions:     map[string]WidgetDefinition{},
		areaOrder:       []string{},
		definitionOrder: []string{},
		providers:       map[string]Provider{},
		runtimes:        map[string]widgetSpecRuntime{},
		manifestMeta:    map[string]ManifestProvider{},
	}
	if err := reg.RegisterArea(WidgetAreaDefinition{Code: "dashboard.hero", Name: "Hero"}); err != nil {
		t.Fatalf("RegisterArea returned error: %v", err)
	}
	if err := reg.RegisterArea(WidgetAreaDefinition{Code: "dashboard.footer", Name: "Footer"}); err != nil {
		t.Fatalf("RegisterArea returned error: %v", err)
	}
	if err := reg.RegisterDefinition(WidgetDefinition{
		Code: "widget.alpha",
		Name: "Alpha",
		Schema: map[string]any{
			"properties": map[string]any{
				"series": []map[string]any{
					{"type": "number"},
				},
			},
		},
	}); err != nil {
		t.Fatalf("RegisterDefinition returned error: %v", err)
	}
	if err := reg.RegisterDefinition(WidgetDefinition{Code: "widget.beta", Name: "Beta"}); err != nil {
		t.Fatalf("RegisterDefinition returned error: %v", err)
	}
	if err := reg.RegisterProvider("widget.alpha", ProviderFunc(func(context.Context, WidgetContext) (WidgetData, error) {
		return WidgetData{"ok": true}, nil
	})); err != nil {
		t.Fatalf("RegisterProvider returned error: %v", err)
	}
	reg.recordProviderMetadata("widget.alpha", ManifestProvider{
		Name:         "Alpha Provider",
		Capabilities: []string{"render"},
	})

	catalog := reg.Catalog()
	if len(catalog.Areas) != 2 || catalog.Areas[0].Code != "dashboard.hero" || catalog.Areas[1].Code != "dashboard.footer" {
		t.Fatalf("expected stable area ordering, got %+v", catalog.Areas)
	}
	if len(catalog.Definitions) != 2 || catalog.Definitions[0].Code != "widget.alpha" || catalog.Definitions[1].Code != "widget.beta" {
		t.Fatalf("expected stable definition ordering, got %+v", catalog.Definitions)
	}
	if len(catalog.Providers) != 2 || catalog.Providers[0].DefinitionCode != "widget.alpha" {
		t.Fatalf("expected provider discovery aligned to definition order, got %+v", catalog.Providers)
	}
	if !catalog.Providers[0].Registered || !catalog.Providers[0].RuntimeBacked {
		t.Fatalf("expected registered runtime-backed provider discovery, got %+v", catalog.Providers[0])
	}
	if catalog.Providers[0].Manifest == nil || catalog.Providers[0].Manifest.Name != "Alpha Provider" {
		t.Fatalf("expected manifest metadata preserved, got %+v", catalog.Providers[0].Manifest)
	}
	if catalog.Providers[1].Registered {
		t.Fatalf("expected widget.beta to remain unregistered in provider discovery, got %+v", catalog.Providers[1])
	}

	catalog.Areas[0].Name = "Mutated"
	catalog.Definitions[0].Schema["properties"].(map[string]any)["series"].([]map[string]any)[0]["type"] = "string"
	catalog.Providers[0].Manifest.Capabilities[0] = "mutated"

	refreshed := reg.Catalog()
	if refreshed.Areas[0].Name != "Hero" {
		t.Fatalf("expected area snapshot cloning, got %+v", refreshed.Areas)
	}
	series := refreshed.Definitions[0].Schema["properties"].(map[string]any)["series"].([]map[string]any)
	if series[0]["type"] != "number" {
		t.Fatalf("expected nested schema cloning, got %+v", refreshed.Definitions[0].Schema)
	}
	if refreshed.Providers[0].Manifest.Capabilities[0] != "render" {
		t.Fatalf("expected manifest metadata cloning, got %+v", refreshed.Providers[0].Manifest.Capabilities)
	}
}

func TestProviderDiscoveryClonesManifestMetadata(t *testing.T) {
	reg := &Registry{
		areas:           map[string]WidgetAreaDefinition{},
		definitions:     map[string]WidgetDefinition{},
		areaOrder:       []string{},
		definitionOrder: []string{},
		providers:       map[string]Provider{},
		runtimes:        map[string]widgetSpecRuntime{},
		manifestMeta:    map[string]ManifestProvider{},
	}
	if err := reg.RegisterDefinition(WidgetDefinition{Code: "widget.discovery", Name: "Discovery"}); err != nil {
		t.Fatalf("RegisterDefinition returned error: %v", err)
	}
	reg.recordProviderMetadata("widget.discovery", ManifestProvider{
		Name:         "Discovery Provider",
		Capabilities: []string{"config"},
	})

	discovery, ok := reg.ProviderDiscovery("widget.discovery")
	if !ok {
		t.Fatalf("expected provider discovery")
	}
	discovery.Manifest.Capabilities[0] = "mutated"

	again, ok := reg.ProviderDiscovery("widget.discovery")
	if !ok {
		t.Fatalf("expected provider discovery on second read")
	}
	if again.Manifest.Capabilities[0] != "config" {
		t.Fatalf("expected manifest metadata to be cloned, got %+v", again.Manifest.Capabilities)
	}
}
