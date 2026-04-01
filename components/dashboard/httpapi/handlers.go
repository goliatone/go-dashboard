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

// Layout resolves the canonical layout payload through the shared controller.
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
