package dashboard

import "context"

// ActivityContext captures actor/user/tenant identifiers for activity events.
type ActivityContext struct {
	ActorID  string
	UserID   string
	TenantID string
}

type activityContextKey struct{}

// ContextWithActivity stores activity context on the provided context.
func ContextWithActivity(ctx context.Context, meta ActivityContext) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, activityContextKey{}, meta)
}

// activityContextFrom extracts the activity context from the context, if present.
func activityContextFrom(ctx context.Context) ActivityContext {
	if ctx == nil {
		return ActivityContext{}
	}
	if meta, ok := ctx.Value(activityContextKey{}).(ActivityContext); ok {
		return meta
	}
	return ActivityContext{}
}
