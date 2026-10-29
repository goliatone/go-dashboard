package gorouter

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/goliatone/go-dashboard/components/dashboard"
	"github.com/goliatone/go-router"
	"github.com/gorilla/websocket"
)

type lifecycleSocket struct {
	router.WebSocketContext
	ctx                           context.Context
	started, interrupted, release chan struct{}
	once                          sync.Once
	writeErr                      error
}

func (s *lifecycleSocket) Context() context.Context { return s.ctx }
func (s *lifecycleSocket) ReadMessage() (int, []byte, error) {
	close(s.started)
	<-s.interrupted
	<-s.release
	return 0, nil, router.ErrWebSocketReadInterrupted
}
func (s *lifecycleSocket) InterruptRead() error {
	s.once.Do(func() { close(s.interrupted) })
	return nil
}
func (s *lifecycleSocket) WriteJSON(any) error { return s.writeErr }

func TestWebSocketTeardownJoinsReader(t *testing.T) {
	for _, reason := range []string{"context", "subscription", "write"} {
		t.Run(reason, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			writeErr := errors.New("write failed")
			socket := &lifecycleSocket{ctx: ctx, started: make(chan struct{}), interrupted: make(chan struct{}), release: make(chan struct{}), writeErr: writeErr}
			events := make(chan dashboard.WidgetEvent, 1)
			result := make(chan error, 1)
			go func() { result <- streamWidgetEvents(socket, events) }()
			waitLifecycle(t, socket.started)
			switch reason {
			case "context":
				cancel()
			case "subscription":
				close(events)
			case "write":
				events <- dashboard.WidgetEvent{}
			}
			waitLifecycle(t, socket.interrupted)
			select {
			case <-result:
				t.Fatal("handler returned while reader still owned the connection")
			default:
			}
			close(socket.release)
			select {
			case err := <-result:
				if reason == "write" && !errors.Is(err, writeErr) {
					t.Fatalf("error = %v", err)
				}
				if reason != "write" && err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("handler did not join reader")
			}
		})
	}
}

func TestWebSocketRejectsUnsupportedInterruption(t *testing.T) {
	if err := streamWidgetEvents(struct{ router.WebSocketContext }{}, nil); err == nil {
		t.Fatal("expected unsupported adapter error before starting a reader")
	}
}

func TestWebSocketFiberKeepaliveDeliveryAndDisconnect(t *testing.T) {
	server := router.NewFiberAdapter()
	hook := dashboard.NewBroadcastHook()
	done := make(chan struct{})
	cfg := router.DefaultWebSocketConfig()
	cfg.PingPeriod = 25 * time.Millisecond
	cfg.PongWait = 250 * time.Millisecond
	server.Router().WebSocket("/ws", cfg, func(ws router.WebSocketContext) error {
		defer close(done)
		events, cancel := hook.Subscribe()
		defer cancel()
		return streamWidgetEvents(ws, events)
	})
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	served := make(chan error, 1)
	go func() { served <- server.WrappedRouter().Listener(listener) }()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = server.Shutdown(ctx)
		select {
		case <-served:
		case <-ctx.Done():
			t.Error("server did not stop")
		}
	})
	conn, _, err := websocket.DefaultDialer.Dial("ws://"+listener.Addr().String()+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	pong := make(chan struct{}, 1)
	conn.SetPongHandler(func(string) error {
		select {
		case pong <- struct{}{}:
		default:
		}
		return nil
	})
	messages := make(chan []byte, 1)
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			messages <- data
		}
	}()
	defer func() { _ = conn.Close(); waitLifecycle(t, clientDone) }()
	// A responsive connection must survive multiple adapter read-deadline windows.
	<-time.After(2 * cfg.PongWait)
	if err := conn.WriteControl(websocket.PingMessage, []byte("probe"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	waitLifecycle(t, pong)
	if err := hook.WidgetUpdated(context.Background(), dashboard.WidgetEvent{Reason: "refresh"}); err != nil {
		t.Fatal(err)
	}
	select {
	case data := <-messages:
		var event dashboard.WidgetEvent
		if err := json.Unmarshal(data, &event); err != nil || event.Reason != "refresh" {
			t.Fatalf("event %s: %v", data, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("refresh not delivered")
	}
	_ = conn.Close()
	waitLifecycle(t, done)
}

func waitLifecycle(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("WebSocket lifecycle did not finish")
	}
}
