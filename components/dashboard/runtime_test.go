package dashboard

import (
	"context"
	"testing"
)

func TestNewRuntimeBuildsCanonicalDefaults(t *testing.T) {
	store := &fakeWidgetStore{}
	runtime := NewRuntime(RuntimeOptions{
		Service: Options{
			WidgetStore: store,
		},
		Controller: ControllerOptions{
			Template: "runtime.html",
			Renderer: &stubRenderer{},
		},
	})
	if runtime == nil {
		t.Fatalf("expected runtime")
	}
	if runtime.Service == nil || runtime.Controller == nil || runtime.API == nil {
		t.Fatalf("expected runtime collaborators, got %+v", runtime)
	}
	if runtime.Broadcast == nil {
		t.Fatalf("expected default broadcast hook")
	}
	if err := runtime.API.Assign(context.Background(), AddWidgetRequest{
		DefinitionID: "admin.widget.user_stats",
		AreaCode:     "admin.dashboard.main",
		Configuration: map[string]any{
			"metric": "total",
		},
	}); err != nil {
		t.Fatalf("expected default executor to be service-backed, got %v", err)
	}
}
