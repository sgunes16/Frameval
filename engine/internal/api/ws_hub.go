package api

import (
	"context"
	"encoding/json"
	"expvar"
	"sync"
	"sync/atomic"
)

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

// droppedBroadcasts is an expvar-exposed counter so operators can spot
// the engine shedding WS messages without instrumenting the source.
// Available at /debug/vars on any engine binary with the default mux.
var droppedBroadcasts = expvar.NewInt("frameval_ws_dropped_total")

type Hub struct {
	clients    map[chan []byte]struct{}
	register   chan chan []byte
	unregister chan chan []byte
	broadcast  chan []byte
	mu         sync.RWMutex
	// dropped counts broadcasts the hub had to drop because the buffered
	// broadcast channel was full. Atomic so DroppedBroadcasts() can read
	// it without taking the hub lock.
	dropped atomic.Uint64
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
// if the broadcast channel is full (orchestrator producing faster than
// the hub can fan out), the message is dropped and the dropped counter
// is incremented rather than stalling the caller.
//
// The trade-off is intentional. The orchestrator runs the experiment
// lifecycle; a wedged WS pipeline must not be allowed to halt run
// finalization. Operators see droppage via expvar
// (frameval_ws_dropped_total) and the DroppedBroadcasts accessor.
func (h *Hub) Broadcast(eventType string, payload any) {
	message, err := json.Marshal(Event{Type: eventType, Payload: payload})
	if err != nil {
		return
	}
	select {
	case h.broadcast <- message:
	default:
		h.dropped.Add(1)
		droppedBroadcasts.Add(1)
	}
}

// DroppedBroadcasts returns the total number of broadcasts the hub
// dropped because its buffer was full. Safe to call from any goroutine.
func (h *Hub) DroppedBroadcasts() uint64 {
	return h.dropped.Load()
}
