package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io" // Importar el paquete io
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.bug.st/serial"
	//"go.bug.st/serial/enumerator" // Posiblemente necesites esto si quieres listar puertos
	"github.com/judwhite/go-svc"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

// ... (mantén las mismas estructuras y variables globales) ...

// Program implementa la interfaz svc.Service
type Program struct {
	wg   sync.WaitGroup
	quit chan struct{}
}

// Start is called after Init. This method must be non-blocking.
func (p *Program) Start() error {
	p.quit = make(chan struct{})
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()
		// Run your existing WebSocket server code here
		log.Println("🌐 Servidor activo en ws://localhost:8765")

		// Initialize your existing components
		rand.Seed(time.Now().UnixNano())
		iniciarLectura()
		iniciarBroadcaster()

		// Start HTTP server in a goroutine
		go func() {
			if err := http.ListenAndServe("localhost:8765", nil); err != nil {
				log.Printf("🛑 Error al iniciar servidor: %v", err)
				// Signal the service to stop on server failure
				close(p.quit)
			}
		}()

		// Wait for quit signal
		<-p.quit
	}()

	return nil
}

// Stop is called when the service is being stopped
func (p *Program) Stop() error {
	log.Println("Service stopping...")
	// Signal the service to stop
	close(p.quit)
	// Wait for all goroutines to exit
	p.wg.Wait()
	log.Println("Service stopped")
	return nil
}

type Configuracion struct {
	Tipo       string `json:"tipo"`
	Puerto     string `json:"puerto"`
	Marca      string `json:"marca"`
	ModoPrueba bool   `json:"modoPrueba"`
}

var (
	configActual      = Configuracion{Puerto: "COM7", Marca: "rhino", ModoPrueba: true}
	mutexConfig       sync.Mutex
	serialPort        serial.Port
	mutexSerial       sync.Mutex
	clientes          = make(map[*websocket.Conn]bool)
	clientesMutex     sync.Mutex
	broadcast         = make(chan string, 100)
	serialRunning     = false // Aún sin usar
	reintentoDelay    = 3 * time.Second
	serialReadTimeout = 5 * time.Second // Timeout para la lectura serial
)

var comandosPorMarca = map[string]string{
	"rhino": "P",
}

func generarPesosSimulados() []float64 {
	base := rand.Float64()*29 + 1
	pesos := []float64{}
	for i := 0; i < 5; i++ {
		variacion := base + rand.Float64()*0.1 - 0.05
		pesos = append(pesos, float64(int(variacion*100))/100)
	}
	pesos = append(pesos, float64(int(base*100))/100)
	return pesos
}

