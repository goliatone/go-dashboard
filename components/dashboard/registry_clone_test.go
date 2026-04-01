package dashboard

import (
	"context"
	"testing"
)

func TestRegistryClonePreservesProvidersDefinitionsAndMetadata(t *testing.T) {
	reg := NewRegistry()
	if err := reg.RegisterDefinition(WidgetDefinition{
		Code:                 "clone.widget",
		Name:                 "Clone Widget",
		Schema:               map[string]any{"type": "object"},
		NameLocalized:        map[string]string{"es": "Clonar"},
		Description:          "clone me",
		DescriptionLocalized: map[string]string{"es": "cloname"},
	}); err != nil {
		t.Fatalf("register definition: %v", err)
	}
	provider := ProviderFunc(func(context.Context, WidgetContext) (WidgetData, error) {
		return WidgetData{"ok": true}, nil
	})
	if err := reg.RegisterProvider("clone.widget", provider); err != nil {
		t.Fatalf("register provider: %v", err)
	}
	reg.recordProviderMetadata("clone.widget", ManifestProvider{
		Name:         "Clone Provider",
		Capabilities: []string{"render"},
	})

	clone := reg.Clone()
	if clone == nil {
		t.Fatalf("expected clone")
	}
	def, ok := clone.Definition("clone.widget")
	if !ok || def.Name != "Clone Widget" {
		t.Fatalf("expected cloned definition, got %+v", def)
	}
	clonedProvider, ok := clone.Provider("clone.widget")
	if !ok || clonedProvider == nil {
		t.Fatalf("expected cloned provider")
	}
	meta, ok := clone.ProviderMetadata("clone.widget")
	if !ok || meta.Name != "Clone Provider" {
		t.Fatalf("expected cloned metadata, got %+v", meta)
	}

	if err := clone.RegisterDefinition(WidgetDefinition{Code: "clone.extra", Name: "Extra"}); err != nil {
		t.Fatalf("register definition on clone: %v", err)
	}
	if _, ok := reg.Definition("clone.extra"); ok {
		t.Fatalf("expected original registry to stay isolated from clone mutations")
	}

	def.Schema["type"] = "array"
	meta.Capabilities[0] = "mutated"
	again, _ := reg.Definition("clone.widget")
	if again.Schema["type"] != "object" {
		t.Fatalf("expected definition maps to be cloned, got %+v", again.Schema)
	}
	againMeta, _ := reg.ProviderMetadata("clone.widget")
	if againMeta.Capabilities[0] != "render" {
		t.Fatalf("expected provider metadata slice to be cloned, got %+v", againMeta.Capabilities)
	}
}
