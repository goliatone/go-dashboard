package dashboard

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync/atomic"
	"testing"
	"time"
)

type fixedWidgetRuntime struct {
	code string
	view WidgetViewModel
	err  error
}

func (runtime fixedWidgetRuntime) Code() string {
	return runtime.code
}

func (runtime fixedWidgetRuntime) Definition() WidgetDefinition {
	return WidgetDefinition{Code: runtime.code}
}

func (runtime fixedWidgetRuntime) Resolve(context.Context, WidgetContext) (ResolvedWidget, error) {
	if runtime.err != nil {
		return ResolvedWidget{}, runtime.err
	}
	return ResolvedWidget{View: runtime.view}, nil
}

type trackingViewModel struct {
	calls   *int32
	payload any
	err     error
}

func (view trackingViewModel) Serialize() (any, error) {
	if view.calls != nil {
		atomic.AddInt32(view.calls, 1)
	}
	if view.err != nil {
		return nil, view.err
	}
	return view.payload, nil
}

type panicRenderer struct {
	called bool
}

func (renderer *panicRenderer) RenderPage(_ string, _ Page, _ ...io.Writer) (string, error) {
	renderer.called = true
	return "", nil
}

func TestConfigureLayoutDefersWidgetSerializationUntilPageAssembly(t *testing.T) {
	store := &fakeWidgetStore{
		resolved: map[string][]WidgetInstance{
			"admin.dashboard.main": {
				{
					ID:           "runtime-1",
					DefinitionID: "custom.widget.runtime",
					AreaCode:     "admin.dashboard.main",
				},
			},
		},
	}
	registry := NewRegistry()
	if err := registry.RegisterDefinition(WidgetDefinition{Code: "custom.widget.runtime"}); err != nil {
		t.Fatalf("RegisterDefinition returned error: %v", err)
	}
	var serializeCalls int32
	if err := registry.registerRuntime("custom.widget.runtime", fixedWidgetRuntime{
		code: "custom.widget.runtime",
		view: trackingViewModel{
			calls:   &serializeCalls,
			payload: map[string]any{"value": 42},
		},
	}); err != nil {
		t.Fatalf("registerRuntime returned error: %v", err)
	}

	service := NewService(Options{
		WidgetStore:     store,
		Providers:       registry,
		PreferenceStore: NewInMemoryPreferenceStore(),
	})
	layout, err := service.ConfigureLayout(context.Background(), ViewerContext{UserID: "user-1"})
	if err != nil {
		t.Fatalf("ConfigureLayout returned error: %v", err)
	}
	if atomic.LoadInt32(&serializeCalls) != 0 {
		t.Fatalf("expected widget serialization deferred until page assembly")
	}
	widget := layout.Areas["admin.dashboard.main"][0]
	if _, ok := widget.Metadata[widgetViewModelMetadataKey].(WidgetViewModel); !ok {
		t.Fatalf("expected widget metadata to store a WidgetViewModel")
	}
	if _, ok := widget.Metadata["data"]; ok {
		t.Fatalf("expected service layout metadata to avoid serialized widget data")
	}

	controller := NewController(ControllerOptions{
		Service: &stubLayoutResolver{layout: layout},
	})
	payload, err := controller.LayoutPayload(context.Background(), ViewerContext{UserID: "user-1"})
	if err != nil {
		t.Fatalf("LayoutPayload returned error: %v", err)
	}
	if atomic.LoadInt32(&serializeCalls) != 1 {
		t.Fatalf("expected widget serialization at page assembly, got %d calls", serializeCalls)
	}
	areas := payload["areas"].(map[string]any)
	main := areas["main"].(map[string]any)
	widgets := main["widgets"].([]map[string]any)
	if widgets[0]["data"].(map[string]any)["value"] != 42 {
		t.Fatalf("expected serialized widget payload available at page assembly, got %#v", widgets[0]["data"])
	}
}