// iniciarLectura permanece igual
func iniciarLectura() {
	go func() {
		for { // Bucle exterior: maneja la conexión/reconexión del puerto serial
			mutexConfig.Lock()
			conf := configActual
			mutexConfig.Unlock()

			if conf.ModoPrueba {
				log.Println("🧪 Modo prueba activado")
				for _, peso := range generarPesosSimulados() {
					broadcast <- fmt.Sprintf("%.2f", peso)
					time.Sleep(300 * time.Millisecond)
				}
				time.Sleep(reintentoDelay) // Esperar un poco antes de volver a simular
				continue
			}

			modo := &serial.Mode{BaudRate: 9600}

			mutexSerial.Lock()
			port, err := serial.Open(conf.Puerto, modo)
			if err != nil {
				mutexSerial.Unlock()
				log.Printf("❌ No se pudo abrir el puerto serial %s: %v. Reintentando en %s...", conf.Puerto, err, reintentoDelay)
				time.Sleep(reintentoDelay)
				continue // Vuelve al inicio del bucle exterior para reintentar
			}
			serialPort = port // Asigna el puerto abierto
			mutexSerial.Unlock()

			err = serialPort.SetReadTimeout(serialReadTimeout)
			if err != nil {
				log.Printf("⚠️ Error al configurar timeout de lectura: %v. Cerrando y reintentando...", err)
				mutexSerial.Lock()
				serialPort.Close()
				serialPort = nil // Marca el puerto como cerrado
				mutexSerial.Unlock()
				time.Sleep(reintentoDelay)
				continue // Vuelve al inicio del bucle exterior
			}
			log.Println("✅ Conectado al puerto serial:", conf.Puerto)

			for { // Bucle interior: lectura continua mientras el puerto está abierto
				mutexConfig.Lock()
				conf = configActual // Obtener la config actual para verificar cambios
				mutexConfig.Unlock()

				mutexSerial.Lock()
				if serialPort == nil { // Si el puerto fue cerrado por otra goroutine (cambio de config, etc.)
					mutexSerial.Unlock() // ¡Importante liberar antes de break!
					log.Println(" Puerto serial cerrado, saliendo del bucle de lectura.")
					break // Sale del bucle interior, permitiendo que el bucle exterior maneje la reconexión
				}

				cmd, ok := comandosPorMarca[strings.ToLower(conf.Marca)]
				if !ok {
					cmd = "P" // Comando por defecto
				}

				_, err := serialPort.Write([]byte(cmd))
				if err != nil {
					log.Printf("⚠️ Error al escribir en el puerto: %v. Cerrando y reintentando...", err)
					serialPort.Close()
					serialPort = nil
					mutexSerial.Unlock()
					time.Sleep(reintentoDelay)
					break // Sale del bucle interior
				}

				time.Sleep(500 * time.Millisecond) // Espera para que el dispositivo responda

				buf := make([]byte, 20)
				n, err := serialPort.Read(buf)
				mutexSerial.Unlock() // Libera el candado después de Read

				if err != nil {
					if err == io.EOF {
						log.Printf("⚠️ EOF recibido al leer del puerto %s. Posible desconexión. Cerrando y reintentando...", conf.Puerto)
					} else if strings.Contains(err.Error(), "timeout") {
						log.Printf("⏱️ Timeout de lectura en puerto %s. No se recibieron datos en %s. Reintentando lectura...", conf.Puerto, serialReadTimeout)
						continue // Solo timeout, reintentar la lectura en el siguiente ciclo
					} else {
						log.Printf("⚠️ Error de lectura en puerto %s: %v. Cerrando y reintentando...", conf.Puerto, err)
						mutexSerial.Lock()
						if serialPort != nil {
							serialPort.Close()
							serialPort = nil
						}
						mutexSerial.Unlock()
						time.Sleep(reintentoDelay)
						break // Sale del bucle interior
					}
					continue // Continúa en el bucle interior si el error fue solo timeout/EOF
				}

				peso := strings.TrimSpace(string(buf[:n]))
				if peso != "" {
					log.Println("📥 Peso enviado:", peso)
					broadcast <- peso
				} else {
					log.Println("⚠️ No se recibió peso significativo.")
				}

				time.Sleep(300 * time.Millisecond) // Espera antes de la siguiente operación
			}
			log.Printf("🔌 Esperando %s antes de intentar reconectar al puerto serial...", reintentoDelay)
			time.Sleep(reintentoDelay)
		}
	}()
}

// iniciarBroadcaster permanece igual
func iniciarBroadcaster() {
	go func() {
		for peso := range broadcast {
			clientesMutex.Lock()
			for c := range clientes {
				// Lanzar envío en goroutine separada para no bloquear a otros clientes
				go func(conn *websocket.Conn, data string) {
					ctx, cancel := context.WithTimeout(context.Background(), time.Second) // Timeout para el envío
					defer cancel()
					err := wsjson.Write(ctx, conn, data)
					if err != nil {
						// Si hay error al enviar (probablemente cliente desconectado)
						log.Printf("⚠️ Error al enviar a cliente: %v", err)
						// Remover el cliente de forma segura
						clientesMutex.Lock()
						delete(clientes, conn)
						clientesMutex.Unlock()
						// Cerrar la conexión por si acaso no lo estaba
						conn.Close(websocket.StatusInternalError, "Error de envío")
					}
				}(c, peso) // Pasar c y peso como argumentos para evitar problemas de clausura
			}
			clientesMutex.Unlock()
		}
	}()
}

