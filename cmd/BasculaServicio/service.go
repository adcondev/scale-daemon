package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/judwhite/go-svc"
	"go.bug.st/serial"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// Variables de build, inyectadas en tiempo de compilación
var (
	BuildEnvironment = "prod"
	BuildDate        = "unknown"
	BuildTime        = "unknown"
)

const (
	serviceName     = "BasculaServicio"
	serviceNameTest = "BasculaServicioTest"
)

// Constante para tamaño máximo de logs
const maxLogSize = 5 * 1024 * 1024 // 5MB

// LogConfig Configuración de logging
type LogConfig struct {
	Verbose bool `json:"verbose"`
}

// Variables globales para logging
var (
	logConfig    = LogConfig{Verbose: true}
	logConfigMux sync.RWMutex
	logFilePath  string
	logFile      *os.File
)

// Prefijos de logs no críticos (se filtran cuando verbose=false)
var nonCriticalPrefixes = []string{
	"[~] Modo prueba activado",
	"[>] Peso enviado",
	"[!] No se recibió peso significativo",
	"[i] Configuración sin cambios",
	"[i] Iniciando escucha",
	"[i] Terminando escucha",
	"[+] Cliente conectado",
	"[-] Cliente desconectado",
}

// FilteredLogger Logger con filtrado de mensajes no críticos
type FilteredLogger struct {
	file *os.File
	mu   sync.Mutex
}

func (l *FilteredLogger) Write(p []byte) (n int, err error) {
	logConfigMux.RLock()
	verbose := logConfig.Verbose
	logConfigMux.RUnlock()

	if !verbose {
		msg := string(p)
		for _, prefix := range nonCriticalPrefixes {
			if strings.Contains(msg, prefix) {
				return len(p), nil // Descarta silenciosamente
			}
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	return l.file.Write(p)
}

// Autorrotación de logs si excede el tamaño máximo
func rotateLogIfNeeded(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Archivo no existe, no hay que rotar
		}
		return err
	}

	if info.Size() < maxLogSize {
		return nil // No excede el límite
	}

	// Rotar: mantener últimas 1000 líneas
	lines := readLastNLines(path, 1000)
	if len(lines) == 0 {
		return nil
	}

	content := strings.Join(lines, "\n") + "\n"
	return os.WriteFile(path, []byte(content), 0666)
}

// Lectura optimizada de últimas N líneas (reverse seek)
func readLastNLines(path string, n int) []string {
	file, err := os.Open(path)
	if err != nil {
		return []string{}
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return []string{}
	}

	size := stat.Size()
	if size == 0 {
		return []string{}
	}

	// Leer últimos 64KB máximo (suficiente para ~1000 líneas)
	bufSize := int64(64 * 1024)
	if size < bufSize {
		bufSize = size
	}

	buf := make([]byte, bufSize)
	_, err = file.Seek(size-bufSize, io.SeekStart)
	if err != nil {
		return []string{}
	}

	_, err = file.Read(buf)
	if err != nil {
		return []string{}
	}

	// Partir en líneas
	allLines := strings.Split(string(buf), "\n")

	// Limpiar líneas vacías al final
	for len(allLines) > 0 && allLines[len(allLines)-1] == "" {
		allLines = allLines[:len(allLines)-1]
	}

	// Si empezamos a leer a mitad de una línea, descartarla
	if size > bufSize && len(allLines) > 0 {
		allLines = allLines[1:]
	}

	if len(allLines) <= n {
		return allLines
	}
	return allLines[len(allLines)-n:]
}

// Flush del archivo de logs (mantiene últimas 50 líneas)
func flushLogFile() error {
	if logFilePath == "" {
		return fmt.Errorf("ruta de log no configurada")
	}

	lines := readLastNLines(logFilePath, 50)
	content := ""
	if len(lines) > 0 {
		content = strings.Join(lines, "\n") + "\n"
	}

	// Cerrar archivo actual, truncar, reabrir
	if logFile != nil {
		logFile.Close()
	}

	if err := os.WriteFile(logFilePath, []byte(content), 0666); err != nil {
		return err
	}

	// Reabrir archivo
	f, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	logFile = f
	log.SetOutput(&FilteredLogger{file: f})
	log.Println("[OK] Logs limpiados")

	return nil
}

