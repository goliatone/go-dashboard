package queries

import (
	"context"

	gocommand "github.com/goliatone/go-command"
	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

type layoutService interface {
	ConfigureLayout(ctx context.Context, viewer dashboard.ViewerContext) (dashboard.Layout, error)
}

// LayoutQuery executes read-only layout resolution.
type LayoutQuery struct {
	service layoutService
}

// NewLayoutQuery builds the query.
func NewLayoutQuery(service layoutService) *LayoutQuery {
	return &LayoutQuery{service: service}
}

var _ gocommand.Querier[dashboard.ViewerContext, dashboard.Layout] = (*LayoutQuery)(nil)

// Query resolves the layout for the viewer.
func (q *LayoutQuery) Query(ctx context.Context, viewer dashboard.ViewerContext) (dashboard.Layout, error) {
	return q.service.ConfigureLayout(ctx, viewer)
}
