package dashboard

import (
	"context"
	"errors"
	"testing"
)

type memoryStore struct {
	areas       map[string]WidgetAreaDefinition
	defs        map[string]WidgetDefinition
	assignCalls int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		areas: map[string]WidgetAreaDefinition{},
		defs:  map[string]WidgetDefinition{},
	}
}

func (m *memoryStore) EnsureArea(ctx context.Context, def WidgetAreaDefinition) (bool, error) {
	if _, ok := m.areas[def.Code]; ok {
		return false, nil
	}
	m.areas[def.Code] = def
	return true, nil
}

func (m *memoryStore) EnsureDefinition(ctx context.Context, def WidgetDefinition) (bool, error) {
	if _, ok := m.defs[def.Code]; ok {
		return false, nil
	}
	m.defs[def.Code] = def
	return true, nil
}

func (m *memoryStore) CreateInstance(ctx context.Context, input CreateWidgetInstanceInput) (WidgetInstance, error) {
	return WidgetInstance{ID: input.DefinitionID + "-instance", DefinitionID: input.DefinitionID}, nil
}

func (m *memoryStore) DeleteInstance(context.Context, string) error { return nil }

func (m *memoryStore) AssignInstance(context.Context, AssignWidgetInput) error {
	m.assignCalls++
	return nil
}

func (m *memoryStore) ReorderArea(context.Context, ReorderAreaInput) error { return nil }

func (m *memoryStore) ResolveArea(context.Context, ResolveAreaInput) (ResolvedArea, error) {
	return ResolvedArea{Widgets: []WidgetInstance{}}, nil
}

type fakeRegistry struct {
	count int
}

func (f *fakeRegistry) RegisterDefinition(def WidgetDefinition) error {
	if def.Code == "" {
		return errors.New("missing code")
	}
	f.count++
	return nil
}

func (fakeRegistry) RegisterProvider(string, Provider) error { return nil }
func (fakeRegistry) Definition(string) (WidgetDefinition, bool) {
	return WidgetDefinition{}, false
}
func (fakeRegistry) Provider(string) (Provider, bool) { return nil, false }
func (fakeRegistry) Definitions() []WidgetDefinition  { return nil }

func TestRegisterAreasIdempotent(t *testing.T) {
	store := newMemoryStore()
	ctx := context.Background()
	if err := RegisterAreas(ctx, store); err != nil {
		t.Fatalf("RegisterAreas returned error: %v", err)
	}
	firstCount := len(store.areas)
	if firstCount != len(DefaultAreaDefinitions()) {
		t.Fatalf("expected %d areas, got %d", len(DefaultAreaDefinitions()), firstCount)
	}
	if err := RegisterAreas(ctx, store); err != nil {
		t.Fatalf("RegisterAreas second run returned error: %v", err)
	}
	if len(store.areas) != firstCount {
		t.Fatalf("expected idempotent area registration")
	}
}

func TestRegisterDefinitionsRegistersRegistry(t *testing.T) {
	store := newMemoryStore()
	reg := &fakeRegistry{}
	if err := RegisterDefinitions(context.Background(), store, reg); err != nil {
		t.Fatalf("RegisterDefinitions returned error: %v", err)
	}
	if len(store.defs) != len(DefaultWidgetDefinitions()) {
		t.Fatalf("expected %d defs, got %d", len(DefaultWidgetDefinitions()), len(store.defs))
	}
	if reg.count != len(DefaultWidgetDefinitions()) {
		t.Fatalf("expected registry to receive %d defs, got %d", len(DefaultWidgetDefinitions()), reg.count)
	}
}

func TestSeedLayoutAddsWidgets(t *testing.T) {
	store := newMemoryStore()
	service := NewService(Options{WidgetStore: store})
	if err := SeedLayout(context.Background(), service); err != nil {
		t.Fatalf("SeedLayout returned error: %v", err)
	}
	if store.assignCalls != len(DefaultSeedWidgets()) {
		t.Fatalf("expected %d assign calls, got %d", len(DefaultSeedWidgets()), store.assignCalls)
	}
}
