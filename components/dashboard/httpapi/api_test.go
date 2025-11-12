package httpapi

import (
	"context"
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

func TestCommandExecutorAssign(t *testing.T) {
	assign := &stubCommander[dashboard.AddWidgetRequest]{}
	exec := &CommandExecutor{AssignCommander: assign}
	req := dashboard.AddWidgetRequest{DefinitionID: "def", AreaCode: "area"}
	if err := exec.Assign(context.Background(), req); err != nil {
		t.Fatalf("Assign returned error: %v", err)
	}
	if assign.calls != 1 {
		t.Fatalf("expected assign command execution")
	}
}

func TestCommandExecutorRemove(t *testing.T) {
	remove := &stubCommander[commands.RemoveWidgetInput]{}
	exec := &CommandExecutor{RemoveCommander: remove}
	input := commands.RemoveWidgetInput{WidgetID: "widget-1"}
	if err := exec.Remove(context.Background(), input); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if remove.last.WidgetID != "widget-1" {
		t.Fatalf("expected widget id propagation")
	}
}

func TestCommandExecutorReorder(t *testing.T) {
	reorder := &stubCommander[commands.ReorderWidgetsInput]{}
	exec := &CommandExecutor{ReorderCommander: reorder}
	input := commands.ReorderWidgetsInput{AreaCode: "area", WidgetIDs: []string{"w1", "w2"}}
	if err := exec.Reorder(context.Background(), input); err != nil {
		t.Fatalf("Reorder returned error: %v", err)
	}
	if reorder.calls != 1 {
		t.Fatalf("expected reorder execution")
	}
}

func TestCommandExecutorRefresh(t *testing.T) {
	refresh := &stubCommander[commands.RefreshWidgetInput]{}
	exec := &CommandExecutor{RefreshCommander: refresh}
	input := commands.RefreshWidgetInput{Event: dashboard.WidgetEvent{AreaCode: "area"}}
	if err := exec.Refresh(context.Background(), input); err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if refresh.calls != 1 {
		t.Fatalf("expected refresh execution")
	}
}

func TestCommandExecutorMissingCommand(t *testing.T) {
	exec := &CommandExecutor{}
	if err := exec.Assign(context.Background(), dashboard.AddWidgetRequest{}); err == nil {
		t.Fatalf("expected error when assign command missing")
	}
}

func TestCommandExecutorPreferences(t *testing.T) {
	prefs := &stubCommander[commands.SaveLayoutPreferencesInput]{}
	exec := &CommandExecutor{PrefsCommander: prefs}
	input := commands.SaveLayoutPreferencesInput{Viewer: dashboard.ViewerContext{UserID: "user"}}
	if err := exec.Preferences(context.Background(), input); err != nil {
		t.Fatalf("Preferences returned error: %v", err)
	}
	if prefs.calls != 1 {
		t.Fatalf("expected preferences execution")
	}
}
