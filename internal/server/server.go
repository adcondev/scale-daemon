// Package server handles WebSocket connections, HTTP endpoints, and configuration updates for the R2k Ticket Servicio dashboard.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"

	"github.com/adcondev/scale-daemon/internal/auth"
	"github.com/adcondev/scale-daemon/internal/config"
	"github.com/adcondev/scale-daemon/internal/logging"

	embedded "github.com/adcondev/scale-daemon"
)

const maxConfigChangesPerMinute = 15

// Server handles HTTP and WebSocket connections
type Server struct {
	config         *config.Config
	env            config.Environment
	broadcaster    *Broadcaster
	logMgr         *logging.Manager
	auth           *auth.Manager
	configLimiter  *ConfigRateLimiter
	buildInfo      string
	onConfigChange func()
	buildDate      string
	buildTime      string
	startTime      time.Time
	mu             sync.RWMutex
	lastWeightTime time.Time
	httpServer     *http.Server
	dashboardTmpl  *template.Template
}

// NewServer creates a new server instance
func NewServer(
	cfg *config.Config,
	env config.Environment,
	broadcaster *Broadcaster,
	logMgr *logging.Manager,
	authMgr *auth.Manager,
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
		auth:           authMgr,
		configLimiter:  NewConfigRateLimiter(maxConfigChangesPerMinute), // Max 15 config changes per minute per client
		buildInfo:      buildInfo,
		onConfigChange: onConfigChange,
		buildDate:      buildDate,
		buildTime:      buildTime,
		startTime:      startTime,
	}

	// Setup embedded filesystem
	webFS, err := fs.Sub(embedded.WebFiles, "internal/assets/web")
	if err != nil {
		log.Fatalf("[FATAL] Error loading web assets: %v", err)
	}

	// Parse index.html as a Go template for token injection
	indexBytes, err := fs.ReadFile(webFS, "index.html")
	if err != nil {
		log.Fatalf("[FATAL] Error reading index.html: %v", err)
	}
	s.dashboardTmpl, err = template.New("dashboard").Parse(string(indexBytes))
	if err != nil {
		log.Fatalf("[FATAL] Error parsing index.html as template: %v", err)
	}

	// Setup HTTP handlers with correct auth boundaries
	mux := http.NewServeMux()

	// ── PUBLIC ROUTES (no auth required) ─────────────────────
	// Static assets must be public so login.html can load CSS
	mux.Handle("/css/", http.FileServer(http.FS(webFS)))
	mux.Handle("/js/", http.FileServer(http.FS(webFS)))
	mux.HandleFunc("/login", s.serveLoginPage(webFS))
	mux.HandleFunc("/auth/login", s.handleLogin)
	mux.HandleFunc("/auth/logout", s.handleLogout)
	mux.HandleFunc("/ping", s.HandlePing)
	mux.HandleFunc("/ws", s.handleWebSocket)
	mux.HandleFunc("/health", s.HandleHealth)

	// ── PROTECTED ROUTES (session required) ──────────────────

	mux.HandleFunc("/", s.requireAuth(s.serveDashboard))

	s.httpServer = &http.Server{
		Addr:         env.ListenAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return s
}

// ═══════════════════════════════════════════════════════════════
// AUTH MIDDLEWARE & HANDLERS
// ═══════════════════════════════════════════════════════════════

// requireAuth wraps a HandlerFunc with session validation.
// If auth is disabled (no hash), all requests pass through.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.auth.Enabled() {
			next(w, r)
			return
		}
		if !s.auth.GetSessionFromRequest(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}

