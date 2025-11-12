package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/goliatone/go-dashboard/components/dashboard"
	"github.com/goliatone/go-dashboard/components/dashboard/commands"
)

type stubCommander[T any] struct {
	last  T
	calls int
	err   error
}

func (s *stubCommander[T]) Execute(ctx context.Context, msg T) error {
	s.last = msg
	s.calls++
	return s.err
}

func TestHandleAssignWidget(t *testing.T) {
	assign := &stubCommander[dashboard.AddWidgetRequest]{}
	api := &Handlers{Assign: assign}
	payload := dashboard.AddWidgetRequest{DefinitionID: "admin.widget.user_stats", AreaCode: "admin.dashboard.main"}
	buf, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/widgets", bytes.NewReader(buf))
	rec := httptest.NewRecorder()
	api.HandleAssignWidget(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if assign.calls != 1 {
		t.Fatalf("expected assign to execute")
	}
}

func TestHandleRemoveWidget(t *testing.T) {
	remove := &stubCommander[commands.RemoveWidgetInput]{}
	api := &Handlers{Remove: remove}
	req := httptest.NewRequest(http.MethodDelete, "/widgets/w1", nil)
	rec := httptest.NewRecorder()
	api.HandleRemoveWidget(rec, req, "w1")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if remove.last.WidgetID != "w1" {
		t.Fatalf("expected widget id propagation")
	}
}

func TestHandleReorderWidgets(t *testing.T) {
	reorder := &stubCommander[commands.ReorderWidgetsInput]{}
	api := &Handlers{Reorder: reorder}
	payload := commands.ReorderWidgetsInput{AreaCode: "admin.dashboard.main", WidgetIDs: []string{"w1", "w2"}}
	buf, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/widgets/reorder", bytes.NewReader(buf))
	rec := httptest.NewRecorder()
	api.HandleReorderWidgets(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if reorder.calls != 1 {
		t.Fatalf("expected reorder to execute")
	}
}

func TestHandleRefreshWidget(t *testing.T) {
	refresh := &stubCommander[commands.RefreshWidgetInput]{}
	api := &Handlers{Refresh: refresh}
	payload := commands.RefreshWidgetInput{Event: dashboard.WidgetEvent{AreaCode: "admin.dashboard.main"}}
	buf, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/widgets/refresh", bytes.NewReader(buf))
	rec := httptest.NewRecorder()
	api.HandleRefreshWidget(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if refresh.calls != 1 {
		t.Fatalf("expected refresh to execute")
	}
}
