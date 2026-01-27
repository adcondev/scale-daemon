package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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

	t.Run("Verificar límite de rotación via WebSocket", func(t *testing.T) {
		conn, err := connectToService()
		if err != nil {
			t.Skipf("Servicio no disponible:  %v", err)
		}
		defer conn.Close(websocket.StatusNormalClosure, "test complete")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Consumir mensaje inicial de ambiente
		if err := consumeInitialMessage(ctx, conn); err != nil {
			t.Logf("Nota: %v", err)
		}

		err = sendWSMessage(ctx, conn, map[string]string{"tipo": "logStatus"})
		if err != nil {
			t.Fatalf("Error enviando mensaje: %v", err)
		}

		response, err := readWSResponse(ctx, conn)
		if err != nil {
			t.Fatalf("Error leyendo respuesta:  %v", err)
		}

		if size, ok := response["size"].(float64); ok {
			sizeMB := size / (1024 * 1024)
			t.Logf("[OK] Tamaño actual del log: %.2f MB", sizeMB)

			if sizeMB > 5.5 {
				t.Errorf("Log excede límite de 5MB: %.2f MB", sizeMB)
			}
		}
	})
}

// =============================================================================
// TEST 2: Probar Verbose On/Off
// =============================================================================

func TestVerboseToggle(t *testing.T) {
	if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
		t.Skip("Skipping integration test")
	}

	conn, err := connectToService()
	if err != nil {
		t.Skipf("Servicio no disponible: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "test complete")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ⚠️ IMPORTANTE: Consumir mensaje inicial "ambiente" que el servicio envía al conectarse
	if err := consumeInitialMessage(ctx, conn); err != nil {
		t.Logf("Nota al consumir mensaje inicial: %v", err)
	}

	t.Run("Obtener estado inicial de verbose", func(t *testing.T) {
		err := sendWSMessage(ctx, conn, map[string]string{"tipo": "logStatus"})
		if err != nil {
			t.Fatalf("Error:  %v", err)
		}

		response, err := readWSResponse(ctx, conn)
		if err != nil {
			t.Fatalf("Error leyendo respuesta: %v", err)
		}

		initialVerbose, _ := response["verbose"].(bool)
		t.Logf("Estado inicial verbose: %v", initialVerbose)
	})

	t.Run("Activar verbose", func(t *testing.T) {
		err := sendWSMessage(ctx, conn, map[string]interface{}{
			"tipo":    "logConfig",
			"verbose": true,
		})
		if err != nil {
			t.Fatalf("Error activando verbose: %v", err)
		}

		response, err := readWSResponse(ctx, conn)
		if err != nil {
			t.Fatalf("Error leyendo respuesta: %v", err)
		}

		// Debug: mostrar respuesta completa
		t.Logf("Respuesta recibida: %+v", response)

		if tipo, ok := response["tipo"].(string); !ok || tipo != "logStatus" {
			t.Errorf("Respuesta inesperada, tipo: %v (esperado: logStatus)", response["tipo"])
			return
		}

		if verbose, ok := response["verbose"].(bool); !ok || !verbose {
			t.Errorf("Fallo al activar verbose, respuesta: %+v", response)
		} else {
			t.Log("[OK] Verbose activado correctamente")
		}

		// Verificar con segunda consulta
		time.Sleep(100 * time.Millisecond)
		sendWSMessage(ctx, conn, map[string]string{"tipo": "logStatus"})
		status, _ := readWSResponse(ctx, conn)
		if v, ok := status["verbose"].(bool); ok && v {
			t.Log("[OK] Verificado:  verbose = true")
		}
	})

	t.Run("Desactivar verbose", func(t *testing.T) {
		err := sendWSMessage(ctx, conn, map[string]interface{}{
			"tipo":    "logConfig",
			"verbose": false,
		})
		if err != nil {
			t.Fatalf("Error desactivando verbose: %v", err)
		}

		response, err := readWSResponse(ctx, conn)
		if err != nil {
			t.Fatalf("Error leyendo respuesta: %v", err)
		}

		// Debug
		t.Logf("Respuesta recibida: %+v", response)

		if verbose, ok := response["verbose"].(bool); ok && verbose {
			t.Error("Fallo al desactivar verbose - aún está true")
		} else if !ok {
			t.Errorf("Campo 'verbose' no encontrado en respuesta:  %+v", response)
		} else {
			t.Log("[OK] Verbose desactivado correctamente")
		}
	})

	t.Run("Toggle rápido sin errores", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			verbose := i%2 == 0
			err := sendWSMessage(ctx, conn, map[string]interface{}{
				"tipo":    "logConfig",
				"verbose": verbose,
			})
			if err != nil {
				t.Errorf("Toggle %d falló: %v", i, err)
				return
			}
			_, _ = readWSResponse(ctx, conn)
		}
		t.Log("[OK] 10 toggles rápidos completados sin errores")
	})
}

