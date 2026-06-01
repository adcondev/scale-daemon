package config

import (
	"testing"
)

func TestConfig_Update(t *testing.T) {
	tests := []struct {
		name       string
		initial    *Config
		puerto     string
		marca      string
		modoPrueba bool
		wantChange bool
		wantPuerto string
		wantMarca  string
		wantModo   bool
	}{
		{
			name: "update all fields",
			initial: &Config{
				Puerto:     "COM1",
				Marca:      "MarcaA",
				ModoPrueba: false,
			},
			puerto:     "COM2",
			marca:      "MarcaB",
			modoPrueba: true,
			wantChange: true,
			wantPuerto: "COM2",
			wantMarca:  "MarcaB",
			wantModo:   true,
		},
		{
			name: "no change",
			initial: &Config{
				Puerto:     "COM1",
				Marca:      "MarcaA",
				ModoPrueba: false,
			},
			puerto:     "COM1",
			marca:      "MarcaA",
			modoPrueba: false,
			wantChange: false,
			wantPuerto: "COM1",
			wantMarca:  "MarcaA",
			wantModo:   false,
		},
		{
			name: "empty strings use current values",
			initial: &Config{
				Puerto:     "COM1",
				Marca:      "MarcaA",
				ModoPrueba: false,
			},
			puerto:     "",
			marca:      "",
			modoPrueba: true,
			wantChange: true,
			wantPuerto: "COM1",
			wantMarca:  "MarcaA",
			wantModo:   true,
		},
		{
			name: "empty strings and no change in bool",
			initial: &Config{
				Puerto:     "COM1",
				Marca:      "MarcaA",
				ModoPrueba: false,
			},
			puerto:     "",
			marca:      "",
			modoPrueba: false,
			wantChange: false,
			wantPuerto: "COM1",
			wantMarca:  "MarcaA",
			wantModo:   false,
		},
		{
			name: "update only puerto",
			initial: &Config{
				Puerto:     "COM1",
				Marca:      "MarcaA",
				ModoPrueba: false,
			},
			puerto:     "COM2",
			marca:      "",
			modoPrueba: false,
			wantChange: true,
			wantPuerto: "COM2",
			wantMarca:  "MarcaA",
			wantModo:   false,
		},
		{
			name: "update only marca",
			initial: &Config{
				Puerto:     "COM1",
				Marca:      "MarcaA",
				ModoPrueba: false,
			},
			puerto:     "",
			marca:      "MarcaB",
			modoPrueba: false,
			wantChange: true,
			wantPuerto: "COM1",
			wantMarca:  "MarcaB",
			wantModo:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a new Config instance using pointer to avoid copylocks
			c := &Config{
				Puerto:     tt.initial.Puerto,
				Marca:      tt.initial.Marca,
				ModoPrueba: tt.initial.ModoPrueba,
				Ambiente:   tt.initial.Ambiente,
				Dir:        tt.initial.Dir,
			}

			changed := c.Update(tt.puerto, tt.marca, tt.modoPrueba)

			if changed != tt.wantChange {
				t.Errorf("Update() returned %v, want %v", changed, tt.wantChange)
			}
			if c.Puerto != tt.wantPuerto {
				t.Errorf("Puerto after Update() = %v, want %v", c.Puerto, tt.wantPuerto)
			}
			if c.Marca != tt.wantMarca {
				t.Errorf("Marca after Update() = %v, want %v", c.Marca, tt.wantMarca)
			}
			if c.ModoPrueba != tt.wantModo {
				t.Errorf("ModoPrueba after Update() = %v, want %v", c.ModoPrueba, tt.wantModo)
			}
		})
	}
}

func TestConfig_Get(t *testing.T) {
	c := &Config{
		Puerto:     "COM1",
		Marca:      "MarcaA",
		ModoPrueba: true,
		Ambiente:   "LOCAL",
		Dir:        "ws://localhost:8888",
	}

	snap := c.Get()

	if snap.Puerto != c.Puerto {
		t.Errorf("Get() Puerto = %v, want %v", snap.Puerto, c.Puerto)
	}
	if snap.Marca != c.Marca {
		t.Errorf("Get() Marca = %v, want %v", snap.Marca, c.Marca)
	}
	if snap.ModoPrueba != c.ModoPrueba {
		t.Errorf("Get() ModoPrueba = %v, want %v", snap.ModoPrueba, c.ModoPrueba)
	}
	if snap.Ambiente != c.Ambiente {
		t.Errorf("Get() Ambiente = %v, want %v", snap.Ambiente, c.Ambiente)
	}
	if snap.Dir != c.Dir {
		t.Errorf("Get() Dir = %v, want %v", snap.Dir, c.Dir)
	}
}

func TestNew(t *testing.T) {
	env := Environment{
		Name:        "LOCAL",
		ServiceName: "TestService",
		ListenAddr:  "localhost:8888",
		DefaultPort: "COM3",
		DefaultMode: false,
	}

	c := New(env)

	if c.Puerto != env.DefaultPort {
		t.Errorf("New() Puerto = %v, want %v", c.Puerto, env.DefaultPort)
	}
	if c.Marca != "Rhino BAR 8RS" {
		t.Errorf("New() Marca = %v, want Rhino BAR 8RS", c.Marca)
	}
	if c.ModoPrueba != env.DefaultMode {
		t.Errorf("New() ModoPrueba = %v, want %v", c.ModoPrueba, env.DefaultMode)
	}
	if c.Ambiente != env.Name {
		t.Errorf("New() Ambiente = %v, want %v", c.Ambiente, env.Name)
	}
	if c.Dir != "ws://localhost:8888" {
		t.Errorf("New() Dir = %v, want ws://localhost:8888", c.Dir)
	}
}

func TestGetEnvironment(t *testing.T) {
	// Setup variables for test
	origServerPort := ServerPort
	ServerPort = "8888"
	defer func() { ServerPort = origServerPort }()

	// Reset Environments just in case
	Environments = map[string]Environment{
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

	tests := []struct {
		name     string
		envName  string
		wantName string
	}{
		{
			name:     "valid local environment",
			envName:  "local",
			wantName: "LOCAL",
		},
		{
			name:     "valid remote environment",
			envName:  "remote",
			wantName: "REMOTO",
		},
		{
			name:     "invalid environment fallback to local",
			envName:  "invalid",
			wantName: "LOCAL",
		},
		{
			name:     "empty environment fallback to local",
			envName:  "",
			wantName: "LOCAL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := GetEnvironment(tt.envName)
			if env.Name != tt.wantName {
				t.Errorf("GetEnvironment(%q) Name = %v, want %v", tt.envName, env.Name, tt.wantName)
			}
		})
	}
}
