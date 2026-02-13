package server

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestBroadcasterLogic(t *testing.T) {
	// Setup a mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		// Just keep reading
		go func() {
			ctx := context.Background()
			for {
				_, _, err := c.Read(ctx)
				if err != nil {
					return
				}
			}
		}()
	}))
	defer server.Close()

	broadcaster := NewBroadcaster(make(chan string))

	// Helper to dial
	dial := func() *websocket.Conn {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		c, _, err := websocket.Dial(ctx, server.URL, nil)
		if err != nil {
			t.Fatalf("Failed to dial: %v", err)
		}
		return c
	}

	c1 := dial()
	defer c1.Close(websocket.StatusNormalClosure, "")

	// 1. AddClient
	broadcaster.AddClient(c1)

	// Verify internal state
	broadcaster.mu.RLock()
	if len(broadcaster.clients) != 1 {
		t.Errorf("Expected 1 client in map, got %d", len(broadcaster.clients))
	}
	if len(broadcaster.clientList) != 1 {
		t.Errorf("Expected 1 client in list, got %d", len(broadcaster.clientList))
	}
	if len(broadcaster.clientList) > 0 && broadcaster.clientList[0] != c1 {
		t.Errorf("Client in list does not match added client")
	}
	broadcaster.mu.RUnlock()

	// 2. Add another client
	c2 := dial()
	defer c2.Close(websocket.StatusNormalClosure, "")
	broadcaster.AddClient(c2)

	broadcaster.mu.RLock()
	if len(broadcaster.clients) != 2 {
		t.Errorf("Expected 2 clients in map, got %d", len(broadcaster.clients))
	}
	if len(broadcaster.clientList) != 2 {
		t.Errorf("Expected 2 clients in list, got %d", len(broadcaster.clientList))
	}
	broadcaster.mu.RUnlock()

	// 3. RemoveClient
	broadcaster.RemoveClient(c1)

	broadcaster.mu.RLock()
	if len(broadcaster.clients) != 1 {
		t.Errorf("Expected 1 client in map after removal, got %d", len(broadcaster.clients))
	}
	if len(broadcaster.clientList) != 1 {
		t.Errorf("Expected 1 client in list after removal, got %d", len(broadcaster.clientList))
	}
	if len(broadcaster.clientList) > 0 && broadcaster.clientList[0] != c2 {
		t.Errorf("Remaining client in list should be c2")
	}
	broadcaster.mu.RUnlock()
}

func BenchmarkBroadcastWeight(b *testing.B) {
	log.SetOutput(io.Discard)
	// Setup a mock server to accept websocket connections
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		// Just keep reading to avoid blocking writing
		go func() {
			ctx := context.Background()
			for {
				_, _, err := c.Read(ctx)
				if err != nil {
					return
				}
			}
		}()
	}))
	defer server.Close()

	// Create broadcaster
	broadcaster := NewBroadcaster(make(chan string))

	// Connect N clients
	numClients := 1000
	clients := make([]*websocket.Conn, numClients)
	var wg sync.WaitGroup

    // Create connections in parallel to speed up setup
    // Limit concurrency to avoid file descriptor limits
    sem := make(chan struct{}, 50)

	for i := 0; i < numClients; i++ {
        wg.Add(1)
        sem <- struct{}{}
		go func(idx int) {
            defer wg.Done()
            defer func() { <-sem }()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			c, _, err := websocket.Dial(ctx, server.URL, nil)
			if err != nil {
				b.Logf("Failed to dial: %v", err)
				return
			}
			clients[idx] = c
			broadcaster.AddClient(c)
		}(i)
	}
    wg.Wait()

	// Ensure all clients are added
    // Check for nil clients if any failed
    validClients := 0
    for _, c := range clients {
        if c != nil {
            validClients++
        }
    }
    if validClients < numClients {
        b.Logf("Warning: only connected %d/%d clients", validClients, numClients)
    }

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broadcaster.broadcastWeight("10.00")
	}
    b.StopTimer()

    if broadcaster.ClientCount() < numClients {
        b.Logf("Warning: client count dropped to %d", broadcaster.ClientCount())
    }

    // Cleanup
    for _, c := range clients {
        if c != nil {
            c.Close(websocket.StatusNormalClosure, "")
        }
    }
}
