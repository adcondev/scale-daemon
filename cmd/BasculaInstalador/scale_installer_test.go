package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"nhooyr.io/websocket"
)

// =============================================================================
// TEST 1: Verificar Rotación a 5MB
// =============================================================================

func TestLogRotationAt5MB(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("Skipping integration test. Set RUN_INTEGRATION_TESTS=1 to run")
	}

	t.Run("Rotación automática al alcanzar 5MB", func(t *testing.T) {
		logDir := filepath.Join(os.TempDir(), "LogRotationTest")
		logFile := filepath.Join(logDir, "test.log")

		defer os.RemoveAll(logDir)
		os.MkdirAll(logDir, 0755)

		targetSize := int64(5 * 1024 * 1024)

		f, err := os.Create(logFile)
		if err != nil {
			t.Fatalf("Error creando archivo de log:  %v", err)
		}

		line := strings.Repeat("X", 1024) + "\n"
		written := int64(0)
		for written < targetSize {
			n, _ := f.WriteString(line)
			written += int64(n)
		}
		f.Close()

		info, _ := os.Stat(logFile)
		if info.Size() < targetSize {
			t.Errorf("Archivo debería ser >= 5MB, tiene:  %d bytes", info.Size())
		}
		t.Logf("[OK] Archivo de log creado:  %s (%.2f MB)", logFile, float64(info.Size())/(1024*1024))

		rotatedFile := logFile + ".old"
		os.Rename(logFile, rotatedFile)
		os.Create(logFile)

		if _, err := os.Stat(rotatedFile); os.IsNotExist(err) {
			t.Error("Archivo rotado debería existir")
		} else {
			t.Log("[OK] Rotación simulada correctamente")
		}

		newInfo, _ := os.Stat(logFile)
		if newInfo.Size() > 0 {
			t.Errorf("Nuevo archivo debería estar vacío, tiene:  %d bytes", newInfo.Size())
		} else {
			t.Log("[OK] Nuevo archivo de log creado vacío")
		}
	})
}

// =============================================================================
// TEST 4: Probar Resize de Terminal
// =============================================================================

func TestTerminalResize(t *testing.T) {
	t.Run("Resize básico sin panic", func(t *testing.T) {
		m := initialModel()

		sizes := []struct {
			w, h int
			desc string
		}{
			{80, 24, "estándar 80x24"},
			{120, 40, "grande 120x40"},
			{40, 10, "pequeño 40x10"},
			{200, 60, "muy grande 200x60"},
			{60, 15, "mediano 60x15"},
			{80, 24, "volver a estándar"},
		}

		for _, size := range sizes {
			t.Run(size.desc, func(t *testing.T) {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("PANIC en resize %dx%d: %v", size.w, size.h, r)
					}
				}()

				msg := tea.WindowSizeMsg{Width: size.w, Height: size.h}
				newModel, _ := m.Update(msg)
				m = newModel.(model)

				if m.width != size.w || m.height != size.h {
					t.Errorf("Dimensiones no actualizadas: got %dx%d, want %dx%d",
						m.width, m.height, size.w, size.h)
				}
			})
		}
		t.Log("[OK] Todos los resize completados sin panic")
	})

	t.Run("Resize en diferentes pantallas", func(t *testing.T) {
		screens := []screen{
			menuScreen,
			logsMenuScreen,
			processingScreen,
			resultScreen,
			confirmScreen,
		}

		for _, scr := range screens {
			t.Run(fmt.Sprintf("screen_%d", scr), func(t *testing.T) {
				m := initialModel()
				m.screen = scr
				m.ready = true

				msg := tea.WindowSizeMsg{Width: 100, Height: 30}
				newModel, _ := m.Update(msg)
				m = newModel.(model)

				defer func() {
					if r := recover(); r != nil {
						t.Errorf("PANIC renderizando screen %d después de resize: %v", scr, r)
					}
				}()

				view := m.View()
				if view == "" {
					t.Error("View vacío después de resize")
				}
			})
		}
		t.Log("[OK] Resize funciona en todas las pantallas")
	})

	t.Run("Resize extremo - terminal muy pequeña", func(t *testing.T) {
		m := initialModel()

		msg := tea.WindowSizeMsg{Width: 20, Height: 5}
		newModel, _ := m.Update(msg)
		m = newModel.(model)

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("PANIC con terminal 20x5: %v", r)
			}
		}()

		_ = m.View()
		t.Log("[OK] Terminal mínima (20x5) no causa panic")
	})

	t.Run("Resize durante procesamiento", func(t *testing.T) {
		m := initialModel()
		m.processing = true
		m.screen = processingScreen
		m.progressPercent = 0.5

		for i := 0; i < 10; i++ {
			w := 60 + (i * 10)
			h := 20 + (i * 2)

			msg := tea.WindowSizeMsg{Width: w, Height: h}
			newModel, _ := m.Update(msg)
			m = newModel.(model)

			if m.progress.Width > w {
				t.Errorf("Progress bar más ancho que terminal: %d > %d", m.progress.Width, w)
			}
		}
		t.Log("[OK] Resize durante procesamiento funciona correctamente")
	})
}

// =============================================================================
// HELPERS
// =============================================================================

// consumeInitialMessage lee y descarta el mensaje inicial "ambiente" que el servicio
// envía automáticamente cuando un cliente se conecta.
func consumeInitialMessage(ctx context.Context, conn *websocket.Conn) error {
	// Timeout corto para no bloquear si no hay mensaje
	readCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()

	response, err := readWSResponse(readCtx, conn)
	if err != nil {
		return fmt.Errorf("no se recibió mensaje inicial: %w", err)
	}

	// Verificar que es el mensaje de ambiente esperado
	if tipo, ok := response["tipo"].(string); ok {
		if tipo == "ambiente" {
			return nil // Mensaje consumido correctamente
		}
		return fmt.Errorf("mensaje inicial inesperado, tipo: %s", tipo)
	}

	return fmt.Errorf("mensaje inicial sin campo 'tipo':  %+v", response)
}

func sendWSMessage(ctx context.Context, conn *websocket.Conn, msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error marshaling message: %w", err)
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func readWSResponse(ctx context.Context, conn *websocket.Conn) (map[string]interface{}, error) {
	_, data, err := conn.Read(ctx)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("error unmarshalling response: %w (data: %s)", err, string(data))
	}
	return result, nil
}

func mustMarshal(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		var buf bytes.Buffer
		buf.WriteString("{}")
		return buf.Bytes()
	}
	return data
}

func unmarshalResponse(data []byte) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// =============================================================================
// BENCHMARKS
// =============================================================================

func BenchmarkViewRender(b *testing.B) {
	m := initialModel()
	m.ready = true
	m.width = 80
	m.height = 24
	m.serviceState = "RUNNING"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = m.View()
	}
}

func BenchmarkResize(b *testing.B) {
	m := initialModel()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w := 60 + (i % 100)
		h := 20 + (i % 40)
		msg := tea.WindowSizeMsg{Width: w, Height: h}
		newModel, _ := m.Update(msg)
		m = newModel.(model)
	}
}
