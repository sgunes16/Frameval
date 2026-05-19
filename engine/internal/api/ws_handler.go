package api

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocket lifecycle constants. Tuned for a local-first tool where
// the engine and browser are on the same machine; pingPeriod just
// needs to be < pongWait so a missed pong is caught within one cycle.
const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (s *Service) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := make(chan []byte, 64)
	s.hub.register <- client

	// readPump consumes close/pong frames so gorilla can detect a
	// dead connection. Without this loop, a browser tab close (which
	// sends a TCP RST or close frame) goes unnoticed by the engine,
	// and the next write surfaces as EPIPE/ECONNRESET in the proxy.
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn.SetReadLimit(512)
		_ = conn.SetReadDeadline(time.Now().Add(pongWait))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(pongWait))
		})
		for {
			if _, _, err := conn.NextReader(); err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		s.hub.unregister <- client
		_ = conn.Close()
	}()

	for {
		select {
		case message, ok := <-client:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
