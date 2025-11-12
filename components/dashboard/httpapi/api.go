package httpapi

import (
	"context"
	"errors"

	gocommand "github.com/goliatone/go-command"
	"github.com/goliatone/go-dashboard/components/dashboard"
	"github.com/goliatone/go-dashboard/components/dashboard/commands"
)

// Executor defines a router-agnostic command surface that transports can call.
// Any HTTP, CLI, or background adapter can translate inbound payloads into the
// strongly typed inputs defined here and delegate to the shared commands.
type Executor interface {
	Assign(ctx context.Context, req dashboard.AddWidgetRequest) error
	Remove(ctx context.Context, input commands.RemoveWidgetInput) error
	Reorder(ctx context.Context, input commands.ReorderWidgetsInput) error
	Refresh(ctx context.Context, input commands.RefreshWidgetInput) error
}

// CommandExecutor wires go-command.Commander instances into the Executor contract.
type CommandExecutor struct {
	AssignCommander  gocommand.Commander[dashboard.AddWidgetRequest]
	RemoveCommander  gocommand.Commander[commands.RemoveWidgetInput]
	ReorderCommander gocommand.Commander[commands.ReorderWidgetsInput]
	RefreshCommander gocommand.Commander[commands.RefreshWidgetInput]
}

var _ Executor = (*CommandExecutor)(nil)

// Assign delegates widget creation to the configured command.
func (e *CommandExecutor) Assign(ctx context.Context, req dashboard.AddWidgetRequest) error {
	if e == nil || e.AssignCommander == nil {
		return errors.New("dashboard: assign command not configured")
	}
	return e.AssignCommander.Execute(ctx, req)
}

// Remove delegates widget removal to the configured command.
func (e *CommandExecutor) Remove(ctx context.Context, input commands.RemoveWidgetInput) error {
	if e == nil || e.RemoveCommander == nil {
		return errors.New("dashboard: remove command not configured")
	}
	return e.RemoveCommander.Execute(ctx, input)
}

// Reorder delegates ordering changes to the configured command.
func (e *CommandExecutor) Reorder(ctx context.Context, input commands.ReorderWidgetsInput) error {
	if e == nil || e.ReorderCommander == nil {
		return errors.New("dashboard: reorder command not configured")
	}
	return e.ReorderCommander.Execute(ctx, input)
}

// Refresh notifies refresh subscribers using the configured command.
func (e *CommandExecutor) Refresh(ctx context.Context, input commands.RefreshWidgetInput) error {
	if e == nil || e.RefreshCommander == nil {
		return errors.New("dashboard: refresh command not configured")
	}
	return e.RefreshCommander.Execute(ctx, input)
}
