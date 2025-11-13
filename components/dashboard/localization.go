package dashboard

import (
	"context"
	"strings"
)

// TranslationService exposes locale-aware translation helpers backed by go-cms (or compatible) engines.
// Implementations can provide pluralization, interpolation, or other advanced behaviors while transports
// and providers rely on the lightweight interface defined here.
type TranslationService interface {
	Translate(ctx context.Context, key, locale string, args map[string]any) (string, error)
}

// ResolveLocalizedValue selects the best translation for the provided locale and falls back to the supplied value.
// Keys are matched case-insensitively, and language-region pairs (`es-mx`) automatically fall back to their
// base language (`es`) when present.
func ResolveLocalizedValue(values map[string]string, locale, fallback string) string {
	if len(values) == 0 {
		return fallback
	}
	candidates := localeCandidates(locale)
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		for key, value := range values {
			if strings.EqualFold(key, candidate) && value != "" {
				return value
			}
		}
	}
	if value, ok := values["default"]; ok && value != "" {
		return value
	}
	return fallback
}

func (def *WidgetDefinition) normalizeLocalizedFields() {
	def.NameLocalized = normalizeLocaleMap(def.NameLocalized)
	def.DescriptionLocalized = normalizeLocaleMap(def.DescriptionLocalized)
}

// NameForLocale returns the display name for the requested locale with graceful fallback to the default name.
func (def WidgetDefinition) NameForLocale(locale string) string {
	return ResolveLocalizedValue(def.NameLocalized, locale, def.Name)
}

// DescriptionForLocale returns the localized description if available.
func (def WidgetDefinition) DescriptionForLocale(locale string) string {
	return ResolveLocalizedValue(def.DescriptionLocalized, locale, def.Description)
}

func normalizeLocaleMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	normalized := make(map[string]string, len(values))
	for key, value := range values {
		key = normalizeLocale(key)
		if key == "" || value == "" {
			continue
		}
		normalized[key] = value
	}
	return normalized
}

func localeCandidates(locale string) []string {
	locale = normalizeLocale(locale)
	if locale == "" {
		return []string{"default"}
	}
	candidates := []string{locale}
	if idx := strings.Index(locale, "-"); idx > 0 {
		candidates = append(candidates, locale[:idx])
	}
	candidates = append(candidates, "default")
	return candidates
}

func normalizeLocale(locale string) string {
	return strings.TrimSpace(strings.ToLower(locale))
}

func translateOrFallback(ctx context.Context, svc TranslationService, key, locale, fallback string, params map[string]any) string {
	if svc != nil {
		if translated, err := svc.Translate(ctx, key, locale, params); err == nil && translated != "" {
			return translated
		}
	}
	if fallback != "" {
		return fallback
	}
	return key
}