// Obtener tamaño del archivo de logs
func getLogFileSize() int64 {
	if logFilePath == "" {
		return 0
	}
	info, err := os.Stat(logFilePath)
	if err != nil {
		return 0
	}
	return info.Size()
}

// Obtener estado actual de configuración de logs
func getLogStatus() map[string]interface{} {
	logConfigMux.RLock()
	verbose := logConfig.Verbose
	logConfigMux.RUnlock()

	return map[string]interface{}{
		"tipo":    "logStatus",
		"verbose": verbose,
		"size":    getLogFileSize(),
	}
}

// EnvironmentConfig guarda la configuración específica de entornos
type EnvironmentConfig struct {
	Name        string
	ServiceName string
	ListenAddr  string
	DefaultPort string
	DefaultMode bool
}

// TODO: usar campo para Puerto
// TODO: Agregar prod/local, test/remote
var envConfigs = map[string]EnvironmentConfig{
	"prod": {
		Name:        "PRODUCCIÓN",
		ServiceName: serviceName,
		ListenAddr:  "0.0.0.0:8765", // Escucha en todas las interfaces
		DefaultPort: "COM3",
		DefaultMode: false, // Producción empieza en modo real
	},
	"test": {
		Name:        "TEST/DEV",
		ServiceName: serviceNameTest,
		ListenAddr:  "localhost:8765", // Solo localhost
		DefaultPort: "COM3",
		DefaultMode: true, // Test empieza en modo prueba
	},
}

// Get current environment config
func getEnvConfig() EnvironmentConfig {
	if config, ok := envConfigs[BuildEnvironment]; ok {
		return config
	}
	return envConfigs["prod"] // Default a producción si no se reconoce
}

// Program implementa la interfaz svc.Service
type Program struct {
	wg   sync.WaitGroup
	quit chan struct{}
}

func (p *Program) Start() error {
	p.quit = make(chan struct{})
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()
		envConfig := getEnvConfig()

		log.Printf("[i] Servidor BASCULA - Ambiente: %s", envConfig.Name)
		log.Printf("[i] Build: %s %s", BuildDate, BuildTime)
		log.Printf("[i] Servidor activo en ws://%s", envConfig.ListenAddr)

		rand.Seed(time.Now().UnixNano())
		iniciarLectura()
		iniciarBroadcaster()

		go func() {
			if err := http.ListenAndServe(envConfig.ListenAddr, nil); err != nil {
				log.Printf("[X] Error al iniciar servidor: %v", err)
				close(p.quit)
			}
		}()

		<-p.quit
	}()

	return nil
}

func (p *Program) Stop() error {
	log.Println("[.] Servicio deteniéndose...")
	close(p.quit)
	p.wg.Wait()
	log.Println("[.] Servicio detenido")
	return nil
}

type Configuracion struct {
	Tipo       string `json:"tipo"`       // Tipo de mensaje
	Puerto     string `json:"puerto"`     // Puerto serial de la báscula
	Marca      string `json:"marca"`      // Marca de la báscula
	ModoPrueba bool   `json:"modoPrueba"` // Indica si está en modo prueba
	Dir        string `json:"dir"`        // Dirección del servidor WebSocket
	Ambiente   string `json:"ambiente"`   // Nuevo campo
}

var (
	configActual      Configuracion
	mutexConfig       sync.Mutex
	serialPort        serial.Port
	mutexSerial       sync.Mutex
	clientes          = make(map[*websocket.Conn]bool)
	clientesMutex     sync.Mutex
	broadcast         = make(chan string, 100)
	reintentoDelay    = 3 * time.Second
	serialReadTimeout = 5 * time.Second
)