func TestRegisterProviderPreservesRuntimeBackedWidgetSpecs(t *testing.T) {
	store := &fakeWidgetStore{
		resolved: map[string][]WidgetInstance{
			"admin.dashboard.main": {
				{
					ID:           "runtime-provider-1",
					DefinitionID: "custom.widget.runtime_provider",
					AreaCode:     "admin.dashboard.main",
				},
			},
		},
	}
	registry := NewRegistry()
	if err := registry.RegisterDefinition(WidgetDefinition{Code: "custom.widget.runtime_provider"}); err != nil {
		t.Fatalf("RegisterDefinition returned error: %v", err)
	}
	var serializeCalls int32
	provider := NewWidgetProvider(WidgetSpec[struct{}, int, trackingViewModel]{
		Definition: WidgetDefinition{Code: "custom.widget.runtime_provider"},
		Fetch: func(context.Context, WidgetRequest[struct{}]) (int, error) {
			return 99, nil
		},
		BuildView: func(_ context.Context, data int, _ WidgetViewContext[struct{}]) (trackingViewModel, error) {
			return trackingViewModel{
				calls:   &serializeCalls,
				payload: map[string]any{"value": data},
			}, nil
		},
	})
	if err := registry.RegisterProvider("custom.widget.runtime_provider", provider); err != nil {
		t.Fatalf("RegisterProvider returned error: %v", err)
	}

	service := NewService(Options{
		WidgetStore:     store,
		Providers:       registry,
		PreferenceStore: NewInMemoryPreferenceStore(),
	})
	layout, err := service.ConfigureLayout(context.Background(), ViewerContext{UserID: "user-2"})
	if err != nil {
		t.Fatalf("ConfigureLayout returned error: %v", err)
	}
	if atomic.LoadInt32(&serializeCalls) != 0 {
		t.Fatalf("expected RegisterProvider to preserve the runtime-backed provider without eager serialization")
	}
	widget := layout.Areas["admin.dashboard.main"][0]
	if _, ok := widget.Metadata[widgetViewModelMetadataKey].(WidgetViewModel); !ok {
		t.Fatalf("expected runtime-backed provider registration to keep WidgetViewModel metadata")
	}
}

func TestWidgetSerializeFailuresSurfaceThroughControllerAndTransport(t *testing.T) {
	layout := Layout{
		Areas: map[string][]WidgetInstance{
			"admin.dashboard.main": {
				{
					ID:           "broken-1",
					DefinitionID: "custom.widget.broken",
					AreaCode:     "admin.dashboard.main",
					Metadata: map[string]any{
						widgetViewModelMetadataKey: trackingViewModel{
							err: errors.New("serialize failed"),
						},
					},
				},
			},
		},
	}
	renderer := &panicRenderer{}
	controller := NewController(ControllerOptions{
		Service:  &stubLayoutResolver{layout: layout},
		Renderer: renderer,
	})

	if _, err := controller.LayoutPayload(context.Background(), ViewerContext{UserID: "user-1"}); err == nil {
		t.Fatalf("expected LayoutPayload to return serialization error")
	}
	var buf bytes.Buffer
	if err := controller.RenderTemplate(context.Background(), ViewerContext{UserID: "user-1"}, &buf); err == nil {
		t.Fatalf("expected RenderTemplate to return serialization error")
	}
	if renderer.called {
		t.Fatalf("renderer should not be invoked when page assembly fails")
	}
}

func TestSerializedWidgetDataHonorsJSONMarshalerValues(t *testing.T) {
	payload, err := serializedWidgetData(struct {
		GeneratedAt time.Time `json:"generated_at"`
	}{
		GeneratedAt: time.Date(2026, time.April, 6, 12, 30, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("serializedWidgetData returned error: %v", err)
	}
	if payload["generated_at"] != "2026-04-06T12:30:00Z" {
		t.Fatalf("expected time.Time to serialize through MarshalJSON, got %#v", payload["generated_at"])
	}
}