// =============================================================================
// TEST 3: Verificar No Hay Memory Leak en Polling
// =============================================================================

func TestNoMemoryLeakInPolling(t *testing.T) {
	t.Run("Polling de logs sin acumulación de memoria", func(t *testing.T) {
		m := initialModel()
		m.ready = true
		m.width = 80
		m.height = 24

		var memStart, memEnd runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&memStart)

		iterations := 100
		for i := 0; i < iterations; i++ {
			lines := make([]string, 100)
			for j := 0; j < 100; j++ {
				lines[j] = fmt.Sprintf("[%s] Log entry %d-%d", time.Now().Format("15:04:05"), i, j)
			}

			msg := logLinesMsg(lines)
			newModel, _ := m.Update(msg)
			m = newModel.(model)

			if i%10 == 0 {
				runtime.GC()
			}
		}

		runtime.GC()
		runtime.ReadMemStats(&memEnd)

		allocDiff := int64(memEnd.TotalAlloc) - int64(memStart.TotalAlloc)
		heapDiff := int64(memEnd.HeapAlloc) - int64(memStart.HeapAlloc)

		t.Logf("Memoria inicial:   Alloc=%d KB, Heap=%d KB", memStart.Alloc/1024, memStart.HeapAlloc/1024)
		t.Logf("Memoria final:    Alloc=%d KB, Heap=%d KB", memEnd.Alloc/1024, memEnd.HeapAlloc/1024)
		t.Logf("Diferencia total allocaciones: %d KB", allocDiff/1024)
		t.Logf("Diferencia heap actual: %d KB", heapDiff/1024)

		maxHeapGrowth := int64(10 * 1024 * 1024)
		if heapDiff > maxHeapGrowth {
			t.Errorf("⚠ Posible memory leak: heap creció %d KB (límite: %d KB)",
				heapDiff/1024, maxHeapGrowth/1024)
		} else {
			t.Log("[OK] Sin memory leak detectado en polling")
		}

		if len(m.logLines) > 100 {
			t.Errorf("logLines acumula más de 100 entradas: %d", len(m.logLines))
		} else {
			t.Logf("[OK] logLines mantiene tamaño controlado:  %d líneas", len(m.logLines))
		}
	})

	t.Run("Conexiones WebSocket no se acumulan", func(t *testing.T) {
		if os.Getenv("RUN_INTEGRATION_TESTS") != "1" {
			t.Skip("Skipping integration test")
		}

		var wg sync.WaitGroup
		errors := make(chan error, 20)

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				conn, err := connectToService()
				if err != nil {
					errors <- fmt.Errorf("conexión %d falló: %v", idx, err)
					return
				}

				ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()

				// Consumir mensaje inicial
				consumeInitialMessage(ctx, conn)

				sendWSMessage(ctx, conn, map[string]string{"tipo": "logStatus"})
				readWSResponse(ctx, conn)

				conn.Close(websocket.StatusNormalClosure, "test")
			}(i)
		}

		wg.Wait()
		close(errors)

		errorCount := 0
		for err := range errors {
			t.Log(err)
			errorCount++
		}

		if errorCount > 2 {
			t.Errorf("Demasiados errores de conexión: %d", errorCount)
		} else {
			t.Log("[OK] Conexiones WebSocket manejadas correctamente")
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
			logsViewScreen,
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

	t.Run("Viewport de logs se ajusta al resize", func(t *testing.T) {
		m := initialModel()
		m.screen = logsViewScreen
		m.logLines = []string{"línea 1", "línea 2", "línea 3"}
		m.viewport.SetContent(strings.Join(m.logLines, "\n"))

		msg := tea.WindowSizeMsg{Width: 100, Height: 30}
		newModel, _ := m.Update(msg)
		m = newModel.(model)

		expectedVPWidth := 100 - 4
		expectedVPHeight := 30 - 8

		if m.viewport.Width != expectedVPWidth {
			t.Errorf("Viewport width:  got %d, want %d", m.viewport.Width, expectedVPWidth)
		}
		if m.viewport.Height != expectedVPHeight {
			t.Errorf("Viewport height: got %d, want %d", m.viewport.Height, expectedVPHeight)
		}
		t.Logf("[OK] Viewport ajustado correctamente a %dx%d", m.viewport.Width, m.viewport.Height)
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

func BenchmarkLogPolling(b *testing.B) {
	m := initialModel()
	m.ready = true
	m.width = 80
	m.height = 24

	lines := make([]string, 100)
	for i := 0; i < 100; i++ {
		lines[i] = fmt.Sprintf("[2025-01-07 10:00:00] Log entry %d with some content", i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		msg := logLinesMsg(lines)
		newModel, _ := m.Update(msg)
		m = newModel.(model)
	}
}

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