// Inicializar configuración según ambiente
func init() {
	envConfig := getEnvConfig()
	configActual = Configuracion{
		Tipo:       "Configuración Inicial",
		Puerto:     envConfig.DefaultPort,
		Marca:      "Rhino BAR 8RS",
		ModoPrueba: envConfig.DefaultMode,
		Dir:        fmt.Sprintf("ws://%s", envConfig.ListenAddr),
		Ambiente:   envConfig.Name,
	}
}

var comandosPorMarca = map[string]string{
	"rhino":         "P",
	"rhino bar 8rs": "P",
}

func generarPesosSimulados() []float64 {
	base := rand.Float64()*29 + 1
	var pesos []float64
	for i := 0; i < 5; i++ {
		variacion := base + rand.Float64()*0.1 - 0.05
		pesos = append(pesos, float64(int(variacion*100))/100)
	}
	pesos = append(pesos, float64(int(base*100))/100)
	return pesos
}

func iniciarLectura() {
	go func() {
		for {
			mutexConfig.Lock()
			conf := configActual
			mutexConfig.Unlock()

			if conf.ModoPrueba {
				log.Printf("[~] Modo prueba activado - Ambiente: %s", conf.Ambiente)
				for _, peso := range generarPesosSimulados() {
					broadcast <- fmt.Sprintf("%.2f", peso)
					time.Sleep(300 * time.Millisecond)
				}
				time.Sleep(reintentoDelay)
				continue
			}

			modo := &serial.Mode{BaudRate: 9600}

			mutexSerial.Lock()
			port, err := serial.Open(conf.Puerto, modo)
			if err != nil {
				mutexSerial.Unlock()
				log.Printf("[X] No se pudo abrir el puerto serial %s: %v. Reintentando en %s...", conf.Puerto, err, reintentoDelay)
				time.Sleep(reintentoDelay)
				continue
			}
			serialPort = port
			mutexSerial.Unlock()

			err = serialPort.SetReadTimeout(serialReadTimeout)
			if err != nil {
				log.Printf("[!] Error al configurar timeout de lectura: %v. Cerrando y reintentando...", err)
				mutexSerial.Lock()
				serialPort.Close()
				serialPort = nil
				mutexSerial.Unlock()
				time.Sleep(reintentoDelay)
				continue
			}
			log.Printf("[OK] Conectado al puerto serial: %s", conf.Puerto)

			for {
				mutexConfig.Lock()
				conf = configActual
				mutexConfig.Unlock()

				mutexSerial.Lock()
				if serialPort == nil {
					mutexSerial.Unlock()
					log.Println("[i] Puerto serial cerrado, saliendo del bucle de lectura.")
					break
				}

				cmd, ok := comandosPorMarca[strings.ToLower(conf.Marca)]
				if !ok {
					cmd = "P"
				}

				_, err := serialPort.Write([]byte(cmd))
				if err != nil {
					log.Printf("[!] Error al escribir en el puerto: %v. Cerrando y reintentando...", err)
					serialPort.Close()
					serialPort = nil
					mutexSerial.Unlock()
					time.Sleep(reintentoDelay)
					break
				}

				time.Sleep(500 * time.Millisecond)

				buf := make([]byte, 20)
				n, err := serialPort.Read(buf)
				mutexSerial.Unlock()

				if err != nil {
					if err == io.EOF {
						log.Printf("[!] EOF recibido al leer del puerto %s. Posible desconexion.", conf.Puerto)
					} else if strings.Contains(err.Error(), "timeout") {
						log.Printf("[~] Timeout de lectura en puerto %s. Reintentando...", conf.Puerto)
						continue
					} else {
						log.Printf("[!] Error de lectura en puerto %s: %v", conf.Puerto, err)
						mutexSerial.Lock()
						if serialPort != nil {
							serialPort.Close()
							serialPort = nil
						}
						mutexSerial.Unlock()
						time.Sleep(reintentoDelay)
						break
					}
					continue
				}

				peso := strings.TrimSpace(string(buf[:n]))
				if peso != "" {
					log.Printf("[>] Peso enviado: %s", peso)
					broadcast <- peso
				} else {
					log.Println("[!] No se recibio peso significativo.")
				}

				time.Sleep(300 * time.Millisecond)
			}
			log.Printf("[~] Esperando %s antes de intentar reconectar al puerto serial...", reintentoDelay)
			time.Sleep(reintentoDelay)
		}
	}()
}

