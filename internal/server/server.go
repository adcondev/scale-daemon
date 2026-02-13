// Package server implements the HTTP and WebSocket server for the scale daemon.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/adcondev/scale-daemon/internal/config"
	"github.com/adcondev/scale-daemon/internal/logging"

	"github.com/adcondev/scale-daemon"
)

// Server handles HTTP and WebSocket connections
type Server struct {
	config         *config.Config
	env            config.Environment
	broadcaster    *Broadcaster
	logMgr         *logging.Manager
	buildInfo      string
	onConfigChange func()
	buildDate      string
	buildTime      string
	startTime      time.Time
	mu             sync.RWMutex
	lastWeightTime time.Time
	httpServer     *http.Server
}

// NewServer creates a new server instance
func NewServer(
	cfg *config.Config,
	env config.Environment,
	broadcaster *Broadcaster,
	logMgr *logging.Manager,
	buildInfo string,
	onConfigChange func(),
	buildDate string,
	buildTime string,
	startTime time.Time,
) *Server {
	s := &Server{
		config:         cfg,
		env:            env,
		broadcaster:    broadcaster,
		logMgr:         logMgr,
		buildInfo:      buildInfo,
		onConfigChange: onConfigChange,
		buildDate:      buildDate,
		buildTime:      buildTime,
		startTime:      startTime,
	}

	// Setup HTTP handlers
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.HandleHealth)
	mux.HandleFunc("/ping", s.HandlePing)

	// Setup FS
	webFS, err := fs.Sub(embedded.WebFiles, "internal/assets/web")
	if err != nil {
		// Panic is acceptable here as service cannot function without assets
		log.Fatalf("[FATAL] Error loading web assets: %v", err)
	}
	mux.Handle("/", http.FileServer(http.FS(webFS)))

	// This ensures s.httpServer is NEVER nil once NewServer returns
	s.httpServer = &http.Server{
		Addr:    env.ListenAddr,
		Handler: mux,
		// ALWAYS add timeouts to prevent Slowloris attacks
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// handleWebSocket upgrades the connection
func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// No need to check "Upgrade" header here manually;
	// clients connecting to /ws likely intend to upgrade.

	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// FIXME: InsecureSkipVerify should be false in production with proper certs
		InsecureSkipVerify: true,
		OriginPatterns:     []string{"*"},
	})

	if err != nil {
		log.Printf("[X] Error accepting websocket: %v", err)
		return
	}
	defer func(c *websocket.Conn, code websocket.StatusCode, reason string) {
		err := c.Close(code, reason)
		if err != nil {
			log.Printf("[!] Error closing websocket: %v", err)
		}
	}(c, websocket.StatusInternalError, "closing")

	ctx := r.Context()

	// Register client
	s.broadcaster.AddClient(c)
	log.Printf("[+] Client connected (Total: %d)", s.broadcaster.ClientCount())

	// Send initial state
	s.sendEnvironmentInfo(ctx, c)

	// Listen for incoming config messages
	s.listenForMessages(ctx, c)

	// Cleanup
	s.broadcaster.RemoveClient(c)
	log.Println("[-] Client disconnected")
}

func (s *Server) sendEnvironmentInfo(ctx context.Context, c *websocket.Conn) {
	conf := s.config.Get()

	envInfo := EnvironmentInfo{
		Tipo:     "ambiente",
		Ambiente: conf.Ambiente,
		Version:  s.buildInfo,
		Config: ConfigForClient{
			Puerto:     conf.Puerto,
			Marca:      conf.Marca,
			ModoPrueba: conf.ModoPrueba,
			Dir:        conf.Dir,
			Ambiente:   conf.Ambiente,
		},
	}

	ctx2, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_ = wsjson.Write(ctx2, c, envInfo)
}

func (s *Server) listenForMessages(ctx context.Context, c *websocket.Conn) {
	log.Println("[i] Iniciando escucha de mensajes del cliente...")
	defer log.Println("[i] Terminando escucha de mensajes del cliente.")

	for {
		var mensaje map[string]interface{}
		err := wsjson.Read(ctx, c, &mensaje)

		if err != nil {
			switch {
			case errors.Is(err, context.Canceled):
				log.Println("[i] Contexto del cliente cancelado")
			case websocket.CloseStatus(err) == websocket.StatusNormalClosure || errors.Is(err, io.EOF):
				log.Println("[i] Cliente cerró la conexión normalmente")
			default:
				log.Printf("[!] Error de lectura: %v", err)
			}
			break
		}

		tipo, ok := mensaje["tipo"].(string)
		if !ok {
			continue
		}

		s.handleMessage(ctx, c, tipo, mensaje)
	}
}

