package server

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

// Broadcaster fans out weight readings to all connected clients
type Broadcaster struct {
	clients      map[*websocket.Conn]bool
	mu           sync.RWMutex
	broadcast    <-chan string
	onWeightSent func() // Called after successful weight broadcast
}

// NewBroadcaster creates a broadcaster for the given channel
func NewBroadcaster(broadcast <-chan string, onWeightSent func()) *Broadcaster {
	return &Broadcaster{
		clients:      make(map[*websocket.Conn]bool),
		broadcast:    broadcast,
		onWeightSent: onWeightSent,
	}
}

// Start begins broadcasting weights to clients (blocking)
func (b *Broadcaster) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case peso, ok := <-b.broadcast:
			if !ok {
				return
			}
			b.broadcastWeight(peso)
		}
	}
}

// broadcastWeight sends raw weight string to all clients
// CONSTRAINT: Weight is sent as JSON string, NOT wrapped in object
func (b *Broadcaster) broadcastWeight(peso string) {
	b.mu.RLock()
	clients := make([]*websocket.Conn, 0, len(b.clients))
	for c := range b.clients {
		clients = append(clients, c)
	}
	b.mu.RUnlock()

	if len(clients) == 0 {
		return
	}

	for _, conn := range clients {
		go func(c *websocket.Conn) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()

			// CRITICAL: wsjson.Write with string sends "12.50" as JSON string
			// This preserves the exact format expected by clients
			if err := wsjson.Write(ctx, c, peso); err != nil {
				log.Printf("[!] Error al enviar a cliente: %v", err)
				b.removeAndCloseClient(c)
			}
		}(conn)
	}
	// Record activity after broadcasting to at least one client
	if b.onWeightSent != nil {
		b.onWeightSent()
	}
}

// removeAndCloseClient safely removes and closes a client connection
func (b *Broadcaster) removeAndCloseClient(conn *websocket.Conn) {
	b.mu.Lock()
	_, exists := b.clients[conn]
	if exists {
		delete(b.clients, conn)
	}
	b.mu.Unlock()

	if exists {
		err := conn.Close(websocket.StatusInternalError, "Error de envío")
		if err != nil {
			return
		}
	}
}

// AddClient registers a new WebSocket connection
func (b *Broadcaster) AddClient(conn *websocket.Conn) {
	b.mu.Lock()
	b.clients[conn] = true
	b.mu.Unlock()
}

// RemoveClient unregisters a WebSocket connection
func (b *Broadcaster) RemoveClient(conn *websocket.Conn) {
	b.mu.Lock()
	delete(b.clients, conn)
	b.mu.Unlock()
}

// ClientCount returns the number of connected clients
func (b *Broadcaster) ClientCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.clients)
}
