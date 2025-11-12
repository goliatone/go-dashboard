package dashboard

import (
	"context"
	"sync"
)

// BroadcastHook fans out widget events to in-process subscribers.
// Transports should subscribe (see components/dashboard/gorouter) to stream
// events over WebSockets or any other channel they control.
type BroadcastHook struct {
	mu    sync.RWMutex
	subs  map[int]chan WidgetEvent
	next  int
	close chan struct{}
}

// NewBroadcastHook creates a broadcast hook.
func NewBroadcastHook() *BroadcastHook {
	return &BroadcastHook{
		subs:  make(map[int]chan WidgetEvent),
		close: make(chan struct{}),
	}
}

// WidgetUpdated satisfies the RefreshHook interface and broadcasts events.
func (h *BroadcastHook) WidgetUpdated(ctx context.Context, event WidgetEvent) error {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.subs {
		select {
		case ch <- event:
		default:
		}
	}
	return nil
}

// Subscribe returns a channel of widget events and a cancel func.
func (h *BroadcastHook) Subscribe() (<-chan WidgetEvent, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	id := h.next
	h.next++
	ch := make(chan WidgetEvent, 8)
	h.subs[id] = ch
	cancel := func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if sub, ok := h.subs[id]; ok {
			delete(h.subs, id)
			close(sub)
		}
	}
	return ch, cancel
}