func iniciarBroadcaster() {
	go func() {
		for peso := range broadcast {
			clientesMutex.Lock()
			for c := range clientes {
				go func(conn *websocket.Conn, data string) {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second)
					defer cancel()
					err := wsjson.Write(ctx, conn, data)
					if err != nil {
						log.Printf("[!] Error al enviar a cliente: %v", err)
						clientesMutex.Lock()
						delete(clientes, conn)
						clientesMutex.Unlock()
						conn.Close(websocket.StatusInternalError, "Error de envio")
					}
				}(c, peso)
			}
			clientesMutex.Unlock()
		}
	}()
}

func listenForConfig(ctx context.Context, c *websocket.Conn) {
	log.Println("[i] Iniciando escucha de mensajes del cliente...")
	defer log.Println("[i] Terminando escucha de mensajes del cliente.")

	// Enviar configuración inicial al cliente
	mutexConfig.Lock()
	initialConfig := configActual
	mutexConfig.Unlock()

	// Enviar info del ambiente al conectarse
	envInfo := map[string]interface{}{
		"tipo":     "ambiente",
		"ambiente": initialConfig.Ambiente,
		"version":  fmt.Sprintf("%s %s", BuildDate, BuildTime),
		"config":   initialConfig,
	}

	ctx2, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_ = wsjson.Write(ctx2, c, envInfo)

	for {
		var mensaje map[string]interface{}
		err := wsjson.Read(ctx, c, &mensaje)

		if err != nil {
			if errors.Is(err, context.Canceled) {
				log.Println("[i] Contexto del cliente cancelado")
			} else if websocket.CloseStatus(err) == websocket.StatusNormalClosure || err == io.EOF {
				log.Println("[i] Cliente cerro la conexión normalmente")
			} else {
				log.Printf("[!] Error de lectura: %v", err)
			}
			break
		}

		tipo, ok := mensaje["tipo"].(string)
		if !ok {
			continue
		}

		// Switch para manejar diferentes tipos de mensajes
		switch tipo {
		case "config":
			handleConfigMessage(mensaje)
		// Handler para configuración de logs
		case "logConfig":
			if v, ok := mensaje["verbose"].(bool); ok {
				logConfigMux.Lock()
				logConfig.Verbose = v
				logConfigMux.Unlock()
				log.Printf("[OK] Verbosidad de logs: %v", v)

				// Enviar confirmación con estado actual
				ctx3, cancel3 := context.WithTimeout(ctx, time.Second)
				_ = wsjson.Write(ctx3, c, getLogStatus())
				cancel3()
			}
		// Handler para flush de logs
		case "logFlush":
			if err := flushLogFile(); err != nil {
				log.Printf("[X] Error en flush de logs: %v", err)
				ctx3, cancel3 := context.WithTimeout(ctx, time.Second)
				_ = wsjson.Write(ctx3, c, map[string]interface{}{
					"tipo":  "logFlushResult",
					"ok":    false,
					"error": err.Error(),
				})
				cancel3()
			} else {
				ctx3, cancel3 := context.WithTimeout(ctx, time.Second)
				_ = wsjson.Write(ctx3, c, map[string]interface{}{
					"tipo": "logFlushResult",
					"ok":   true,
				})
				cancel3()
			}
		// Handler para tail de logs
		case "logTail":
			lines := 100
			if n, ok := mensaje["lines"].(float64); ok {
				lines = int(n)
			}
			tailLines := readLastNLines(logFilePath, lines)

			ctx3, cancel3 := context.WithTimeout(ctx, time.Second)
			_ = wsjson.Write(ctx3, c, map[string]interface{}{
				"tipo":  "logLines",
				"lines": tailLines,
			})
			cancel3()
		// Handler para obtener estado de logs
		case "logStatus":
			ctx3, cancel3 := context.WithTimeout(ctx, time.Second)
			_ = wsjson.Write(ctx3, c, getLogStatus())
			cancel3()
		}
	}
}

