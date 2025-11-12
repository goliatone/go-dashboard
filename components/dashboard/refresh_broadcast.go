package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// BroadcastHook fans out widget events to in-process subscribers.
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

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ServeWebSocket upgrades the request and streams widget events as JSON.
func (h *BroadcastHook) ServeWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	defer conn.Close()

	events, cancel := h.Subscribe()
	defer cancel()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		}
	}
}

// ServeSSE provides a Server-Sent Events endpoint for refresh events.
func (h *BroadcastHook) ServeSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	events, cancel := h.Subscribe()
	defer cancel()

	encoder := json.NewEncoder(w)
	flusher, _ := w.(http.Flusher)

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			w.Write([]byte("data: "))
			if err := encoder.Encode(event); err != nil {
				return
			}
			w.Write([]byte("\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}
}
