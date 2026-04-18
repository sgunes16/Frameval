package api

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

func (s *Service) HandleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	client := make(chan []byte, 64)
	s.hub.register <- client
	defer func() { s.hub.unregister <- client; _ = conn.Close() }()
	for {
		select {
		case message := <-client:
			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		default:
			if err := conn.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(time.Second)); err != nil {
				return
			}
			time.Sleep(2 * time.Second)
		}
	}
}