// Función extraída para manejar mensajes de configuración
func handleConfigMessage(mensaje map[string]interface{}) {
	data, _ := json.Marshal(mensaje)
	var nuevaConfig Configuracion
	_ = json.Unmarshal(data, &nuevaConfig)

	mutexConfig.Lock()
	actual := configActual
	if nuevaConfig.Puerto == "" {
		nuevaConfig.Puerto = actual.Puerto
	}
	if nuevaConfig.Marca == "" {
		nuevaConfig.Marca = actual.Marca
	}
	nuevaConfig.Ambiente = actual.Ambiente
	mutexConfig.Unlock()

	log.Printf("[i] Configuración recibida: %+v", nuevaConfig)

	mutexConfig.Lock()
	mismoPuerto := nuevaConfig.Puerto == configActual.Puerto
	mismaMarca := nuevaConfig.Marca == configActual.Marca
	mismoModo := nuevaConfig.ModoPrueba == configActual.ModoPrueba
	mutexConfig.Unlock()

	if !mismoPuerto || !mismaMarca || !mismoModo {
		log.Println("[*] Cambiando configuración...")
		mutexSerial.Lock()
		if serialPort != nil {
			log.Println("[.] Cerrando puerto serial...")
			serialPort.Close()
			serialPort = nil
		}
		mutexSerial.Unlock()

		mutexConfig.Lock()
		configActual = nuevaConfig
		mutexConfig.Unlock()
		log.Println("[OK] Configuración actualizada")
	} else {
		log.Println("[i] Configuración sin cambios")
	}
}

func manejarCliente(w http.ResponseWriter, r *http.Request) {
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
		OriginPatterns:     []string{"*"},
	})
	if err != nil {
		log.Printf("[X] Error al aceptar cliente: %v", err)
		return
	}
	defer c.Close(websocket.StatusInternalError, "cerrando")

	ctx := r.Context()

	clientesMutex.Lock()
	clientes[c] = true
	clientesMutex.Unlock()

	envConfig := getEnvConfig()
	log.Printf("[+] Cliente conectado - Ambiente: %s", envConfig.Name)

	go listenForConfig(ctx, c)

	<-ctx.Done()

	clientesMutex.Lock()
	delete(clientes, c)
	clientesMutex.Unlock()
	log.Println("[-] Cliente desconectado")
}

// Init con logging a archivo en ambos ambientes y rotación
func (p *Program) Init(env svc.Environment) error {
	envConfig := getEnvConfig()

	// Configurar ruta del archivo de logs
	logFilePath = filepath.Join(os.Getenv("PROGRAMDATA"), envConfig.ServiceName, envConfig.ServiceName+".log")

	// Crear directorio si no existe
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		return err
	}

	// Auto-rotar si excede 5MB
	if err := rotateLogIfNeeded(logFilePath); err != nil {
		// No es crítico, continuar
		fmt.Printf("[!] Error en rotación de logs: %v\n", err)
	}

	// Abrir archivo de logs
	f, err := os.OpenFile(logFilePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	logFile = f

	// Usar logger filtrado
	log.SetOutput(&FilteredLogger{file: f})

	// Default: prod = no verbose, test = verbose
	logConfigMux.Lock()
	logConfig.Verbose = (BuildEnvironment == "test")
	logConfigMux.Unlock()

	log.Printf("[i] Iniciando Servicio - Ambiente: %s", envConfig.Name)
	log.Printf("[i] Build: %s %s", BuildDate, BuildTime)
	log.Printf("[i] Logs en: %s", logFilePath)
	log.Printf("[i] Verbose: %v", logConfig.Verbose)

	return nil
}

func main() {
	http.HandleFunc("/", manejarCliente)

	prg := &Program{}
	if err := svc.Run(prg, syscall.SIGINT, syscall.SIGTERM); err != nil {
		log.Fatal(err)
	}
}
