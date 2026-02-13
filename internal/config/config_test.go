package config

import (
	"fmt"
	"testing"
)

func TestNew(t *testing.T) {
	// 1. Happy path: standard environment
	env := Environment{
		Name:        "TEST",
		ServiceName: "TestService",
		ListenAddr:  "localhost:9999",
		DefaultPort: "COM1",
		DefaultMode: true,
	}

	cfg := New(env)

	if cfg.Puerto != env.DefaultPort {
		t.Errorf("Expected Puerto to be %s, got %s", env.DefaultPort, cfg.Puerto)
	}
	if cfg.Marca != "Rhino BAR 8RS" {
		t.Errorf("Expected Marca to be 'Rhino BAR 8RS', got '%s'", cfg.Marca)
	}
	if cfg.ModoPrueba != env.DefaultMode {
		t.Errorf("Expected ModoPrueba to be %v, got %v", env.DefaultMode, cfg.ModoPrueba)
	}
	if cfg.Ambiente != env.Name {
		t.Errorf("Expected Ambiente to be %s, got %s", env.Name, cfg.Ambiente)
	}
	expectedDir := fmt.Sprintf("ws://%s", env.ListenAddr)
	if cfg.Dir != expectedDir {
		t.Errorf("Expected Dir to be %s, got %s", expectedDir, cfg.Dir)
	}

	// 2. Edge case: empty environment
	emptyEnv := Environment{}
	emptyCfg := New(emptyEnv)

	if emptyCfg.Puerto != "" {
		t.Errorf("Expected Puerto to be empty, got %s", emptyCfg.Puerto)
	}
	if emptyCfg.Marca != "Rhino BAR 8RS" {
		t.Errorf("Expected Marca to be 'Rhino BAR 8RS', got '%s'", emptyCfg.Marca)
	}
	if emptyCfg.ModoPrueba != false {
		t.Errorf("Expected ModoPrueba to be false, got %v", emptyCfg.ModoPrueba)
	}
	if emptyCfg.Ambiente != "" {
		t.Errorf("Expected Ambiente to be empty, got %s", emptyCfg.Ambiente)
	}
	if emptyCfg.Dir != "ws://" {
		t.Errorf("Expected Dir to be 'ws://', got %s", emptyCfg.Dir)
	}
}

func TestGetEnvironment(t *testing.T) {
	// Test known environments
	remote := GetEnvironment("remote")
	if remote.Name != "REMOTO" {
		t.Errorf("Expected remote name 'REMOTO', got %s", remote.Name)
	}

	local := GetEnvironment("local")
	if local.Name != "LOCAL" {
		t.Errorf("Expected local name 'LOCAL', got %s", local.Name)
	}

	// Test unknown environment fallback
	unknown := GetEnvironment("unknown_env")
	if unknown.Name != "REMOTO" { // Fallback is remote
		t.Errorf("Expected fallback to REMOTO, got %s", unknown.Name)
	}
}
