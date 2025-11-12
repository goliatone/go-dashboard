package dashboard

import "context"

// NotificationsClient defines the minimal interface needed from go-notifications (or similar).
type NotificationsClient interface {
	PublishDashboardEvent(ctx context.Context, event WidgetEvent) error
}

// NotificationsHook forwards widget events to an external notifications client.
type NotificationsHook struct {
	Client  NotificationsClient
	Channel string
}

// WidgetUpdated publishes events to the configured notifications client.
func (h *NotificationsHook) WidgetUpdated(ctx context.Context, event WidgetEvent) error {
	if h == nil || h.Client == nil {
		return nil
	}
	return h.Client.PublishDashboardEvent(ctx, event)
}
