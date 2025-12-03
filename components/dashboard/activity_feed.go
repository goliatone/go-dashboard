package dashboard

import (
	"context"
	"time"
)

// ActivityItem represents a recent activity entry displayed by the widget.
type ActivityItem struct {
	User    string
	Action  string
	Details string
	Ago     time.Duration
}

// ActivityFeed fetches recent activity entries for the current viewer.
type ActivityFeed interface {
	Recent(ctx context.Context, viewer ViewerContext, limit int) ([]ActivityItem, error)
}

// StaticActivityFeed returns fixed entries useful for demos/tests.
type StaticActivityFeed struct {
	Items []ActivityItem
}

// Recent returns up to limit items from the static list.
func (f StaticActivityFeed) Recent(_ context.Context, _ ViewerContext, limit int) ([]ActivityItem, error) {
	if limit <= 0 || limit >= len(f.Items) {
		return append([]ActivityItem{}, f.Items...), nil
	}
	return append([]ActivityItem{}, f.Items[:limit]...), nil
}

// DefaultActivityFeed provides placeholder entries for the demo widget.
func DefaultActivityFeed() ActivityFeed {
	now := time.Now()
	return StaticActivityFeed{
		Items: []ActivityItem{
			{User: "Candice Reed", Action: "published the spring pricing update", Details: "Billing · Plan v3 rollout", Ago: now.Sub(now.Add(-5 * time.Minute))},
			{User: "Noah Patel", Action: "invited 24 enterprise seats", Details: "Acme Industrial — Enterprise", Ago: now.Sub(now.Add(-22 * time.Minute))},
			{User: "Marcos Valle", Action: "resolved 14 aging invoices", Details: "Finance · Treasury automation", Ago: now.Sub(now.Add(-49 * time.Minute))},
			{User: "Sara Ndlovu", Action: "shipped a dashboard theme change", Details: "Design System · Canary env", Ago: now.Sub(now.Add(-2 * time.Hour))},
			{User: "Elena Ibarra", Action: "closed incident #782", Details: "Checkout API · On-call", Ago: now.Sub(now.Add(-6 * time.Hour))},
		},
	}
}
