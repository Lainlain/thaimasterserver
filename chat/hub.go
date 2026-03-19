package chat

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// OutgoingMessage is what gets broadcast to all clients
type OutgoingMessage struct {
	Type        string `json:"type"` // "message" | "system" | "banned" | "online_users"
	ID          int    `json:"id,omitempty"`
	UserID      int    `json:"user_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	AvatarURL   string `json:"avatar_url,omitempty"`
	Message     string `json:"message,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
	OnlineCount int    `json:"online_count,omitempty"`
	UserIDs     []int  `json:"user_ids,omitempty"`
}

// Client represents a connected WebSocket user
type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID int    // 0 = guest (read-only)
	name   string // display name for logging
}

// Hub manages all active WebSocket connections
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]bool
}

var globalHub = &Hub{
	clients: make(map[*Client]bool),
}

// GetHub returns the singleton hub
func GetHub() *Hub {
	return globalHub
}

// Register adds a client
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()
	h.broadcastOnlineUsers()
	log.Printf("💬 Chat client connected (userID=%d, total=%d)", c.userID, h.count())
}

// Unregister removes a client and closes its send channel
func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	if _, ok := h.clients[c]; ok {
		delete(h.clients, c)
		close(c.send)
	}
	h.mu.Unlock()
	h.broadcastOnlineUsers()
	log.Printf("💬 Chat client disconnected (userID=%d, total=%d)", c.userID, h.count())
}

// Broadcast sends a message to every connected client
func (h *Hub) Broadcast(msg OutgoingMessage) {
	msg.OnlineCount = h.count()
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
			// slow client — skip
		}
	}
}

// broadcastOnlineUsers sends the current online count + list of logged-in user IDs to all clients
func (h *Hub) broadcastOnlineUsers() {
	// Collect IDs of all authenticated connected users (userID > 0)
	var userIDs []int
	h.mu.RLock()
	for c := range h.clients {
		if c.userID > 0 {
			userIDs = append(userIDs, c.userID)
		}
	}
	count := len(h.clients)
	h.mu.RUnlock()

	if userIDs == nil {
		userIDs = []int{}
	}
	data, _ := json.Marshal(OutgoingMessage{
		Type:        "online_users",
		OnlineCount: count,
		UserIDs:     userIDs,
	})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- data:
		default:
		}
	}
}

func (h *Hub) count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// writePump pumps messages from the send channel to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
