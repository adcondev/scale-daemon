package server

import (
	"sync"
	"time"
)

// ConfigRateLimiter restricts how frequently a single client
// can send config change requests via WebSocket.
type ConfigRateLimiter struct {
	mu        sync.Mutex
	attempts  map[string][]time.Time
	maxPerMin int
}

// NewConfigRateLimiter creates a limiter allowing maxPerMinute config changes per client.
func NewConfigRateLimiter(maxPerMinute int) *ConfigRateLimiter {
	return &ConfigRateLimiter{
		attempts:  make(map[string][]time.Time),
		maxPerMin: maxPerMinute,
	}
}

// Allow returns true if the client has not exceeded the rate limit.
// It prunes old entries on every call.
func (rl *ConfigRateLimiter) Allow(clientAddr string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-time.Minute)

	// Keep only entries within the window
	recent := make([]time.Time, 0, rl.maxPerMin)
	for _, t := range rl.attempts[clientAddr] {
		if t.After(cutoff) {
			recent = append(recent, t)
		}
	}

	if len(recent) >= rl.maxPerMin {
		return false
	}

	rl.attempts[clientAddr] = append(recent, now)
	return true
}
