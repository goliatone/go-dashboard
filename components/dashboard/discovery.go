package dashboard

// CatalogSnapshot exposes the typed discovery state needed by diagnostics,
// tooling, and host integrations.
type CatalogSnapshot struct {
	Areas       []WidgetAreaDefinition `json:"areas,omitempty"`
	Definitions []WidgetDefinition     `json:"definitions,omitempty"`
	Providers   []ProviderDiscovery    `json:"providers,omitempty"`
}

// ProviderDiscovery captures stable provider/discovery metadata for a widget.
type ProviderDiscovery struct {
	DefinitionCode string            `json:"definition_code"`
	Registered     bool              `json:"registered"`
	RuntimeBacked  bool              `json:"runtime_backed,omitempty"`
	Manifest       *ManifestProvider `json:"manifest,omitempty"`
}

// Catalog returns an immutable, ordered snapshot of the registry discovery
// state for tooling and diagnostics consumers.
func (r *Registry) Catalog() CatalogSnapshot {
	if r == nil {
		return CatalogSnapshot{}
	}
	definitions := r.Definitions()
	providers := make([]ProviderDiscovery, 0, len(definitions))
	for _, def := range definitions {
		discovery, _ := r.ProviderDiscovery(def.Code)
		providers = append(providers, discovery)
	}
	return CatalogSnapshot{
		Areas:       r.Areas(),
		Definitions: definitions,
		Providers:   providers,
	}
}

// ProviderDiscovery returns the typed provider/discovery state for a widget
// definition code.
func (r *Registry) ProviderDiscovery(code string) (ProviderDiscovery, bool) {
	if r == nil || code == "" {
		return ProviderDiscovery{}, false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, defOK := r.definitions[code]
	provider, providerOK := r.providers[code]
	runtime, runtimeOK := r.runtimes[code]
	manifest, manifestOK := r.manifestMeta[code]
	if !defOK && !providerOK && !manifestOK && !runtimeOK {
		return ProviderDiscovery{}, false
	}
	discovery := ProviderDiscovery{
		DefinitionCode: code,
		Registered:     providerOK && provider != nil,
		RuntimeBacked:  runtimeOK && runtime != nil,
	}
	if manifestOK {
		cloned := cloneManifestProvider(manifest)
		discovery.Manifest = &cloned
	}
	return discovery, true
}
