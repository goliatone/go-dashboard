package goadmin_test

import (
	"context"
	"testing"

	core "github.com/goliatone/go-dashboard/components/dashboard"
	dashboardpkg "github.com/goliatone/go-dashboard/pkg/dashboard"
	"github.com/goliatone/go-dashboard/pkg/goadmin"
)

type stubMenuBuilder struct {
	calls int
}

func (s *stubMenuBuilder) EnsureMenuItem(context.Context, string, goadmin.MenuItem) error {
	s.calls++
	return nil
}

func TestAdminBootstrapSeedsMenu(t *testing.T) {
	builder := &stubMenuBuilder{}
	service := dashboardpkg.NewService(core.Options{WidgetStore: &stubStore{}})
	admin, err := goadmin.New(goadmin.Config{
		EnableDashboard: true,
		Service:         service,
		MenuBuilder:     builder,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := admin.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}
	if builder.calls != 1 {
		t.Fatalf("expected 1 call, got %d", builder.calls)
	}
	if admin.Dashboard() == nil {
		t.Fatalf("expected dashboard service")
	}
}

func TestAdminDisabledSkipsBootstrap(t *testing.T) {
	builder := &stubMenuBuilder{}
	admin, err := goadmin.New(goadmin.Config{
		EnableDashboard: false,
		MenuBuilder:     builder,
	})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if err := admin.Bootstrap(context.Background()); err != nil {
		t.Fatalf("Bootstrap returned error: %v", err)
	}
	if builder.calls != 0 {
		t.Fatalf("expected 0 calls, got %d", builder.calls)
	}
	if admin.Dashboard() != nil {
		t.Fatalf("expected nil dashboard when disabled")
	}
}

type stubStore struct{}

func (stubStore) EnsureArea(context.Context, core.WidgetAreaDefinition) (bool, error) {
	return true, nil
}
func (stubStore) EnsureDefinition(context.Context, core.WidgetDefinition) (bool, error) {
	return true, nil
}
func (stubStore) CreateInstance(context.Context, core.CreateWidgetInstanceInput) (core.WidgetInstance, error) {
	return core.WidgetInstance{}, nil
}
func (stubStore) DeleteInstance(context.Context, string) error { return nil }
func (stubStore) AssignInstance(context.Context, core.AssignWidgetInput) error {
	return nil
}
func (stubStore) ReorderArea(context.Context, core.ReorderAreaInput) error { return nil }
func (stubStore) ResolveArea(context.Context, core.ResolveAreaInput) (core.ResolvedArea, error) {
	return core.ResolvedArea{}, nil
}
