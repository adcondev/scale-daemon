// Package config provides configuration management for the scale service, including environment-specific settings and runtime configuration updates.
package config

import (
	"fmt"
	"log"
	"sync"
)

// Build variables (injected via ldflags)
var (
	// ServiceName is the name of the service, used for logging and identification.
	ServiceName = "R2k_BasculaServicio_Local"
	// BuildEnvironment defines the deployment target (local, remote).
	BuildEnvironment = "local"
	// BuildDate is the date the binary was built.
	BuildDate = "unknown"
	// BuildTime is the time the binary was built.
	BuildTime = "unknown"
	// PasswordHashB64 is injected at build time via ldflags.
	// It contains a bcrypt hash, NOT the plaintext password.
	PasswordHashB64 = ""
	// AuthToken is injected at build time via ldflags.
	// If empty, config messages are accepted without token validation.
	AuthToken = ""
	// ServerPort is the default port for the scale service, can be overridden by environment config.
	ServerPort = ""
)

// Environment holds environment-specific settings
type Environment struct {
	Name        string
	ServiceName string
	ListenAddr  string
	DefaultPort string
	DefaultMode bool // true = test mode (simulated weights), false = real weights
}

// TODO: Make Port inyectable via ldflags. Same port and addres could cause conflicts

// Environments defines available deployment configurations
var Environments = map[string]Environment{
	"remote": {
		Name:        "REMOTO",
		ServiceName: ServiceName,
		ListenAddr:  "0.0.0.0:" + ServerPort,
		DefaultPort: "COM3",
		DefaultMode: false,
	},
	"local": {
		Name:        "LOCAL",
		ServiceName: ServiceName,
		ListenAddr:  "localhost:" + ServerPort,
		DefaultPort: "COM3",
		DefaultMode: false,
	},
}

// GetEnvironment returns config for the specified environment
// Falls back to "prod" if unknown
func GetEnvironment(env string) Environment {
	if cfg, ok := Environments[env]; ok {
		return cfg
	}
	log.Printf("[!] Unknown environment '%s', defaulting to 'local'", env)
	return Environments["local"]
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