// --- NUEVA FUNCIÓN PARA ESCUCHAR MENSAJES DE CONFIG DE UN CLIENTE ESPECÍFICO ---
func listenForConfig(ctx context.Context, c *websocket.Conn) {
	log.Println("👂 Iniciando escucha de mensajes del cliente...")
	// Este defer se ejecutará cuando la goroutine termine (por break en el loop)
	defer log.Println("👂 Terminando escucha de mensajes del cliente.")

	for {
		var mensaje map[string]interface{}
		// Usamos el contexto de la conexión. Read se bloqueará aquí hasta recibir un mensaje o que el ctx se cancele
		err := wsjson.Read(ctx, c, &mensaje)

		// Manejar errores de lectura (incluyendo desconexión del cliente)
		if err != nil {
			// Si el error es por cancelación del contexto o un cierre normal, no loggear como error grave
			if err == context.Canceled {
				log.Println("👂 Contexto del cliente cancelado, terminando escucha de mensajes.")
			} else if websocket.CloseStatus(err) == websocket.StatusNormalClosure || err == io.EOF {
				// Cliente cerró la conexión de forma limpia
				log.Println("👂 Cliente cerró la conexión WebSocket normalmente.")
			} else {
				// Otros errores de lectura (conexión rota, datos inválidos, etc.)
				log.Printf("⚠️ Error de lectura de mensaje del cliente: %v", err)
			}
			break // Salir del bucle de lectura en caso de error
		}

		// --- Procesar el mensaje recibido ---
		// Verificamos si es un mensaje de configuración
		if tipo, ok := mensaje["tipo"]; ok && tipo == "config" {
			data, _ := json.Marshal(mensaje) // Convertimos el mapa de vuelta a JSON para Unmarshal
			var nuevaConfig Configuracion
			// Intentamos decodificar en la estructura Configuracion. Ignoramos el error por simplicidad
			_ = json.Unmarshal(data, &nuevaConfig)

			// Combinar con la configuración actual para no perder valores si el cliente no los envía
			mutexConfig.Lock() // Bloquear para acceder a configActual
			actual := configActual
			// Si el cliente envió un puerto, usarlo; si no, mantener el actual
			if nuevaConfig.Puerto == "" {
				nuevaConfig.Puerto = actual.Puerto
			}
			// Si el cliente envió una marca, usarla; si no, mantener la actual
			if nuevaConfig.Marca == "" {
				nuevaConfig.Marca = actual.Marca
			}
			// ModoPrueba es bool, su valor por defecto (false) es válido.
			// Si el cliente envía true o false, se actualizará. Si no lo envía, se mantiene el valor actual.
			// nuevaConfig.ModoPrueba se mantendrá como false si no viene en el JSON o es inválido,
			// lo cual está bien si queremos que el valor por defecto sea false al no enviarlo.

			mutexConfig.Unlock() // Liberar el candado

			log.Printf("⚙️ Configuración recibida de cliente: %+v\n", nuevaConfig)

			// Verificar si la nueva configuración es diferente a la actual global
			mutexConfig.Lock() // Bloquear de nuevo para comparar con la config global actual
			mismoPuerto := nuevaConfig.Puerto == configActual.Puerto
			mismaMarca := nuevaConfig.Marca == configActual.Marca
			mismoModo := nuevaConfig.ModoPrueba == configActual.ModoPrueba // Comparar con configActual directamente
			mutexConfig.Unlock()                                           // Liberar candado después de leer configActual

			if !mismoPuerto || !mismaMarca || !mismoModo {
				log.Println("🔄 Cambiando configuración y reiniciando lector por solicitud del cliente")
				// La configuración ha cambiado. Necesitamos cerrar el puerto serial actual
				// para que iniciarLectura pueda reabrirlo con la nueva configuración.
				mutexSerial.Lock() // Bloquear para acceder a serialPort
				if serialPort != nil {
					log.Println("Cerrando puerto serial actual debido a cambio de config...")
					serialPort.Close() // Cerrar la conexión serial
					serialPort = nil   // Marcar la variable global como nil
				}
				mutexSerial.Unlock() // Liberar candado serial

				// Actualizar la configuración global
				mutexConfig.Lock()         // Bloquear para actualizar configActual
				configActual = nuevaConfig // Actualizar la configuración global
				mutexConfig.Unlock()       // Liberar candado de configuración
				log.Println("Configuración global actualizada.")

			} else {
				log.Println("⚙️ Configuración recibida es igual a la actual, no se reinicia el lector.")
			}

		} else {
			// Opcional: loggear o ignorar otros tipos de mensajes recibidos
			// log.Printf("📡 Mensaje no 'config' recibido del cliente (ignorado): %+v", mensaje)
			// Si no es un mensaje de configuración, simplemente lo ignoramos y seguimos leyendo.
		}
		// --- Fin Procesamiento de mensaje ---
	}
	// La goroutine termina cuando el bucle `for` se rompe (por error de lectura)
}