// serveLoginPage returns a handler that serves login.html from the embedded FS.
func (s *Server) serveLoginPage(webFS fs.FS) http.HandlerFunc {
	loginHTML, err := fs.ReadFile(webFS, "login.html")
	if err != nil {
		log.Fatalf("[FATAL] Error reading login.html: %v", err)
	}
	return func(w http.ResponseWriter, r *http.Request) {
		// If auth is disabled, skip login entirely
		if !s.auth.Enabled() {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		// If already authenticated, redirect to dashboard
		if s.auth.GetSessionFromRequest(r) {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(loginHTML)
	}
}

// serveDashboard renders index.html as a Go template, injecting the config auth token.
// This solves the "static file injection paradox": index.html is a template, not a static file.
func (s *Server) serveDashboard(w http.ResponseWriter, r *http.Request) {
	// Only serve dashboard for root path (avoid catching /favicon.ico etc.)
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	data := struct {
		AuthToken string
	}{
		AuthToken: config.AuthToken,
	}
	if err := s.dashboardTmpl.Execute(w, data); err != nil {
		log.Printf("[X] Error rendering dashboard template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// handleLogin processes POST /auth/login
func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := r.RemoteAddr

	// Check lockout FIRST
	if s.auth.IsLockedOut(ip) {
		log.Printf("[AUDIT] LOGIN_BLOCKED | IP=%s | reason=lockout", ip)
		http.Redirect(w, r, "/login?locked=1", http.StatusSeeOther)
		return
	}

	password := r.FormValue("password")
	if !s.auth.ValidatePassword(password) {
		s.auth.RecordFailedLogin(ip)
		log.Printf("[AUDIT] LOGIN_FAILED | IP=%s", ip)
		http.Redirect(w, r, "/login?error=1", http.StatusSeeOther)
		return
	}

	// Success
	s.auth.ClearFailedLogins(ip)
	s.auth.SetSessionCookie(w)
	log.Printf("[AUDIT] LOGIN_SUCCESS | IP=%s", ip)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleLogout clears the session
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// ═══════════════════════════════════════════════════════════════
// WEBSOCKET
// ═══════════════════════════════════════════════════════════════

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		OriginPatterns:     s.allowedOrigins(),
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

	s.broadcaster.AddClient(c)
	log.Printf("[+] Client connected (Total: %d)", s.broadcaster.ClientCount())

	s.sendEnvironmentInfo(ctx, c)
	s.listenForMessages(ctx, c)

	s.broadcaster.RemoveClient(c)
	log.Println("[-] Client disconnected")
}

// allowedOrigins returns environment-specific WebSocket origin patterns.
func (s *Server) allowedOrigins() []string {
	if s.env.Name == "LOCAL" {
		return []string{"localhost:*", "127.0.0.1:*"}
	}
	// Remote: allow common private network ranges
	return []string{"192.168.*.*:*", "10.*.*.*:*", "172.16.*.*:*", "localhost:*"}
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
		// ── RATE LIMIT CHECK ─────────────────────────────────
		// Use connection pointer address as unique client identifier
		clientAddr := fmt.Sprintf("%p", c)
		if !s.configLimiter.Allow(clientAddr) {
			log.Printf("[AUDIT] CONFIG_RATE_LIMITED | client=%s", clientAddr)
			s.sendJSON(ctx, c, ErrorResponse{Tipo: "error", Error: "RATE_LIMITED"})
			return
		}
		s.handleConfigMessage(ctx, c, mensaje)

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

func (s *Server) handleConfigMessage(ctx context.Context, c *websocket.Conn, mensaje map[string]interface{}) {
	// Parse into struct for type safety
	data, _ := json.Marshal(mensaje)
	var configMsg ConfigMessage
	if err := json.Unmarshal(data, &configMsg); err != nil {
		log.Printf("[X] Error parsing config message: %v", err)
		return
	}

	// ── TOKEN VALIDATION ─────────────────────────────────────
	if config.AuthToken != "" && configMsg.AuthToken != config.AuthToken {
		log.Printf("[AUDIT] CONFIG_REJECTED | reason=invalid_token | puerto=%s marca=%s",
			configMsg.Puerto, configMsg.Marca)
		s.sendJSON(ctx, c, ErrorResponse{Tipo: "error", Error: "AUTH_INVALID_TOKEN"})
		return
	}

	log.Printf("[AUDIT] CONFIG_ACCEPTED | puerto=%s marca=%s modoPrueba=%v",
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

// ═══════════════════════════════════════════════════════════════
// HTTP ENDPOINTS
// ═══════════════════════════════════════════════════════════════

// HandlePing responds with "pong" for health checks.
func (s *Server) HandlePing(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/plain")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("pong"))
}

// HandleHealth returns service health and scale connection status.
func (s *Server) HandleHealth(w http.ResponseWriter, _ *http.Request) {
	cfg := s.config.Get()

	s.mu.RLock()
	isConnected := !s.lastWeightTime.IsZero() && time.Since(s.lastWeightTime) < 15*time.Second
	s.mu.RUnlock()

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
		Uptime: int(time.Since(s.startTime).Seconds()),
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	_ = json.NewEncoder(w).Encode(response)
}

func (s *Server) sendJSON(ctx context.Context, c *websocket.Conn, v interface{}) {
	ctx2, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_ = wsjson.Write(ctx2, c, v)
}

// RecordWeightActivity updates the last weight timestamp for health checks.
func (s *Server) RecordWeightActivity() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastWeightTime = time.Now()
}

// ListenAndServe starts the HTTP server and logs the active endpoints and auth status.
func (s *Server) ListenAndServe() error {
	log.Printf("[i] Dashboard active at http://%s/", s.env.ListenAddr)
	log.Printf("[i] WebSocket active at ws://%s/ws", s.env.ListenAddr)
	log.Printf("[i] Auth enabled: %v", s.auth.Enabled())
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the HTTP server with a timeout context.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return errors.New("server Shutdown called with nil httpServer; invariant violated")
	}
	return s.httpServer.Shutdown(ctx)
}
