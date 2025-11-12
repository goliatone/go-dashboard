package dashboard

import (
	"context"
	"errors"
	"fmt"
)

// RegisterAreas ensures dashboard widget areas exist in go-cms.
func RegisterAreas(ctx context.Context, store WidgetStore) error {
	if store == nil {
		return errMissingWidgetStore
	}
	for _, area := range DefaultAreaDefinitions() {
		if _, err := store.EnsureArea(ctx, area); err != nil {
			return fmt.Errorf("register area %s: %w", area.Code, err)
		}
	}
	return nil
}

// RegisterDefinitions registers admin widget definitions.
func RegisterDefinitions(ctx context.Context, store WidgetStore, registry ProviderRegistry) error {
	if store == nil {
		return errMissingWidgetStore
	}
	for _, def := range DefaultWidgetDefinitions() {
		if _, err := store.EnsureDefinition(ctx, def); err != nil {
			return fmt.Errorf("register definition %s: %w", def.Code, err)
		}
		if registry != nil {
			if err := registry.RegisterDefinition(def); err != nil {
				return fmt.Errorf("register definition in registry %s: %w", def.Code, err)
			}
		}
	}
	return nil
}

// SeedLayout creates the starter dashboard widget assignments.
func SeedLayout(ctx context.Context, service *Service) error {
	if service == nil {
		return errors.New("dashboard: service is required to seed layout")
	}
	var seedErr error
	for _, req := range DefaultSeedWidgets() {
		if err := service.AddWidget(ctx, req); err != nil {
			seedErr = errors.Join(seedErr, err)
		}
	}
	return seedErr
}
