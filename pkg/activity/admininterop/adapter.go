package admininterop

import (
	"context"
	"strings"
	"time"

	dashboardactivity "github.com/goliatone/go-dashboard/pkg/activity"
)

// Record is a generic admin-style activity payload.
type Record struct {
	Actor      string
	Action     string
	Object     string
	Channel    string
	Metadata   map[string]any
	OccurredAt time.Time
}

// Sink receives admin-style activity records.
type Sink interface {
	Record(context.Context, Record) error
}

// SinkFunc lets plain functions satisfy Sink.
type SinkFunc func(context.Context, Record) error

// Record dispatches to the underlying function.
func (fn SinkFunc) Record(ctx context.Context, record Record) error {
	if fn == nil {
		return nil
	}
	return fn(ctx, record)
}

// Adapter forwards admin-style records to dashboard activity hooks.
type Adapter struct {
	emitter        *dashboardactivity.Emitter
	defaultChannel string
}

// Option customizes adapter behavior.
type Option func(*Adapter)

// WithDefaultChannel overrides the adapter-level channel fallback.
//
// Precedence:
// 1. record.Channel when provided
// 2. this adapter default channel
// 3. emitter channel default (from activity.Config, default "dashboard")
func WithDefaultChannel(channel string) Option {
	return func(adapter *Adapter) {
		if adapter == nil {
			return
		}
		adapter.defaultChannel = strings.TrimSpace(channel)
	}
}

// NewAdapter builds a write adapter for admin-style activity records.
//
// The adapter does not import go-admin types to avoid dependency cycles.
func NewAdapter(hooks dashboardactivity.Hooks, cfg dashboardactivity.Config, opts ...Option) *Adapter {
	adapter := &Adapter{
		emitter:        dashboardactivity.NewEmitter(hooks, cfg),
		defaultChannel: "admin",
	}
	for _, opt := range opts {
		if opt != nil {
			opt(adapter)
		}
	}
	return adapter
}

// NewSink returns a sink-like object suitable for external systems.
func NewSink(hooks dashboardactivity.Hooks, cfg dashboardactivity.Config, opts ...Option) Sink {
	return NewAdapter(hooks, cfg, opts...)
}

// NewSinkFunc returns a function adapter suitable for callback-style wiring.
func NewSinkFunc(hooks dashboardactivity.Hooks, cfg dashboardactivity.Config, opts ...Option) SinkFunc {
	adapter := NewAdapter(hooks, cfg, opts...)
	return adapter.Record
}

// Enabled reports whether record forwarding is active.
func (a *Adapter) Enabled() bool {
	return a != nil && a.emitter != nil && a.emitter.Enabled()
}

// Record maps Record fields into dashboardactivity.Event and emits to hooks.
func (a *Adapter) Record(ctx context.Context, record Record) error {
	if !a.Enabled() {
		return nil
	}
	return a.emitter.Emit(ctx, EventFromRecord(record, a.defaultChannel))
}

// EventFromRecord maps a generic admin-style record into a dashboard activity event.
func EventFromRecord(record Record, defaultChannel string) dashboardactivity.Event {
	objectType, objectID, _ := dashboardactivity.ParseCompositeObject(record.Object)

	channel := strings.TrimSpace(record.Channel)
	if channel == "" {
		channel = strings.TrimSpace(defaultChannel)
	}

	return dashboardactivity.Event{
		Verb:       strings.TrimSpace(record.Action),
		ActorID:    strings.TrimSpace(record.Actor),
		ObjectType: objectType,
		ObjectID:   objectID,
		Channel:    channel,
		Metadata:   cloneMap(record.Metadata),
		OccurredAt: record.OccurredAt,
	}
}

func cloneMap(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]any, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}

var _ Sink = (*Adapter)(nil)
var _ Sink = SinkFunc(nil)