func (s *Server) handleMessage(ctx context.Context, c *websocket.Conn, tipo string, mensaje map[string]interface{}) {
	switch tipo {
	case "config":
		s.handleConfigMessage(mensaje)

	case "logConfig":
		if v, ok := mensaje["verbose"].(bool); ok {
			s.logMgr.SetVerbose(v)
			s.sendJSON(ctx, c, s.logMgr.GetStatus())
		}

	case "logFlush":
		result := LogFlushResult{Tipo: "logFlushResult"}
		if err := s.logMgr.Flush(); err != nil {
			result.OK = false
			result.Error = err.Error()
			log.Printf("[X] Error en flush de logs: %v", err)
		} else {
			result.OK = true
		}
		s.sendJSON(ctx, c, result)

	case "logTail":
		lines := 100
		if n, ok := mensaje["lines"].(float64); ok {
			lines = int(n)
		}
		tailLines := s.logMgr.GetTail(lines)
		s.sendJSON(ctx, c, LogLinesResponse{
			Tipo:  "logLines",
			Lines: tailLines,
		})

	case "logStatus":
		s.sendJSON(ctx, c, s.logMgr.GetStatus())
	}
}

func (s *Server) handleConfigMessage(mensaje map[string]interface{}) {
	// Parse into struct for type safety
	data, _ := json.Marshal(mensaje)
	var configMsg ConfigMessage
	err := json.Unmarshal(data, &configMsg)
	if err != nil {
		log.Printf("[X] Error parsing config message: %v", err)
		return
	}

	log.Printf("[i] Configuración recibida: Puerto=%s Marca=%s ModoPrueba=%v",
		configMsg.Puerto, configMsg.Marca, configMsg.ModoPrueba)

	if s.config.Update(configMsg.Puerto, configMsg.Marca, configMsg.ModoPrueba) {
		log.Println("[*] Cambiando configuración...")
		if s.onConfigChange != nil {
			s.onConfigChange()
		}
		log.Println("[OK] Configuración actualizada")
	} else {
		log.Println("[i] Configuración sin cambios")
	}
}

// HandlePing is a lightweight liveness check
func (s *Server) HandlePing(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("pong"))
	if err != nil {
		return
	}
}

// HandleHealth returns service health metrics
func (s *Server) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	cfg := s.config.Get()

	// If last weight was received < 15 seconds ago, assume connected.
	// Adjust threshold based on your poll interval.
	s.mu.RLock()
	isConnected := !s.lastWeightTime.IsZero() && time.Since(s.lastWeightTime) < 15*time.Second
	s.mu.RUnlock()

	// If in Test Mode, we are always "connected" to the generator
	if cfg.ModoPrueba {
		isConnected = true
	}

	response := HealthResponse{
		Status: "ok",
		Scale: ScaleStatus{
			Connected: isConnected,
			Port:      cfg.Puerto,
			Brand:     cfg.Marca,
			TestMode:  cfg.ModoPrueba,
		},
		Build: BuildInfo{
			Env:  s.env.Name,
			Date: s.buildDate,
			Time: s.buildTime,
		},
		// Safe uptime calculation
		Uptime: int(time.Since(s.startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		return
	}
}

func (s *Server) sendJSON(ctx context.Context, c *websocket.Conn, v interface{}) {
	// Record activity whenever we successfully send data (e.g. weight updates)
	s.recordWeightActivity()

	ctx2, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_ = wsjson.Write(ctx2, c, v)
}

func (s *Server) recordWeightActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastWeightTime = time.Now()
}

// ListenAndServe inicia el servidor HTTP
func (s *Server) ListenAndServe() error {
	log.Printf("[i] Dashboard active at http://%s/", s.env.ListenAddr)
	log.Printf("[i] WebSocket active at ws://%s/ws", s.env.ListenAddr)

	// Just start the already-configured server
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return errors.New("server Shutdown called with nil httpServer; invariant violated")
	}
	return s.httpServer.Shutdown(ctx)
}
