package dashboard

import (
	"fmt"
	"maps"
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
	areas           map[string]WidgetAreaDefinition
	mu              sync.RWMutex
	definitions     map[string]WidgetDefinition
	areaOrder       []string
	definitionOrder []string
	providers       map[string]Provider
	runtimes        map[string]widgetSpecRuntime
	manifestMeta    map[string]ManifestProvider
}

// NewRegistry builds an empty registry and applies global hooks.
func NewRegistry() *Registry {
	reg := &Registry{
		areas:           map[string]WidgetAreaDefinition{},
		definitions:     map[string]WidgetDefinition{},
		areaOrder:       []string{},
		definitionOrder: []string{},
		providers:       map[string]Provider{},
		runtimes:        map[string]widgetSpecRuntime{},
		manifestMeta:    map[string]ManifestProvider{},
	}
	reg.registerDefaults()
	_ = reg.ApplyHooks()
	return reg
}

func (r *Registry) registerDefaults() {
	for _, area := range DefaultAreaDefinitions() {
		_ = r.RegisterArea(area)
	}
	for _, def := range DefaultWidgetDefinitions() {
		_ = r.RegisterDefinition(def)
	}
	registerDefaultWidgetRuntimes(r)
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

// RegisterArea stores widget area metadata.
func (r *Registry) RegisterArea(area WidgetAreaDefinition) error {
	if area.Code == "" {
		return fmt.Errorf("widget area code is required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.areas[area.Code]; !ok {
		r.areaOrder = append(r.areaOrder, area.Code)
	}
	r.areas[area.Code] = cloneWidgetAreaDefinition(area)
	return nil
}

// Area fetches a widget area definition by code.
func (r *Registry) Area(code string) (WidgetAreaDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	area, ok := r.areas[code]
	return cloneWidgetAreaDefinition(area), ok
}

// Areas returns all registered areas in stable registration order.
func (r *Registry) Areas() []WidgetAreaDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	areas := make([]WidgetAreaDefinition, 0, len(r.areaOrder))
	for _, code := range r.areaOrder {
		area, ok := r.areas[code]
		if !ok {
			continue
		}
		areas = append(areas, cloneWidgetAreaDefinition(area))
	}
	return areas
}

// RegisterDefinition stores widget metadata.
func (r *Registry) RegisterDefinition(def WidgetDefinition) error {
	if def.Code == "" {
		return fmt.Errorf("widget definition code is required")
	}
	def.normalizeLocalizedFields()
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.definitions[def.Code]; !ok {
		r.definitionOrder = append(r.definitionOrder, def.Code)
	}
	r.definitions[def.Code] = cloneWidgetDefinition(def)
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
	def, ok := r.definitions[code]
	if !ok {
		return fmt.Errorf("widget definition %s not found", code)
	}
	r.providers[code] = provider
	if runtimeProvider, ok := provider.(runtimeProviderAdapter); ok && runtimeProvider.runtime != nil {
		r.runtimes[code] = runtimeProvider.runtime
	} else {
		r.runtimes[code] = newProviderRuntimeAdapter(code, def, provider)
	}
	return nil
}

// Definition fetches a widget definition by code.
func (r *Registry) Definition(code string) (WidgetDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	def, ok := r.definitions[code]
	return cloneWidgetDefinition(def), ok
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
	return cloneManifestProvider(meta), ok
}

// Definitions returns all registered definitions.
func (r *Registry) Definitions() []WidgetDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	defs := make([]WidgetDefinition, 0, len(r.definitionOrder))
	for _, code := range r.definitionOrder {
		def, ok := r.definitions[code]
		if !ok {
			continue
		}
		defs = append(defs, cloneWidgetDefinition(def))
	}
	return defs
}

// Clone returns an immutable snapshot of the current registry state.
func (r *Registry) Clone() *Registry {
	if r == nil {
		return nil
	}
	snapshot := &Registry{
		areas:        map[string]WidgetAreaDefinition{},
		definitions:  map[string]WidgetDefinition{},
		providers:    map[string]Provider{},
		runtimes:     map[string]widgetSpecRuntime{},
		manifestMeta: map[string]ManifestProvider{},
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	snapshot.areaOrder = append([]string{}, r.areaOrder...)
	snapshot.definitionOrder = append([]string{}, r.definitionOrder...)
	for code, area := range r.areas {
		snapshot.areas[code] = cloneWidgetAreaDefinition(area)
	}
	for code, def := range r.definitions {
		snapshot.definitions[code] = cloneWidgetDefinition(def)
	}
	maps.Copy(snapshot.providers, r.providers)
	maps.Copy(snapshot.runtimes, r.runtimes)
	for code, meta := range r.manifestMeta {
		snapshot.manifestMeta[code] = cloneManifestProvider(meta)
	}
	return snapshot
}

func (r *Registry) registerRuntime(code string, runtime widgetSpecRuntime) error {
	if code == "" {
		return fmt.Errorf("widget definition code is required to register runtime")
	}
	if runtime == nil {
		return fmt.Errorf("widget runtime cannot be nil")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.definitions[code]; !ok {
		return fmt.Errorf("widget definition %s not found", code)
	}
	r.runtimes[code] = runtime
	r.providers[code] = runtimeProviderAdapter{runtime: runtime}
	return nil
}

func (r *Registry) widgetRuntime(code string) (widgetSpecRuntime, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runtime, ok := r.runtimes[code]
	return runtime, ok
}

func (r *Registry) recordProviderMetadata(code string, meta ManifestProvider) {
	if meta.isZero() {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.manifestMeta[code] = cloneManifestProvider(meta)
}

func cloneWidgetDefinition(def WidgetDefinition) WidgetDefinition {
	def.NameLocalized = cloneLocalizedFields(def.NameLocalized)
	def.DescriptionLocalized = cloneLocalizedFields(def.DescriptionLocalized)
	def.Schema = cloneAnyMap(def.Schema)
	return def
}

func cloneLocalizedFields(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneAnyValue(value)
	}
	return out
}
