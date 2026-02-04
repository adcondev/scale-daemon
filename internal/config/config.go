package config

import (
	"fmt"
	"sync"
)

// Environment holds environment-specific settings
type Environment struct {
	Name        string
	ServiceName string
	ListenAddr  string
	DefaultPort string
	DefaultMode bool // true = test mode (simulated weights)
}

// TODO: Make Port inyectable via ldflags. Same port and addres could cause conflicts
// TODO: Frontend should handle the new naming of envs, local and remote

// Environments defines available deployment configurations
var Environments = map[string]Environment{
	"prod": {
		Name:        "PRODUCCIÓN",
		ServiceName: "BasculaServicio",
		ListenAddr:  "0.0.0.0:8765",
		DefaultPort: "COM3",
		DefaultMode: true,
	},
	"test": {
		Name:        "TEST/DEV",
		ServiceName: "BasculaServicioTest",
		ListenAddr:  "localhost:8765",
		DefaultPort: "COM3",
		DefaultMode: true,
	},
}

// GetEnvironment returns config for the specified environment
// Falls back to "prod" if unknown
func GetEnvironment(env string) Environment {
	if cfg, ok := Environments[env]; ok {
		return cfg
	}
	return Environments["prod"]
}

// Config holds the runtime configuration for the scale service
type Config struct {
	mu         sync.RWMutex
	Puerto     string
	Marca      string
	ModoPrueba bool
	Ambiente   string
	Dir        string
}

// New creates a Config initialized from the environment
func New(env Environment) *Config {
	return &Config{
		Puerto:     env.DefaultPort,
		Marca:      "Rhino BAR 8RS",
		ModoPrueba: env.DefaultMode,
		Ambiente:   env.Name,
		Dir:        fmt.Sprintf("ws://%s", env.ListenAddr),
	}
}

// Get returns a snapshot of the current configuration
func (c *Config) Get() Snapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Snapshot{
		Puerto:     c.Puerto,
		Marca:      c.Marca,
		ModoPrueba: c.ModoPrueba,
		Ambiente:   c.Ambiente,
		Dir:        c.Dir,
	}
}

// Snapshot is an immutable copy of configuration
type Snapshot struct {
	Puerto     string
	Marca      string
	ModoPrueba bool
	Ambiente   string
	Dir        string
}

// Update applies new configuration values
// Returns true if any value changed
func (c *Config) Update(puerto, marca string, modoPrueba bool) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Fill empty values with current
	if puerto == "" {
		puerto = c.Puerto
	}
	if marca == "" {
		marca = c.Marca
	}

	changed := c.Puerto != puerto || c.Marca != marca || c.ModoPrueba != modoPrueba

	if changed {
		c.Puerto = puerto
		c.Marca = marca
		c.ModoPrueba = modoPrueba
	}

	return changed
}
