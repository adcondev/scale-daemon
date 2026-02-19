// Package daemon implements the main service logic, including lifecycle management and coordination of components like the scale reader and WebSocket server.
package daemon

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/judwhite/go-svc"

	"github.com/adcondev/scale-daemon/internal/auth"
	"github.com/adcondev/scale-daemon/internal/config"
	"github.com/adcondev/scale-daemon/internal/logging"
	"github.com/adcondev/scale-daemon/internal/scale"
	"github.com/adcondev/scale-daemon/internal/server"
)

// Service implements svc.Service for Windows Service Control Manager
type Service struct {
	// Build info (injected via ldflags)
	BuildEnvironment string
	BuildDate        string
	BuildTime        string
	timeStart        time.Time

	// Components
	env         config.Environment
	cfg         *config.Config
	logMgr      *logging.Manager
	reader      *scale.Reader
	broadcaster *server.Broadcaster
	srv         *server.Server
	authMgr     *auth.Manager

	// Lifecycle
	broadcast chan string
	wg        sync.WaitGroup
	quit      chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
}

// New creates a new service instance
func New(buildEnv, buildDate, buildTime string) *Service {
	return &Service{
		BuildEnvironment: buildEnv,
		BuildDate:        buildDate,
		BuildTime:        buildTime,
		broadcast:        make(chan string, 100),
	}
}

// Init implements svc.Service
func (s *Service) Init(_ svc.Environment) error {
	s.timeStart = time.Now()
	s.env = config.GetEnvironment(s.BuildEnvironment)

	// Setup logging
	defaultVerbose := s.BuildEnvironment == "test"
	logMgr, err := logging.Setup(s.env.ServiceName, defaultVerbose)
	if err != nil {
		return err
	}
	s.logMgr = logMgr

	log.Printf("[i] Iniciando Servicio - Ambiente: %s", s.env.Name)
	log.Printf("[i] Build: %s %s", s.BuildDate, s.BuildTime)
	log.Printf("[i] Verbose: %v", s.logMgr.GetVerbose())

	// Initialize config
	s.cfg = config.New(s.env)

	return nil
}

// Start implements svc.Service
func (s *Service) Start() error {
	s.quit = make(chan struct{})
	s.ctx, s.cancel = context.WithCancel(context.Background())

	// Create auth manager (bound to service ctx for clean shutdown)
	s.authMgr = auth.NewManager(s.ctx)

	// Create broadcaster with weight activity callback
	s.broadcaster = server.NewBroadcaster(s.broadcast, func() {
		s.srv.RecordWeightActivity()
	})

	// Create scale reader
	s.reader = scale.NewReader(s.cfg, s.broadcast)

	// Create HTTP/WebSocket server
	buildInfo := fmt.Sprintf("%s %s", s.BuildDate, s.BuildTime)
	s.srv = server.NewServer(
		s.cfg,
		s.env,
		s.broadcaster,
		s.logMgr,
		s.authMgr,
		buildInfo,
		s.onConfigChange,
		s.BuildDate,
		s.BuildTime,
		s.timeStart,
	)

	// Start components
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.run()
	}()

	return nil
}

func (s *Service) run() {
	log.Printf("[i] Servidor BASCULA - Ambiente: %s", s.env.Name)
	log.Printf("[i] Build: %s %s", s.BuildDate, s.BuildTime)

	// Start broadcaster
	go s.broadcaster.Start(s.ctx)

	// Start scale reader
	go s.reader.Start(s.ctx)

	// Start HTTP server
	go func() {
		if err := s.srv.ListenAndServe(); err != nil {
			// http.ErrServerClosed is expected during graceful shutdown.
			// Stop() closes s.quit after Shutdown() completes, avoiding double-close.
			if !errors.Is(err, http.ErrServerClosed) {
				log.Printf("[X] Error al iniciar servidor: %v", err)
				// Cancel context to stop broadcaster and reader goroutines
				s.cancel()
				select {
				case <-s.quit:
					// Channel already closed; no action needed.
				default:
					close(s.quit)
				}
			}
		}
	}()

	<-s.quit
}

// Stop implements svc.Service
func (s *Service) Stop() error {
	log.Println("[.] Servicio deteniéndose...")

	// 1. Cancel the context (signals broadcaster and reader)
	s.cancel()

	// 2. Stop the serial reader
	s.reader.Stop()

	// 3. Gracefully shut down the HTTP/WS server (with timeout)
	// This causes ListenAndServe() to return with http.ErrServerClosed
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := s.srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("[!] Error al cerrar servidor HTTP: %v", err)
	}

	// 4. Close the quit channel to unblock run()
	// Safe because select checks if already closed by error path
	select {
	case <-s.quit:
		// Already closed, do nothing
	default:
		close(s.quit)
	}

	// 5. Now wg.Wait() will return because run() can unblock
	s.wg.Wait()

	// 6. Close log file
	if s.logMgr != nil {
		if err := s.logMgr.Close(); err != nil {
			log.Printf("[!] Error al cerrar logs: %v", err)
		}
	}

	log.Println("[.] Servicio detenido")
	return nil
}

// onConfigChange is called when config changes via WebSocket
func (s *Service) onConfigChange() {
	log.Println("[.] Cerrando puerto serial...")
	s.reader.ClosePort()
}
