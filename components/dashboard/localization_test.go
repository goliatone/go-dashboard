package dashboard

import (
	"context"
	"errors"
	"testing"
)

type stubTranslationService struct {
	value string
	err   error
}

func (s stubTranslationService) Translate(ctx context.Context, key, locale string, args map[string]any) (string, error) {
	return s.value, s.err
}

func TestResolveLocalizedValue(t *testing.T) {
	values := map[string]string{
		"en":    "Dashboard",
		"es":    "Tablero",
		"es-mx": "Panel",
	}
	if got := ResolveLocalizedValue(values, "es-mx", "fallback"); got != "Panel" {
		t.Fatalf("expected region-specific match, got %q", got)
	}
	if got := ResolveLocalizedValue(values, "es-ar", "fallback"); got != "Tablero" {
		t.Fatalf("expected base locale fallback, got %q", got)
	}
	if got := ResolveLocalizedValue(values, "fr", "Dashboard"); got != "Dashboard" {
		t.Fatalf("expected fallback when locale missing, got %q", got)
	}
	if got := ResolveLocalizedValue(nil, "es", "Dashboard"); got != "Dashboard" {
		t.Fatalf("expected fallback when no localized map, got %q", got)
	}
}

func TestTranslateOrFallback(t *testing.T) {
	svc := stubTranslationService{value: "Tablero"}
	out := translateOrFallback(context.Background(), svc, "dashboard.title", "es", "Dashboard", nil)
	if out != "Tablero" {
		t.Fatalf("expected translator value, got %q", out)
	}
	svc = stubTranslationService{err: errors.New("boom")}
	out = translateOrFallback(context.Background(), svc, "dashboard.title", "es", "Dashboard", nil)
	if out != "Dashboard" {
		t.Fatalf("expected fallback on error, got %q", out)
	}
}
