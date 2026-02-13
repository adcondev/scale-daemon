package config

import (
	"testing"
)

func TestConfig_Update(t *testing.T) {
	// Base configuration values
	basePuerto := "COM3"
	baseMarca := "Rhino"
	baseModoPrueba := false
	baseAmbiente := "test-env"
	baseDir := "ws://localhost:1234"

	tests := []struct {
		name string
		// Initial state
		initialPuerto     string
		initialMarca      string
		initialModoPrueba bool
		// Arguments to Update
		argsPuerto     string
		argsMarca      string
		argsModoPrueba bool
		// Expectations
		wantChanged bool
		wantPuerto  string
		wantMarca   string
		wantModo    bool
	}{
		{
			name:              "No changes",
			initialPuerto:     basePuerto,
			initialMarca:      baseMarca,
			initialModoPrueba: baseModoPrueba,
			argsPuerto:        basePuerto,
			argsMarca:         baseMarca,
			argsModoPrueba:    baseModoPrueba,
			wantChanged:       false,
			wantPuerto:        basePuerto,
			wantMarca:         baseMarca,
			wantModo:          baseModoPrueba,
		},
		{
			name:              "Update all fields",
			initialPuerto:     basePuerto,
			initialMarca:      baseMarca,
			initialModoPrueba: baseModoPrueba,
			argsPuerto:        "COM4",
			argsMarca:         "Torrey",
			argsModoPrueba:    true,
			wantChanged:       true,
			wantPuerto:        "COM4",
			wantMarca:         "Torrey",
			wantModo:          true,
		},
		{
			name:              "Update partial fields (empty strings should be ignored)",
			initialPuerto:     basePuerto,
			initialMarca:      baseMarca,
			initialModoPrueba: baseModoPrueba,
			argsPuerto:        "",
			argsMarca:         "Torrey",
			argsModoPrueba:    baseModoPrueba,
			wantChanged:       true,
			wantPuerto:        basePuerto,
			wantMarca:         "Torrey",
			wantModo:          baseModoPrueba,
		},
		{
			name:              "Update only boolean field",
			initialPuerto:     basePuerto,
			initialMarca:      baseMarca,
			initialModoPrueba: baseModoPrueba,
			argsPuerto:        basePuerto,
			argsMarca:         baseMarca,
			argsModoPrueba:    true,
			wantChanged:       true,
			wantPuerto:        basePuerto,
			wantMarca:         baseMarca,
			wantModo:          true,
		},
		{
			name:              "Empty strings with same values (no change)",
			initialPuerto:     basePuerto,
			initialMarca:      baseMarca,
			initialModoPrueba: baseModoPrueba,
			argsPuerto:        "",
			argsMarca:         "",
			argsModoPrueba:    baseModoPrueba,
			wantChanged:       false,
			wantPuerto:        basePuerto,
			wantMarca:         baseMarca,
			wantModo:          baseModoPrueba,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize Config for this test run
			c := &Config{
				Puerto:     tt.initialPuerto,
				Marca:      tt.initialMarca,
				ModoPrueba: tt.initialModoPrueba,
				Ambiente:   baseAmbiente,
				Dir:        baseDir,
			}

			gotChanged := c.Update(tt.argsPuerto, tt.argsMarca, tt.argsModoPrueba)

			if gotChanged != tt.wantChanged {
				t.Errorf("Config.Update() = %v, want %v", gotChanged, tt.wantChanged)
			}

			if c.Puerto != tt.wantPuerto {
				t.Errorf("Config.Puerto = %v, want %v", c.Puerto, tt.wantPuerto)
			}
			if c.Marca != tt.wantMarca {
				t.Errorf("Config.Marca = %v, want %v", c.Marca, tt.wantMarca)
			}
			if c.ModoPrueba != tt.wantModo {
				t.Errorf("Config.ModoPrueba = %v, want %v", c.ModoPrueba, tt.wantModo)
			}
			// Verify other fields remain unchanged
			if c.Ambiente != baseAmbiente {
				t.Errorf("Config.Ambiente changed unexpectedly: got %v, want %v", c.Ambiente, baseAmbiente)
			}
			if c.Dir != baseDir {
				t.Errorf("Config.Dir changed unexpectedly: got %v, want %v", c.Dir, baseDir)
			}
		})
	}
}
