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

// EnvironmentConfig guarda la configuración específica de entornos
type EnvironmentConfig struct {
	Name        string
	ServiceName string
	ListenAddr  string
	DefaultPort string
	DefaultMode bool
}

// TODO: usar campo para Puerto
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
				log.Println("[i] Cliente cerro la conexion normalmente")
			} else {
				log.Printf("[!] Error de lectura: %v", err)
			}
			break
		}

		if tipo, ok := mensaje["tipo"]; ok && tipo == "config" {
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
			// Mantener el ambiente actual
			nuevaConfig.Ambiente = actual.Ambiente
			mutexConfig.Unlock()

			log.Printf("[i] Configuracion recibida: %+v", nuevaConfig)

			mutexConfig.Lock()
			mismoPuerto := nuevaConfig.Puerto == configActual.Puerto
			mismaMarca := nuevaConfig.Marca == configActual.Marca
			mismoModo := nuevaConfig.ModoPrueba == configActual.ModoPrueba
			mutexConfig.Unlock()

			if !mismoPuerto || !mismaMarca || !mismoModo {
				log.Println("[*] Cambiando configuracion...")
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
				log.Println("[OK] Configuracion actualizada")
			} else {
				log.Println("[i] Configuracion sin cambios")
			}
		}
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

func (p *Program) Init(env svc.Environment) error {
	envConfig := getEnvConfig()

	// Solo escribir a archivo en test/dev
	if BuildEnvironment == "test" {
		logFile := filepath.Join(os.Getenv("PROGRAMDATA"), envConfig.ServiceName, envConfig.ServiceName+".log")
		err := os.MkdirAll(filepath.Dir(logFile), 0755)
		if err != nil {
			return err
		}
		f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			return err
		}
		log.SetOutput(f)
	} else {
		// En prod, logs van a consola (stdout)
		log.SetOutput(os.Stdout)
	}

	log.Printf("[i] Iniciando Servicio - Ambiente: %s", envConfig.Name)
	log.Printf("[i] Build: %s %s", BuildDate, BuildTime)
	return nil
}

func main() {
	http.HandleFunc("/", manejarCliente)

	prg := &Program{}
	if err := svc.Run(prg, syscall.SIGINT, syscall.SIGTERM); err != nil {
		log.Fatal(err)
	}
}
