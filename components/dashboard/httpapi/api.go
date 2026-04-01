package httpapi

import (
	"context"
	"errors"

	gocommand "github.com/goliatone/go-command"
	"github.com/goliatone/go-dashboard/components/dashboard"
)

// Executor is the shared transport executor contract.
type Executor = dashboard.Executor

// ServiceExecutor is the shared service-backed executor.
type ServiceExecutor = dashboard.ServiceExecutor

// CommandExecutor wires go-command.Commander instances into the Executor contract.
type CommandExecutor struct {
	AssignCommander  gocommand.Commander[dashboard.AddWidgetRequest]
	RemoveCommander  gocommand.Commander[dashboard.RemoveWidgetInput]
	ReorderCommander gocommand.Commander[dashboard.ReorderWidgetsInput]
	RefreshCommander gocommand.Commander[dashboard.RefreshWidgetInput]
	PrefsCommander   gocommand.Commander[dashboard.SaveLayoutPreferencesInput]
}

var _ dashboard.Executor = (*CommandExecutor)(nil)

// NewServiceExecutor creates an executor backed directly by the shared dashboard service.
func NewServiceExecutor(service dashboard.ServiceExecutorService) *dashboard.ServiceExecutor {
	return dashboard.NewServiceExecutor(service)
}

// Assign delegates widget creation to the configured command.
func (e *CommandExecutor) Assign(ctx context.Context, req dashboard.AddWidgetRequest) error {
	if e == nil || e.AssignCommander == nil {
		return errors.New("dashboard: assign command not configured")
	}
	return e.AssignCommander.Execute(ctx, req)
}

// Remove delegates widget removal to the configured command.
func (e *CommandExecutor) Remove(ctx context.Context, input dashboard.RemoveWidgetInput) error {
	if e == nil || e.RemoveCommander == nil {
		return errors.New("dashboard: remove command not configured")
	}
	return e.RemoveCommander.Execute(ctx, input)
}

// Reorder delegates ordering changes to the configured command.
func (e *CommandExecutor) Reorder(ctx context.Context, input dashboard.ReorderWidgetsInput) error {
	if e == nil || e.ReorderCommander == nil {
		return errors.New("dashboard: reorder command not configured")
	}
	return e.ReorderCommander.Execute(ctx, input)
}

// Refresh notifies refresh subscribers using the configured command.
func (e *CommandExecutor) Refresh(ctx context.Context, input dashboard.RefreshWidgetInput) error {
	if e == nil || e.RefreshCommander == nil {
		return errors.New("dashboard: refresh command not configured")
	}
	return e.RefreshCommander.Execute(ctx, input)
}

// Preferences saves layout overrides for the viewer.
func (e *CommandExecutor) Preferences(ctx context.Context, input dashboard.SaveLayoutPreferencesInput) error {
	if e == nil || e.PrefsCommander == nil {
		return errors.New("dashboard: preferences command not configured")
	}
	return e.PrefsCommander.Execute(ctx, input)
}
