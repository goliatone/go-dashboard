package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/goliatone/go-dashboard/components/dashboard"
)

// Controller exposes the shared controller operations used by transport helpers.
type Controller interface {
	RenderTemplate(ctx context.Context, viewer dashboard.ViewerContext, out *bytes.Buffer) error
	Page(ctx context.Context, viewer dashboard.ViewerContext) (dashboard.Page, error)
	LayoutPayload(ctx context.Context, viewer dashboard.ViewerContext) (map[string]any, error)
}

// Response captures a transport-agnostic status/payload pair.
type Response struct {
	StatusCode int
	Payload    any
}

// RenderHTML resolves and renders dashboard HTML through the shared controller.
func RenderHTML(ctx context.Context, controller *dashboard.Controller, viewer dashboard.ViewerContext) ([]byte, error) {
	if controller == nil {
		return nil, errors.New("dashboard: controller not configured")
	}
	var buf bytes.Buffer
	if err := controller.RenderTemplate(ctx, viewer, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Page resolves the canonical typed dashboard page through the shared
// controller. JSON transports should prefer this over legacy payload adapters.
func Page(ctx context.Context, controller *dashboard.Controller, viewer dashboard.ViewerContext) (dashboard.Page, error) {
	if controller == nil {
		return dashboard.Page{}, errors.New("dashboard: controller not configured")
	}
	return controller.Page(ctx, viewer)
}

// Layout resolves the legacy layout payload adapter through the shared
// controller. It remains available only for migration compatibility.
func Layout(ctx context.Context, controller *dashboard.Controller, viewer dashboard.ViewerContext) (map[string]any, error) {
	if controller == nil {
		return nil, errors.New("dashboard: controller not configured")
	}
	return controller.LayoutPayload(ctx, viewer)
}

// Assign creates a widget and returns the canonical response envelope.
func Assign(ctx context.Context, api dashboard.Executor, req dashboard.AddWidgetRequest) (Response, error) {
	if api == nil {
		return Response{}, errors.New("dashboard: executor not configured")
	}
	if err := api.Assign(ctx, req); err != nil {
		return Response{}, err
	}
	return Response{StatusCode: 201, Payload: map[string]string{"status": "created"}}, nil
}

// Remove deletes a widget and returns the canonical response envelope.
func Remove(ctx context.Context, api dashboard.Executor, input dashboard.RemoveWidgetInput) (Response, error) {
	if api == nil {
		return Response{}, errors.New("dashboard: executor not configured")
	}
	if err := api.Remove(ctx, input); err != nil {
		return Response{}, err
	}
	return Response{StatusCode: 204, Payload: map[string]string{"status": "removed"}}, nil
}

// Reorder updates widget ordering and returns the canonical response envelope.
func Reorder(ctx context.Context, api dashboard.Executor, input dashboard.ReorderWidgetsInput) (Response, error) {
	if api == nil {
		return Response{}, errors.New("dashboard: executor not configured")
	}
	if err := api.Reorder(ctx, input); err != nil {
		return Response{}, err
	}
	return Response{StatusCode: 200, Payload: map[string]string{"status": "reordered"}}, nil
}

// Refresh notifies refresh subscribers and returns the canonical response envelope.
func Refresh(ctx context.Context, api dashboard.Executor, input dashboard.RefreshWidgetInput) (Response, error) {
	if api == nil {
		return Response{}, errors.New("dashboard: executor not configured")
	}
	if err := api.Refresh(ctx, input); err != nil {
		return Response{}, err
	}
	return Response{StatusCode: 202, Payload: map[string]string{"status": "queued"}}, nil
}

// Preferences saves layout overrides and returns the canonical response envelope.
func Preferences(ctx context.Context, api dashboard.Executor, input dashboard.SaveLayoutPreferencesInput) (Response, error) {
	if api == nil {
		return Response{}, errors.New("dashboard: executor not configured")
	}
	if err := api.Preferences(ctx, input); err != nil {
		return Response{}, err
	}
	return Response{StatusCode: 200, Payload: map[string]string{"status": "saved"}}, nil
}

// PreferencesInputFromJSON decodes a transport payload and injects the resolved viewer.
func PreferencesInputFromJSON(body []byte, viewer dashboard.ViewerContext) (dashboard.SaveLayoutPreferencesInput, error) {
	var input dashboard.SaveLayoutPreferencesInput
	if len(body) == 0 {
		input.Viewer = viewer
		return input, nil
	}
	if err := json.Unmarshal(body, &input); err != nil {
		return dashboard.SaveLayoutPreferencesInput{}, err
	}
	input.Viewer = viewer
	return input, nil
}

// PreferencesInputFromMap converts a generic map payload into the canonical preferences input.
func PreferencesInputFromMap(body map[string]any, viewer dashboard.ViewerContext) (dashboard.SaveLayoutPreferencesInput, error) {
	if len(body) == 0 {
		return dashboard.SaveLayoutPreferencesInput{Viewer: viewer}, nil
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return dashboard.SaveLayoutPreferencesInput{}, err
	}
	return PreferencesInputFromJSON(raw, viewer)
}

// PreferencesInputFromJSONCompatible decodes canonical preference payloads and,
// when explicitly present, the temporary legacy `{"layout":[...]}` payload
// adapter.
func PreferencesInputFromJSONCompatible(body []byte, viewer dashboard.ViewerContext) (dashboard.SaveLayoutPreferencesInput, error) {
	if len(body) == 0 {
		return dashboard.SaveLayoutPreferencesInput{Viewer: viewer}, nil
	}
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return dashboard.SaveLayoutPreferencesInput{}, err
	}
	return PreferencesInputFromMapCompatible(raw, viewer)
}

// PreferencesInputFromMapCompatible converts canonical preference payloads and,
// when explicitly present, the temporary legacy layout-array payload adapter.
func PreferencesInputFromMapCompatible(body map[string]any, viewer dashboard.ViewerContext) (dashboard.SaveLayoutPreferencesInput, error) {
	if isLegacyLayoutPreferencesPayload(body) {
		return LegacyPreferencesInputFromMap(body, viewer)
	}
	return PreferencesInputFromMap(body, viewer)
}

// LegacyPreferencesInputFromMap converts the deprecated layout-array payload
// into the canonical typed preferences contract. Remove once downstream callers
// stop sending `{"layout":[...]}` request bodies.
func LegacyPreferencesInputFromMap(body map[string]any, viewer dashboard.ViewerContext) (dashboard.SaveLayoutPreferencesInput, error) {
	if len(body) == 0 {
		return dashboard.SaveLayoutPreferencesInput{Viewer: viewer}, nil
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return dashboard.SaveLayoutPreferencesInput{}, err
	}
	var input dashboard.LegacyLayoutPreferencesInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return dashboard.SaveLayoutPreferencesInput{}, err
	}
	if _, ok := body["layout"]; ok && len(input.Layout) == 0 {
		return dashboard.SaveLayoutPreferencesInput{}, errors.New("dashboard: legacy layout payload is empty")
	}
	return input.ToSaveLayoutPreferencesInput(viewer), nil
}

func isLegacyLayoutPreferencesPayload(body map[string]any) bool {
	if len(body) == 0 {
		return false
	}
	_, hasLayout := body["layout"]
	if !hasLayout {
		return false
	}
	return !hasCanonicalPreferencesPayload(body)
}

func hasCanonicalPreferencesPayload(body map[string]any) bool {
	if len(body) == 0 {
		return false
	}
	if _, ok := body["area_order"]; ok {
		return true
	}
	if _, ok := body["layout_rows"]; ok {
		return true
	}
	if _, ok := body["hidden_widget_ids"]; ok {
		return true
	}
	if _, ok := body["viewer"]; ok {
		return true
	}
	return false
}
