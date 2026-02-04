package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// NonCriticalPrefixes are log messages filtered when verbose=false
var NonCriticalPrefixes = []string{
	"[~] Modo prueba activado",
	"[>] Peso enviado",
	"[!] No se recibió peso significativo",
	"[i] Configuración sin cambios",
	"[i] Iniciando escucha",
	"[i] Terminando escucha",
	"[+] Cliente conectado",
	"[-] Cliente desconectado",
}

// FilteredLogger wraps a file writer with verbose filtering
type FilteredLogger struct {
	file    *os.File
	mu      sync.Mutex
	verbose *bool
	vMu     *sync.RWMutex
}

// NewFilteredLogger creates a logger that can filter non-critical messages
func NewFilteredLogger(file *os.File, verbose *bool, vMu *sync.RWMutex) *FilteredLogger {
	return &FilteredLogger{
		file:    file,
		verbose: verbose,
		vMu:     vMu,
	}
}

func (l *FilteredLogger) Write(p []byte) (n int, err error) {
	l.vMu.RLock()
	verbose := *l.verbose
	l.vMu.RUnlock()

	if !verbose {
		msg := string(p)
		for _, prefix := range NonCriticalPrefixes {
			if strings.Contains(msg, prefix) {
				return len(p), nil // Silently discard
			}
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Write(p)
}

// Manager handles log file lifecycle and configuration
type Manager struct {
	FilePath string
	file     *os.File
	Verbose  bool
	mu       sync.RWMutex
}

// Setup initializes logging to file with stdout fallback for console mode
func Setup(serviceName string, defaultVerbose bool) (*Manager, error) {
	mgr := &Manager{
		Verbose: defaultVerbose,
	}

	logDir := filepath.Join(os.Getenv("PROGRAMDATA"), serviceName)
	mgr.FilePath = filepath.Join(logDir, serviceName+".log")

	// Try to create log directory
	if err := os.MkdirAll(logDir, 0755); err != nil {
		// Permission denied - fallback to stdout (console mode)
		log.SetOutput(os.Stdout)
		log.Printf("[i] Logging to stdout (no write access to %s)", logDir)
		return mgr, nil
	}

	// Auto-rotate if needed
	if err := RotateIfNeeded(mgr.FilePath); err != nil {
		fmt.Printf("[!] Log rotation error: %v\n", err)
	}

	// Open log file
	f, err := os.OpenFile(mgr.FilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		// Fallback to stdout
		log.SetOutput(os.Stdout)
		log.Printf("[i] Logging to stdout (cannot open %s: %v)", mgr.FilePath, err)
		return mgr, nil
	}

	mgr.file = f
	log.SetOutput(NewFilteredLogger(f, &mgr.Verbose, &mgr.mu))
	log.Printf("[i] Logging to: %s", mgr.FilePath)

	return mgr, nil
}

// Close closes the log file
func (m *Manager) Close() error {
	if m.file != nil {
		return m.file.Close()
	}
	return nil
}

// SetVerbose updates the verbose setting
func (m *Manager) SetVerbose(v bool) {
	m.mu.Lock()
	m.Verbose = v
	m.mu.Unlock()
	log.Printf("[OK] Verbosidad de logs: %v", v)
}

// GetVerbose returns current verbose setting
func (m *Manager) GetVerbose() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.Verbose
}

// Flush reduces the log file keeping last 50 lines
func (m *Manager) Flush() error {
	if m.FilePath == "" {
		return fmt.Errorf("log path not configured")
	}

	// Close current file
	if m.file != nil {
		m.file.Close()
	}

	// Flush to last 50 lines
	if err := Flush(m.FilePath); err != nil {
		return err
	}

	// Reopen file
	f, err := os.OpenFile(m.FilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	m.file = f
	log.SetOutput(NewFilteredLogger(f, &m.Verbose, &m.mu))
	log.Println("[OK] Logs limpiados")

	return nil
}

// GetStatus returns current log status
func (m *Manager) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"tipo":    "logStatus",
		"verbose": m.GetVerbose(),
		"size":    GetFileSize(m.FilePath),
	}
}

// GetTail returns the last n lines of the log
func (m *Manager) GetTail(n int) []string {
	return ReadLastNLines(m.FilePath, n)
}

// Closer returns an io.Closer for the log file
func (m *Manager) Closer() io.Closer {
	if m.file != nil {
		return m.file
	}
	return nopCloser{}
}

type nopCloser struct{}

func (nopCloser) Close() error { return nil }
