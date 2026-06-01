package config

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestGetEnvironment(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		wantName string
		wantLog  bool
	}{
		{
			name:     "Valid remote environment",
			env:      "remote",
			wantName: EnvRemoteName,
			wantLog:  false,
		},
		{
			name:     "Valid local environment",
			env:      "local",
			wantName: EnvLocalName,
			wantLog:  false,
		},
		{
			name:     "Unknown environment falls back to local",
			env:      "unknown",
			wantName: EnvLocalName,
			wantLog:  true,
		},
		{
			name:     "Empty environment falls back to local",
			env:      "",
			wantName: EnvLocalName,
			wantLog:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			originalOutput := log.Writer()
			log.SetOutput(&buf)
			t.Cleanup(func() {
				log.SetOutput(originalOutput)
			})

			got := GetEnvironment(tt.env)

			if got.Name != tt.wantName {
				t.Errorf("GetEnvironment(%q) got environment name = %v, want %v", tt.env, got.Name, tt.wantName)
			}

			if tt.wantLog {
				if buf.Len() == 0 {
					t.Errorf("GetEnvironment(%q) expected log output, but got none", tt.env)
				}
			} else {
				if buf.Len() > 0 {
					t.Errorf("GetEnvironment(%q) unexpected log output: %v", tt.env, buf.String())
				}
			}
		})
	}
}

func TestEnvironmentsConsistency(t *testing.T) {
	// The Environments map is initialized with global variables that can be overridden via ldflags.
	// This test ensures that the map entries accurately reflect the current state of these globals.

	// ServerPort is empty by default
	expectedListenSuffix := ServerPort

	// We test what is currently set in the global state
	remoteEnv := Environments["remote"]
	if remoteEnv.Name != EnvRemoteName {
		t.Errorf("Expected 'remote' environment Name to be 'REMOTO', got %s", remoteEnv.Name)
	}
	if !strings.HasSuffix(remoteEnv.ListenAddr, expectedListenSuffix) {
		t.Errorf("Expected 'remote' ListenAddr to end with %q, got %s", expectedListenSuffix, remoteEnv.ListenAddr)
	}
	if remoteEnv.DefaultPort != DefaultComPort {
		t.Errorf("Expected 'remote' DefaultPort to be 'COM3', got %s", remoteEnv.DefaultPort)
	}

	localEnv := Environments["local"]
	if localEnv.Name != EnvLocalName {
		t.Errorf("Expected 'local' environment Name to be 'LOCAL', got %s", localEnv.Name)
	}
	if !strings.HasSuffix(localEnv.ListenAddr, expectedListenSuffix) {
		t.Errorf("Expected 'local' ListenAddr to end with %q, got %s", expectedListenSuffix, localEnv.ListenAddr)
	}
	if localEnv.DefaultPort != DefaultComPort {
		t.Errorf("Expected 'local' DefaultPort to be 'COM3', got %s", localEnv.DefaultPort)
	}
}
