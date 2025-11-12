package dashboard

import (
	"fmt"
	"sync"
)

// WidgetHook lets packages register widgets/providers during init().
type WidgetHook func(reg *Registry) error

var (
	globalHookMu sync.Mutex
	globalHooks  []WidgetHook
)

// RegisterWidgetHook registers a hook executed against new registries.
func RegisterWidgetHook(h WidgetHook) {
	globalHookMu.Lock()
	defer globalHookMu.Unlock()
	globalHooks = append(globalHooks, h)
}

// WidgetManifest represents config-driven registration entries.
type WidgetManifest struct {
	Definition WidgetDefinition
	Provider   Provider
}

// Registry implements ProviderRegistry with hook + manifest support.
type Registry struct {
	mu          sync.RWMutex
	definitions map[string]WidgetDefinition
	providers   map[string]Provider
	manifestMeta map[string]ManifestProvider
}

// NewRegistry builds an empty registry and applies global hooks.
func NewRegistry() *Registry {
	reg := &Registry{
		definitions:  map[string]WidgetDefinition{},
		providers:    map[string]Provider{},
		manifestMeta: map[string]ManifestProvider{},
	}
	reg.registerDefaults()
	_ = reg.ApplyHooks()
	return reg
}

func (r *Registry) registerDefaults() {
	for _, def := range DefaultWidgetDefinitions() {
		_ = r.RegisterDefinition(def)
		if provider, ok := defaultProviders[def.Code]; ok {
			_ = r.RegisterProvider(def.Code, provider)
		}
	}
}

// ApplyHooks executes registered widget hooks.
func (r *Registry) ApplyHooks() error {
	globalHookMu.Lock()
	defer globalHookMu.Unlock()
	for _, hook := range globalHooks {
		if err := hook(r); err != nil {
			return err
		}
	}
	return nil
}

// LoadManifest registers definitions/providers from config manifests.
func (r *Registry) LoadManifest(items []WidgetManifest) error {
	for _, item := range items {
		if err := r.RegisterDefinition(item.Definition); err != nil {
			return err
		}
		if item.Provider != nil {
			if err := r.RegisterProvider(item.Definition.Code, item.Provider); err != nil {
				return err
			}
		}
	}
	return nil
}

// RegisterDefinition stores widget metadata.
func (r *Registry) RegisterDefinition(def WidgetDefinition) error {
	if def.Code == "" {
		return fmt.Errorf("widget definition code is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.definitions[def.Code] = def
	return nil
}

// RegisterProvider associates a provider implementation with a definition.
func (r *Registry) RegisterProvider(code string, provider Provider) error {
	if code == "" {
		return fmt.Errorf("widget definition code is required to register provider")
	}
	if provider == nil {
		return fmt.Errorf("provider cannot be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.definitions[code]; !ok {
		return fmt.Errorf("widget definition %s not found", code)
	}
	r.providers[code] = provider
	return nil
}

// Definition fetches a widget definition by code.
func (r *Registry) Definition(code string) (WidgetDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.definitions[code]
	return def, ok
}

// Provider fetches a widget provider by code.
func (r *Registry) Provider(code string) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	provider, ok := r.providers[code]
	return provider, ok
}

// ProviderMetadata returns any manifest metadata registered for a widget.
func (r *Registry) ProviderMetadata(code string) (ManifestProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	meta, ok := r.manifestMeta[code]
	return meta, ok
}

// Definitions returns all registered definitions.
func (r *Registry) Definitions() []WidgetDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]WidgetDefinition, 0, len(r.definitions))
	for _, def := range r.definitions {
		defs = append(defs, def)
	}
	return defs
}

func (r *Registry) recordProviderMetadata(code string, meta ManifestProvider) {
	if meta.isZero() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manifestMeta[code] = meta
}
