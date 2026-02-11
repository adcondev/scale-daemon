package scale

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"go.bug.st/serial"

	"github.com/adcondev/scale-daemon/internal/config"
)

const (
	RetryDelay        = 3 * time.Second
	SerialReadTimeout = 5 * time.Second
	BaudRate          = 9600
)

const (
	ErrEOF     = "ERR_EOF"
	ErrTimeout = "ERR_TIMEOUT"
	ErrRead    = "ERR_READ"
)

var ErrorDescriptions = map[string]string{
	ErrEOF:     "EOF recibido. Posible desconexión.",
	ErrTimeout: "Timeout de lectura.",
	ErrRead:    "Error de lectura.",
}

// BrandCommands maps scale brands to their weight request commands
var BrandCommands = map[string]string{
	"rhino":         "P",
	"rhino bar 8rs": "P",
}

// GetCommand returns the command for a given brand
// Defaults to "P" if brand is unknown
func GetCommand(brand string) string {
	if cmd, ok := BrandCommands[strings.ToLower(brand)]; ok {
		return cmd
	}
	return "P"
}

// GenerateSimulatedWeights creates a sequence of realistic weight readings
// Returns 5 fluctuating values followed by a stable reading
func GenerateSimulatedWeights() []float64 {
	base := rand.Float64()*29 + 1 // 1.0 to 30.0
	var weights []float64

	// 5 readings with small fluctuation
	for i := 0; i < 5; i++ {
		variation := base + rand.Float64()*0.1 - 0.05
		weights = append(weights, float64(int(variation*100))/100)
	}

	// Final stable reading
	weights = append(weights, float64(int(base*100))/100)

	return weights
}

// Reader manages serial port communication with the scale
type Reader struct {
	config    *config.Config
	broadcast chan<- string
	port      serial.Port
	mu        sync.Mutex
	stopCh    chan struct{}
}

// NewReader creates a new scale reader
func NewReader(cfg *config.Config, broadcast chan<- string) *Reader {
	return &Reader{
		config:    cfg,
		broadcast: broadcast,
		stopCh:    make(chan struct{}),
	}
}

// Start begins the reading loop (blocking)
func (r *Reader) Start(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			r.closePort()
			return
		case <-r.stopCh:
			r.closePort()
			return
		default:
			r.readCycle(ctx)
		}
	}
}

// Stop signals the reader to stop
func (r *Reader) Stop() {
	close(r.stopCh)
}

// ClosePort closes the serial port for config changes
func (r *Reader) ClosePort() {
	r.closePort()
}

func (r *Reader) closePort() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.port != nil {
		err := r.port.Close()
		if err != nil {
			return
		}
		r.port = nil
	}
}

func (r *Reader) readCycle(ctx context.Context) {
	conf := r.config.Get()

	// Test mode: generate simulated weights
	if conf.ModoPrueba {
		log.Printf("[~] Modo prueba activado - Ambiente: %s", conf.Ambiente)
		for _, peso := range GenerateSimulatedWeights() {
			select {
			case <-ctx.Done():
				return
			case r.broadcast <- fmt.Sprintf("%.2f", peso):
			}
			time.Sleep(300 * time.Millisecond)
		}
		time.Sleep(RetryDelay)
		return
	}

	// Real mode: connect to serial port
	if err := r.connect(conf.Puerto); err != nil {
		log.Printf("[X] No se pudo abrir el puerto serial %s: %v. Reintentando en %s...",
			conf.Puerto, err, RetryDelay)
		time.Sleep(RetryDelay)
		return
	}

	log.Printf("[OK] Conectado al puerto serial: %s", conf.Puerto)

	// Read loop
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check if config changed
		newConf := r.config.Get()

		r.mu.Lock()
		if r.port == nil {
			r.mu.Unlock()
			log.Println("[i] Puerto serial cerrado, saliendo del bucle de lectura.")
			break
		}

		// Send weight request command
		cmd := GetCommand(newConf.Marca)
		_, err := r.port.Write([]byte(cmd))
		if err != nil {
			log.Printf("[!] Error al escribir en el puerto: %v. Cerrando y reintentando...", err)
			err := r.port.Close()
			if err != nil {
				return
			}
			r.port = nil
			r.mu.Unlock()
			time.Sleep(RetryDelay)
			break
		}

		time.Sleep(500 * time.Millisecond)

		// Read response
		buf := make([]byte, 20)
		n, err := r.port.Read(buf)
		r.mu.Unlock()

		if err != nil {
			if err == io.EOF {
				log.Printf("[!] %s: %s", ErrorDescriptions[ErrEOF], conf.Puerto)
				r.sendError(ErrEOF)
			} else if strings.Contains(err.Error(), "timeout") {
				log.Printf("[~] %s: %s. Reintentando...", ErrorDescriptions[ErrTimeout], conf.Puerto)
				r.sendError(ErrTimeout)
				continue
			} else {
				log.Printf("[!] %s: %s - %v", ErrorDescriptions[ErrRead], conf.Puerto, err)
				r.sendError(ErrRead)
				r.closePort()
				time.Sleep(RetryDelay)
				break
			}
			continue
		}

		peso := strings.TrimSpace(string(buf[:n]))
		if peso != "" {
			log.Printf("[>] Peso enviado: %s", peso)
			select {
			case r.broadcast <- peso:
			default:
				// Channel full, skip
			}
		} else {
			log.Println("[!] No se recibió peso significativo.")
		}

		time.Sleep(300 * time.Millisecond)
	}

	log.Printf("[~] Esperando %s antes de intentar reconectar al puerto serial...", RetryDelay)
	time.Sleep(RetryDelay)
}

func (r *Reader) sendError(code string) {
	select {
	case r.broadcast <- code:
	default:
		// Channel full, skip
	}
}

func (r *Reader) connect(puerto string) error {
	mode := &serial.Mode{BaudRate: BaudRate}

	r.mu.Lock()
	defer r.mu.Unlock()

	port, err := serial.Open(puerto, mode)
	if err != nil {
		return err
	}

	if err := port.SetReadTimeout(SerialReadTimeout); err != nil {
		err := port.Close()
		if err != nil {
			return err
		}
		return err
	}

	r.port = port
	return nil
}
