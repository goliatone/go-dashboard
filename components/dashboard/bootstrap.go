package dashboard

import "context"

// RegisterAreas ensures dashboard widget areas exist in go-cms.
func RegisterAreas(ctx context.Context, store WidgetStore) error {
	return nil
}

// RegisterDefinitions registers admin widget definitions.
func RegisterDefinitions(ctx context.Context, store WidgetStore, registry ProviderRegistry) error {
	return nil
}

// SeedLayout creates the starter dashboard widget assignments.
func SeedLayout(ctx context.Context, service *Service) error {
	return nil
}
