package queries

import (
	"context"

	gocommand "github.com/goliatone/go-command"
	dashboard "github.com/goliatone/go-dashboard/components/dashboard"
)

// WidgetAreaInput identifies an area request for a viewer.
type WidgetAreaInput struct {
	Viewer   dashboard.ViewerContext
	AreaCode string
}

type areaService interface {
	ResolveArea(ctx context.Context, viewer dashboard.ViewerContext, areaCode string) (dashboard.ResolvedArea, error)
}

// WidgetAreaQuery fetches widgets for a specific area.
type WidgetAreaQuery struct {
	service areaService
}

// NewWidgetAreaQuery builds the query.
func NewWidgetAreaQuery(service areaService) *WidgetAreaQuery {
	return &WidgetAreaQuery{service: service}
}

var _ gocommand.Querier[WidgetAreaInput, dashboard.ResolvedArea] = (*WidgetAreaQuery)(nil)

// Query resolves an individual area for the viewer.
func (q *WidgetAreaQuery) Query(ctx context.Context, input WidgetAreaInput) (dashboard.ResolvedArea, error) {
	return q.service.ResolveArea(ctx, input.Viewer, input.AreaCode)
}
