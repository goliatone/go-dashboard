package dashboard

import (
	"context"
	"net/http/httptest"
	"testing"
)

func TestBroadcastHookSubscribe(t *testing.T) {
	hook := NewBroadcastHook()
	ch, cancel := hook.Subscribe()
	defer cancel()
	event := WidgetEvent{AreaCode: "admin.dashboard.main"}
	if err := hook.WidgetUpdated(context.Background(), event); err != nil {
		t.Fatalf("WidgetUpdated returned error: %v", err)
	}
	select {
	case e := <-ch:
		if e.AreaCode != event.AreaCode {
			t.Fatalf("expected area %s, got %s", event.AreaCode, e.AreaCode)
		}
	default:
		t.Fatalf("expected event to be delivered")
	}
}

func TestBroadcastHookServeSSETerminatesOnContext(t *testing.T) {
	hook := NewBroadcastHook()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/sse", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	go hook.ServeSSE(rec, req)
	cancel()
}
