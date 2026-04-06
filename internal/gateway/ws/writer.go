package ws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsConnWriter serializes all writes to a gorilla websocket connection (WriteJSON + WriteControl).
type wsConnWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (w *wsConnWriter) writeJSON(v interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(v)
}

func (w *wsConnWriter) writeControl(messageType int, data []byte, deadline time.Time) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteControl(messageType, data, deadline)
}
