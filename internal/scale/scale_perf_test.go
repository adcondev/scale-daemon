package scale

import (
	"context"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/adcondev/scale-daemon/internal/config"
	"go.bug.st/serial"
)

// MockPort implements Port interface for testing
type MockPort struct {
	mu     sync.Mutex
	closed bool
}

func (m *MockPort) Read(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.EOF
	}
	// Simulate some data
	return copy(p, []byte("10.50")), nil
}

func (m *MockPort) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed {
		return 0, io.ErrClosedPipe
	}
	return len(p), nil
}

func (m *MockPort) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *MockPort) SetReadTimeout(_ time.Duration) error {
	return nil
}

func TestLockContention(t *testing.T) {
	// Setup config
	cfg := config.New(config.Environment{
		DefaultPort: "COM_TEST",
		DefaultMode: false, // Ensure we use the real connection path
	})

	// Override serialOpen for testing
	origSerialOpen := serialOpen
	defer func() { serialOpen = origSerialOpen }()

	mockPort := &MockPort{}
	serialOpen = func(_ string, _ *serial.Mode) (Port, error) {
		return mockPort, nil
	}

	broadcast := make(chan string, 10)
	r := NewReader(cfg, broadcast)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	// Start reader in background
	go func() {
		defer wg.Done()
		r.Start(ctx)
	}()

	// Wait for the reader to enter the read loop and acquire the lock.
	// We want to catch it during the 500ms sleep.
	// Since we don't know exactly when it starts sleeping, we can try multiple times or just wait a bit.
	// Connect happens fast (mock). Write happens fast (mock).
	// So sleep starts almost immediately.
	time.Sleep(50 * time.Millisecond)

	// Now try to close port. This acquires the lock.
	start := time.Now()

	// r.ClosePort() will block until the lock is released.
	// If the lock is held during sleep, this will take ~450ms (500ms - 50ms).
	r.ClosePort()

	duration := time.Since(start)

	t.Logf("ClosePort duration: %v", duration)

	if duration > 100*time.Millisecond {
		t.Errorf("Lock contention detected: ClosePort took %v, expected < 100ms. The lock is likely held during sleep.", duration)
	}

	// Wait for the goroutine to finish
	cancel()
	wg.Wait()
}
