package queries

import (
	"context"
	"testing"

	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

type stubLayoutService struct {
	calls int
}

func (s *stubLayoutService) ConfigureLayout(context.Context, dashboard.ViewerContext) (dashboard.Layout, error) {
	s.calls++
	return dashboard.Layout{Areas: map[string][]dashboard.WidgetInstance{}}, nil
}

type stubAreaService struct {
	calls int
}

func (s *stubAreaService) ResolveArea(context.Context, dashboard.ViewerContext, string) (dashboard.ResolvedArea, error) {
	s.calls++
	return dashboard.ResolvedArea{}, nil
}

func TestLayoutQuery(t *testing.T) {
	service := &stubLayoutService{}
	query := NewLayoutQuery(service)
	_, err := query.Query(context.Background(), dashboard.ViewerContext{})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if service.calls != 1 {
		t.Fatalf("expected 1 call, got %d", service.calls)
	}
}

func TestWidgetAreaQuery(t *testing.T) {
	service := &stubAreaService{}
	query := NewWidgetAreaQuery(service)
	_, err := query.Query(context.Background(), WidgetAreaInput{AreaCode: "admin.dashboard.main"})
	if err != nil {
		t.Fatalf("Query returned error: %v", err)
	}
	if service.calls != 1 {
		t.Fatalf("expected 1 call, got %d", service.calls)
	}
}
