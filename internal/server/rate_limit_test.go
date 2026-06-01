package server

import (
	"sync"
	"testing"
	"time"
)

func TestNewConfigRateLimiter(t *testing.T) {
	rl := NewConfigRateLimiter(5)
	if rl == nil {
		t.Fatal("Expected NewConfigRateLimiter to return a valid object, got nil")
	}
	if rl.maxPerMin != 5 {
		t.Errorf("Expected maxPerMin to be 5, got %d", rl.maxPerMin)
	}
	if rl.attempts == nil {
		t.Error("Expected attempts map to be initialized, got nil")
	}
}

func TestConfigRateLimiter_Allow(t *testing.T) {
	rl := NewConfigRateLimiter(3)
	client := "192.168.1.100"

	// Should allow up to maxPerMin
	for i := 0; i < 3; i++ {
		if !rl.Allow(client) {
			t.Errorf("Expected request %d to be allowed", i+1)
		}
	}

	// Should block the next one
	if rl.Allow(client) {
		t.Error("Expected 4th request to be blocked")
	}
}

func TestConfigRateLimiter_IndependentClients(t *testing.T) {
	rl := NewConfigRateLimiter(2)
	clientA := "192.168.1.100"
	clientB := "192.168.1.101"

	// Exhaust client A
	rl.Allow(clientA)
	rl.Allow(clientA)
	if rl.Allow(clientA) {
		t.Error("Expected client A to be blocked")
	}

	// Client B should still be allowed
	if !rl.Allow(clientB) {
		t.Error("Expected client B to be allowed despite client A being blocked")
	}
}

func TestConfigRateLimiter_Pruning(t *testing.T) {
	rl := NewConfigRateLimiter(2)
	client := "192.168.1.100"

	// Inject old timestamps
	rl.mu.Lock()
	rl.attempts[client] = []time.Time{
		time.Now().Add(-2 * time.Minute),
		time.Now().Add(-90 * time.Second),
	}
	rl.mu.Unlock()

	// Since the previous requests are old, new requests should be allowed
	if !rl.Allow(client) {
		t.Error("Expected request to be allowed after pruning old entries")
	}
	if !rl.Allow(client) {
		t.Error("Expected second request to be allowed after pruning old entries")
	}

	// Should block the next one
	if rl.Allow(client) {
		t.Error("Expected third request to be blocked")
	}
}

func TestConfigRateLimiter_Concurrency(t *testing.T) {
	rl := NewConfigRateLimiter(100)
	client := "192.168.1.100"
	var wg sync.WaitGroup

	numGoroutines := 150
	results := make(chan bool, numGoroutines)

	// Launch multiple goroutines trying to call Allow concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- rl.Allow(client)
		}()
	}

	wg.Wait()
	close(results)

	allowedCount := 0
	for res := range results {
		if res {
			allowedCount++
		}
	}

	if allowedCount != 100 {
		t.Errorf("Expected exactly 100 allowed requests, got %d", allowedCount)
	}
}
