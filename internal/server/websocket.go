package server

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketHub manages WebSocket client connections and broadcasts.
type WebSocketHub struct {
	mu      sync.RWMutex
	clients map[*WSClient]struct{}
}

// NewWebSocketHub creates a new hub.
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients: make(map[*WSClient]struct{}),
	}
}

// Register adds a client to the hub.
func (h *WebSocketHub) Register(c *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[c] = struct{}{}
	slog.Debug("ws client registered", "total", len(h.clients))
}

// Unregister removes a client from the hub.
func (h *WebSocketHub) Unregister(c *WSClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, c)
	close(c.send)
	slog.Debug("ws client unregistered", "total", len(h.clients))
}

// Broadcast sends a message to all connected clients.
func (h *WebSocketHub) Broadcast(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		slog.Error("broadcast marshal", "error", err)
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// Client buffer full, drop it.
			go h.Unregister(c)
		}
	}
}

// Run starts the hub's cleanup loop (currently a no-op placeholder).
func (h *WebSocketHub) Run() {
	// Could add periodic ping/pong cleanup here.
}

// WSClient represents a single WebSocket connection.
type WSClient struct {
	conn *websocket.Conn
	send chan []byte
}

// WritePump pumps messages from the hub to the WebSocket connection.
func (c *WSClient) WritePump() {
	defer c.conn.Close()

	for msg := range c.send {
		c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}
