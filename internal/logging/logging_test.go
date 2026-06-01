package logging

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestFilteredLogger_Write(t *testing.T) {
	tests := []struct {
		name         string
		verbose      bool
		message      string
		expectOutput bool
	}{
		{
			name:         "discard non-critical when not verbose",
			verbose:      false,
			message:      "[~] Modo prueba activado: test mode",
			expectOutput: false,
		},
		{
			name:         "keep critical when not verbose",
			verbose:      false,
			message:      "CRITICAL: System crash",
			expectOutput: true,
		},
		{
			name:         "keep non-critical when verbose",
			verbose:      true,
			message:      "[~] Modo prueba activado: test mode",
			expectOutput: true,
		},
		{
			name:         "discard another non-critical when not verbose",
			verbose:      false,
			message:      "2023/10/26 12:00:00 [>] Peso enviado a cliente",
			expectOutput: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			vMu := &sync.RWMutex{}
			verbose := tc.verbose

			logger := NewFilteredLogger(&buf, &verbose, vMu)

			n, err := logger.Write([]byte(tc.message))

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if n != len(tc.message) {
				t.Errorf("expected %d bytes written, got %d", len(tc.message), n)
			}

			output := buf.String()
			hasOutput := len(output) > 0

			if hasOutput != tc.expectOutput {
				t.Errorf("expected output: %v, got output: %v (output: %q)", tc.expectOutput, hasOutput, output)
			}

			if tc.expectOutput && !strings.Contains(output, tc.message) {
				t.Errorf("expected output to contain %q, but got %q", tc.message, output)
			}
		})
	}
}
