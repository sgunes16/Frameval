package api

import (
	"context"
	"encoding/json"
	"expvar"
	"sync"
)

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// droppedBroadcasts is an expvar-exposed counter so operators can spot
// the engine shedding WS messages without instrumenting the source.
// Available at /debug/vars on any engine binary with the default mux.
//
// init() pattern (not a top-level NewInt call) so a second package that
// might also register the same name does not panic the process.
var droppedBroadcasts *expvar.Int

func init() {
	if v := expvar.Get("frameval_ws_dropped_total"); v != nil {
		droppedBroadcasts = v.(*expvar.Int)
		return
	}
	droppedBroadcasts = expvar.NewInt("frameval_ws_dropped_total")
}

type Hub struct {
	clients    map[chan []byte]struct{}
	register   chan chan []byte
	unregister chan chan []byte
	broadcast  chan []byte
	mu         sync.RWMutex
}

func NewHub() *Hub {
	return &Hub{
		clients:    map[chan []byte]struct{}{},
		register:   make(chan chan []byte),
		unregister: make(chan chan []byte),
		broadcast:  make(chan []byte, 256),
	}
}

func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()
		case client := <-h.unregister:
			h.mu.Lock()
			delete(h.clients, client)
			close(client)
			h.mu.Unlock()
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client <- message:
				default:
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Broadcast publishes an event to every connected client. Non-blocking:
// if the broadcast channel is full (orchestrator producing events
// faster than connected WS clients can consume them), the message is
// dropped and the package-level dropped counter is incremented rather
// than stalling the caller.
//
// The trade-off is intentional. The orchestrator runs the experiment
// lifecycle; a wedged WS pipeline must not be allowed to halt run
// finalization. Operators see droppage via expvar
// (frameval_ws_dropped_total) or DroppedBroadcasts() which read the
// same underlying counter.
func (h *Hub) Broadcast(eventType string, payload any) {
	message, err := json.Marshal(Event{Type: eventType, Payload: payload})
	if err != nil {
		return
	}
	select {
	case h.broadcast <- message:
	default:
		droppedBroadcasts.Add(1)
	}
}

// DroppedBroadcasts returns the total number of broadcasts the hub
// dropped because its buffer was full. Reads the same expvar counter
// the operator-facing /debug/vars endpoint exposes, so the two surfaces
// can never diverge. Safe to call from any goroutine — expvar.Int is
// atomic underneath.
func (h *Hub) DroppedBroadcasts() uint64 {
	return uint64(droppedBroadcasts.Value())
}
