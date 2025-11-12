package dashboard

import "context"

// Controller orchestrates HTTP handlers/routes for the admin dashboard.
type Controller struct {
	service *Service
}

// NewController wires the service into a controller.
func NewController(service *Service) *Controller {
	return &Controller{service: service}
}

// Render resolves the layout for a viewer and returns it to the caller.
func (c *Controller) Render(ctx context.Context, viewer ViewerContext) (Layout, error) {
	if c.service == nil {
		return Layout{}, nil
	}
	return c.service.ConfigureLayout(ctx, viewer)
}