// manejarCliente modificado para no esperar config inicial
func manejarCliente(w http.ResponseWriter, r *http.Request) {
	// Aceptar la conexión WebSocket
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true,          // Considerar remover en producción
		OriginPatterns:     []string{"*"}, // Considerar restringir en producción
	})
	if err != nil {
		log.Println("❌ Error al aceptar cliente:", err)
		return
	}
	// Asegurar que la conexión se cierre al salir de la función
	defer c.Close(websocket.StatusInternalError, "cerrando")

	// Obtener el contexto de la solicitud
	ctx := r.Context()

	// Añadir el cliente al mapa de clientes conectados para recibir broadcasts
	clientesMutex.Lock()
	clientes[c] = true
	clientesMutex.Unlock()

	log.Println("🔌 Cliente conectado")

	// --- NUEVA LÓGICA ---
	// En lugar de esperar la config inicial, iniciamos inmediatamente una goroutine
	// para escuchar mensajes de configuración (o cualquier otro) de este cliente en segundo plano.
	go listenForConfig(ctx, c) // Llamar a la nueva función para escuchar mensajes

	// La función principal manejarCliente ahora simplemente espera a que la conexión WebSocket se cierre.
	// Cuando el cliente se desconecte (o haya un error fatal), el contexto 'ctx' se cancelará,
	// desbloqueando esta línea y permitiendo que el resto del 'defer' se ejecute.
	<-ctx.Done()

	// --- Lógica de limpieza al desconectar el cliente ---
	// Remover el cliente del mapa de clientes conectados
	clientesMutex.Lock()
	delete(clientes, c)
	clientesMutex.Unlock()
	// c.Close(...) ya se maneja con el defer
	log.Println("🔌 Cliente desconectado")
}

// Init es llamada cuando el servicio se inicia
func (p *Program) Init(env svc.Environment) error {
	// archivo de logs
	logFile := filepath.Join(os.Getenv("PROGRAMDATA"), "BasculaServicio", "service.log")
	err := os.MkdirAll(filepath.Dir(logFile), 0755)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return err
	}
	log.SetOutput(f)
	log.Printf("Iniciando Servicio...")
	return nil
}

func main() {
	// Register HTTP handler
	http.HandleFunc("/", manejarCliente)

	// Create and run the service
	prg := &Program{}
	if err := svc.Run(prg, syscall.SIGINT, syscall.SIGTERM); err != nil {
		log.Fatal(err)
	}
}
