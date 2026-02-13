package scale

import (
	"math"
	"testing"
	"time"
)

func TestErrorConstants(t *testing.T) {
	if ErrEOF != "ERR_EOF" {
		t.Errorf("Expected ErrEOF to be 'ERR_EOF', got '%s'", ErrEOF)
	}
	if ErrTimeout != "ERR_TIMEOUT" {
		t.Errorf("Expected ErrTimeout to be 'ERR_TIMEOUT', got '%s'", ErrTimeout)
	}
	if ErrRead != "ERR_READ" {
		t.Errorf("Expected ErrRead to be 'ERR_READ', got '%s'", ErrRead)
	}

	if ErrorDescriptions[ErrEOF] == "" {
		t.Error("Expected description for ErrEOF")
	}
	if ErrorDescriptions[ErrTimeout] == "" {
		t.Error("Expected description for ErrTimeout")
	}
	if ErrorDescriptions[ErrRead] == "" {
		t.Error("Expected description for ErrRead")
	}
}

func TestSendError(t *testing.T) {
	// Create a buffered channel to receive the error
	ch := make(chan string, 1)

	// Create a Reader with the channel
	r := &Reader{
		broadcast: ch,
	}

	// Send an error
	r.sendError(ErrEOF)

	// Check if the error was received
	select {
	case msg := <-ch:
		if msg != ErrEOF {
			t.Errorf("Expected message '%s', got '%s'", ErrEOF, msg)
		}
	case <-time.After(1 * time.Second):
		t.Error("Timed out waiting for error message")
	}

	// Test non-blocking behavior
	// Fill the channel
	ch <- "full"

	// Try to send another error, should not block
	done := make(chan bool)
	go func() {
		r.sendError(ErrTimeout)
		done <- true
	}()

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Error("sendError blocked when channel was full")
	}
}

func TestGenerateSimulatedWeights(t *testing.T) {
	weights := GenerateSimulatedWeights()

	// Check length
	if len(weights) != 6 {
		t.Errorf("Expected 6 weights, got %d", len(weights))
	}

	// Check values
	for i, w := range weights {
		// Check range
		if w < 0.95 || w > 30.05 {
			t.Errorf("Weight %d out of range (0.95-30.05): %f", i, w)
		}

		// Check decimal places (should be at most 2)
		// We multiply by 100 and check if it's close to an integer
		scaled := w * 100
		if math.Abs(scaled-math.Round(scaled)) > 1e-9 {
			t.Errorf("Weight %d has more than 2 decimal places: %f (scaled: %f)", i, w, scaled)
		}
	}
}
