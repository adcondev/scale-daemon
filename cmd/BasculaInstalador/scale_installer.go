package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	embedded "github.com/adcondev/scale-daemon"
)

// Variable de build - via ldflags
var (
	BuildEnvironment = "prod"
	BuildDate        = "unknown"
	BuildTime        = "unknown"
)

// Serializar binario embebido según entorno
func getEmbeddedService() []byte {
	return embedded.BasculaServicio
}

const (
	serviceName        = "BasculaServicio"
	serviceNameTest    = "BasculaServicioTest"
	serviceDisplayName = "Servicio de Bascula"
	serviceDescription = "Servicio WebSocket y Serial para bascula"
)

// Obtener nombre de servicio segun entorno
func getServiceName() string {
	if BuildEnvironment == "test" {
		return serviceNameTest
	}
	return serviceName
}

func getServiceDisplayName() string {
	if BuildEnvironment == "test" {
		return serviceDisplayName + "TEST"
	}
	return serviceDisplayName + "PROD"
}

// Environment colors
func getEnvironmentColors() (primary, secondary lipgloss.Color) {
	if BuildEnvironment == "test" {
		// TEST: Verde y Amarillo
		return "#00FF40", "#FFD700"
	}
	// PROD: Rojo y Azul
	return "#FF0040", "#0080FF"
}

func getBanner() string {
	envLabel := "PROD"
	if BuildEnvironment == "test" {
		envLabel = "TEST"
	}

	return fmt.Sprintf(`
╔═════════════════════════════════════════════╗
║             SCALE DAEMON v1.1.0             ║
║                                             ║
║     ____     /                _             ║
║    | __ )  __ _ ___  ___ _  _| | __ _       ║
║    |  _ \ / _' / __|/ __| || | |/ _' |      ║
║    | |_) | (_| \__ \ (__| || | | (_| |      ║
║    |____/ \__,_|___/\___|\_,_|_|\__,_|      ║
║                                             ║
║           Instalador de Servicio            ║
║           (C) 2025 Red2000 - %s           ║
╚═════════════════════════════════════════════╝`,
		envLabel,
	)
}

// Estilos con más colores y variedad
var (
	// Colores principales - esquema rojo/azul
	primaryColor, secondaryColor = getEnvironmentColors()
	darkColor                    = lipgloss.Color("#1a1b26") // Fondo oscuro
	lightColor                   = lipgloss.Color("#c0caf5") // Texto claro
	warningColor                 = lipgloss.Color("#ff9e64") // Naranja
	errorColor                   = lipgloss.Color("#f7768e") // Rojo error
	successColor                 = lipgloss.Color("#9ece6a") // Verde éxito
	infoColor                    = lipgloss.Color("#7aa2f7") // Azul info

	// Banner ASCII
	banner = getBanner()

	// Estilos mejorados
	bannerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			Background(darkColor).
			Padding(0, 1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(primaryColor).
			Bold(true).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().
			Foreground(lightColor)

	disabledStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#565f89")).
			Faint(true)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(infoColor)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)

	statusBarStyle = lipgloss.NewStyle().
			Background(darkColor).
			Foreground(lightColor).
			Padding(0, 1)

	// Estilo para el visor de logs
	logViewerStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(infoColor).
			Padding(0, 1)
)

// Implementación de item para la lista
type menuItem struct {
	title       string
	description string
	icon        string
	action      func() tea.Cmd
	enabled     func(string) bool
}

func (i menuItem) Title() string       { return i.icon + " " + i.title }
func (i menuItem) Description() string { return i.description }
func (i menuItem) FilterValue() string { return i.title }

// KeyMap personalizado para navegación
type keyMap struct {
	Up        key.Binding
	Down      key.Binding
	Enter     key.Binding
	Help      key.Binding
	Quit      key.Binding
	Restart   key.Binding
	ToggleLog key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Enter, k.Restart},
		{k.ToggleLog, k.Help},
		{k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "arriba"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "abajo"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "seleccionar"),
	),
	Restart: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "reiniciar servicio"),
	),
	ToggleLog: key.NewBinding(
		key.WithKeys("l"),
		key.WithHelp("l", "ver logs"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "ayuda"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "salir"),
	),
}

type screen int

const (
	menuScreen screen = iota
	processingScreen
	resultScreen
	confirmScreen
	logsMenuScreen // Submenú de logs
	logsViewScreen // Visor de logs en vivo
)

// Tipos de mensajes para logs
type logLinesMsg []string
type logStatusMsg struct {
	Verbose bool
	Size    int64
}

type model struct {
	screen          screen
	list            list.Model
	menuItems       []menuItem
	spinner         spinner.Model
	progress        progress.Model
	help            help.Model
	keys            keyMap
	processing      bool
	result          string
	success         bool
	serviceState    string
	width           int
	height          int
	confirmAction   string
	confirmCallback tea.Cmd
	progressPercent float64
	statusMessage   string
	showHelp        bool
	animationFrame  int
	ready           bool

	// Campos para gestión de logs
	viewport       viewport.Model
	logLines       []string
	logVerbose     bool
	logSize        int64
	wsConn         *websocket.Conn
	logsConnected  bool
	previousScreen screen     // Para volver al menú correcto
	mainMenuItems  []menuItem // Guardar items del menú principal
}

// Items del submenú de logs
func getLogsMenuItems() []menuItem {
	return []menuItem{
		{
			title:       "Ver Logs en Vivo",
			description: "Muestra últimos 100 logs, actualiza cada 1s",
			icon:        "[>]",
			action:      nil,
			enabled:     func(s string) bool { return true },
		},
		{
			title:       "Abrir Archivo de Logs",
			description: "Abre el archivo . log en el editor",
			icon:        "[#]",
			action:      nil,
			enabled:     func(s string) bool { return true },
		},
		{
			title:       "Abrir Ubicación",
			description: "Abre la carpeta que contiene los logs",
			icon:        "[D]",
			action:      nil,
			enabled:     func(s string) bool { return true },
		},
		{
			title:       "Limpiar Logs (Flush)",
			description: "Reduce el archivo manteniendo últimas 50 líneas",
			icon:        "[~]",
			action:      nil,
			enabled:     func(s string) bool { return true },
		},
		{
			title:       "Toggle Logs Detallados",
			description: "Habilita/deshabilita logs no críticos",
			icon:        "[*]",
			action:      nil,
			enabled:     func(s string) bool { return true },
		},
		{
			title:       "Volver",
			description: "Regresar al menú principal",
			icon:        "[<]",
			action:      nil,
			enabled:     func(s string) bool { return true },
		},
	}
}

func initialModel() model {
	// Configurar spinner con animación más llamativa
	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	// Configurar barra de progreso con gradiente personalizado
	p := progress.New(
		progress.WithScaledGradient("#CC0033", "#33A0FF"),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	// Configurar ayuda
	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(secondaryColor)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lightColor)
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(primaryColor)
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(lightColor)

	// Crear items del menú con iconos ASCII
	items := []menuItem{
		{
			title:       "Instalar Servicio",
			description: "Instala y configura el servicio de bascula",
			icon:        "[+]",
			action:      installServiceCmd,
			enabled: func(state string) bool {
				return strings.Contains(state, "NOT INSTALLED")
			},
		},
		{
			title:       "Desinstalar Servicio",
			description: "Elimina completamente el servicio del sistema",
			icon:        "[-]",
			action:      uninstallServiceCmd,
			enabled: func(state string) bool {
				return !strings.Contains(state, "NOT INSTALLED")
			},
		},
		{
			title:       "Iniciar Servicio",
			description: "Inicia el servicio de bascula",
			icon:        "[>]",
			action:      startServiceCmd,
			enabled: func(state string) bool {
				return strings.Contains(state, "STOPPED")
			},
		},
		{
			title:       "Detener Servicio",
			description: "Detiene el servicio en ejecucion",
			icon:        "[.]",
			action:      stopServiceCmd,
			enabled: func(state string) bool {
				return strings.Contains(state, "RUNNING")
			},
		},
		{
			title:       "Reiniciar Servicio",
			description: "Detiene e inicia nuevamente el servicio",
			icon:        "[*]",
			action:      restartServiceCmd,
			enabled: func(state string) bool {
				return strings.Contains(state, "RUNNING") || strings.Contains(state, "STOPPED")
			},
		},
		{
			title:       "Ver Estado",
			description: "Muestra el estado actual del servicio",
			icon:        "[i]",
			action:      nil,
			enabled:     func(state string) bool { return true },
		},
		{
			title:       "Gestionar Logs",
			description: "Submenú de opciones de logs",
			icon:        "[#]",
			action:      nil,
			enabled:     func(state string) bool { return true },
		},
		{
			title:       "Salir",
			description: "Cierra el instalador",
			icon:        "[X]",
			action:      nil,
			enabled:     func(state string) bool { return true },
		},
	}

	// Convertir a list.Item
	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	// Crear delegado personalizado para la lista
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedStyle
	delegate.Styles.SelectedDesc = selectedStyle.Faint(true)
	delegate.Styles.NormalTitle = normalStyle
	delegate.Styles.NormalDesc = normalStyle.Faint(true)
	delegate.Styles.DimmedTitle = disabledStyle
	delegate.Styles.DimmedDesc = disabledStyle.Faint(true)

	// Crear lista con tamaño inicial por defecto
	l := list.New(listItems, delegate, 80, 20)
	l.Title = "Menu Principal"
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	// Inicializar viewport para logs
	vp := viewport.New(80, 20)
	vp.Style = lipgloss.NewStyle().Foreground(lightColor)

	return model{
		screen:        menuScreen,
		list:          l,
		menuItems:     items,
		mainMenuItems: items, // Guardar referencia
		spinner:       s,
		progress:      p,
		help:          h,
		keys:          keys,
		serviceState:  checkServiceStatus(),
		ready:         false,
		viewport:      vp,
		logLines:      []string{},
		logVerbose:    BuildEnvironment == "test", // Default según ambiente
	}
}

// Comandos

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		checkServiceStatusCmd(),
		animationCmd(),
	)
}

// Comando para actualizar el estado del servicio
func checkServiceStatusCmd() tea.Cmd {
	return tea.Every(2*time.Second, func(t time.Time) tea.Msg {
		return serviceCheckMsg(checkServiceStatus())
	})
}

// Comando para animar elementos
func animationCmd() tea.Cmd {
	return tea.Every(100*time.Millisecond, func(t time.Time) tea.Msg {
		return animationMsg{}
	})
}

// Comando de progreso simulado
func simulateProgress() tea.Cmd {
	return tea.Every(100*time.Millisecond, func(t time.Time) tea.Msg {
		return progressMsg(0.1)
	})
}

// Comando para polling de logs - lee directamente del archivo
func tailLogsCmd() tea.Cmd {
	return tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		lines := readLogsFromFile(100)
		return logLinesMsg(lines)
	})
}

// Conectar al servicio via WebSocket
func connectToService() (*websocket.Conn, error) {
	addr := "ws://localhost:8765"
	if BuildEnvironment == "prod" {
		addr = "ws://127.0.0.1:8765"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, addr, nil)
	return conn, err
}

// Enviar comando de logs al servicio
func (m model) sendLogCommand(tipo string, args ...interface{}) (model, tea.Cmd) {
	conn := m.wsConn
	closeAfter := false

	if conn == nil {
		var err error
		conn, err = connectToService()
		if err != nil {
			m.screen = resultScreen
			m.success = false
			m.result = "[X] Servicio no disponible"
			return m, nil
		}
		closeAfter = true

		// Consumir mensaje inicial de ambiente para conexiones nuevas
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		_, _ = readLogResponse(ctx, conn)
		cancel()
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	msg := map[string]interface{}{"tipo": tipo}
	if len(args) > 0 {
		if v, ok := args[0].(bool); ok {
			msg["verbose"] = v
			m.logVerbose = v
		}
	}

	_ = wsjson.Write(ctx, conn, msg)

	if closeAfter {
		conn.Close(websocket.StatusNormalClosure, "")
	}

	m.statusMessage = "[OK] Comando enviado"
	return m, nil
}

// Comando para flush de logs
func flushLogsCmd() tea.Cmd {
	return func() tea.Msg {
		conn, err := connectToService()
		if err != nil {
			return operationDoneMsg{false, "[X] No se pudo conectar al servicio"}
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		// Consumir mensaje inicial de ambiente
		_, _ = readLogResponse(ctx, conn)

		// Enviar comando flush
		err = wsjson.Write(ctx, conn, map[string]string{"tipo": "logFlush"})
		if err != nil {
			return operationDoneMsg{false, "[X] Error enviando comando"}
		}

		// Esperar respuesta (descartando pesos)
		response, err := readLogResponse(ctx, conn)
		if err != nil {
			return operationDoneMsg{false, "[X] Error leyendo respuesta"}
		}

		if ok, exists := response["ok"].(bool); exists && ok {
			return operationDoneMsg{true, "[OK] Logs limpiados correctamente"}
		}

		if errMsg, exists := response["error"].(string); exists {
			return operationDoneMsg{false, fmt.Sprintf("[X] Error: %s", errMsg)}
		}

		return operationDoneMsg{true, "[OK] Logs limpiados"}
	}
}

// Tipos de mensajes
type serviceCheckMsg string
type progressMsg float64
type animationMsg struct{}
type operationDoneMsg struct {
	success bool
	message string
}

// Update principal
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Calcular altura disponible para la lista
		headerHeight := 13
		footerHeight := 3
		if m.showHelp {
			footerHeight = 6
		}

		listHeight := m.height - headerHeight - footerHeight
		if listHeight < 8 {
			listHeight = 8
		}

		// Ajustar tamaños de componentes
		m.list.SetSize(m.width-4, listHeight)
		m.progress.Width = m.width - 10
		if m.progress.Width < 20 {
			m.progress.Width = 20
		}
		m.help.Width = m.width

		// Actualizar viewport para logs
		m.viewport.Width = m.width - 4
		m.viewport.Height = m.height - 8
		if m.viewport.Height < 10 {
			m.viewport.Height = 10
		}

		return m, nil

	case serviceCheckMsg:
		m.serviceState = string(msg)
		for i, item := range m.menuItems {
			if !item.enabled(m.serviceState) {
				m.list.SetItem(i, item)
			}
		}
		return m, nil

	case animationMsg:
		m.animationFrame++
		return m, animationCmd()

	case progressMsg:
		if m.processing {
			m.progressPercent += float64(msg)
			if m.progressPercent >= 1.0 {
				m.progressPercent = 1.0
			}
			return m, simulateProgress()
		}

	case operationDoneMsg:
		m.processing = false
		m.result = msg.message
		m.success = msg.success
		m.screen = resultScreen
		m.progressPercent = 0
		return m, checkServiceStatusCmd()

	// Manejar mensajes de logs
	case logLinesMsg:
		if msg == nil || len(msg) == 0 {
			// Continuar polling
			if m.screen == logsViewScreen {
				return m, tailLogsCmd()
			}
			return m, nil
		}
		m.logLines = []string(msg)
		m.logSize = getLogFileInfo()
		m.viewport.SetContent(strings.Join(m.logLines, "\n"))
		m.viewport.GotoBottom()
		if m.screen == logsViewScreen {
			return m, tailLogsCmd()
		}
		return m, nil

	case logStatusMsg:
		m.logVerbose = msg.Verbose
		m.logSize = msg.Size
		return m, nil

	case tea.KeyMsg:
		switch m.screen {
		case menuScreen:
			switch {
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			case key.Matches(msg, m.keys.Help):
				m.showHelp = !m.showHelp
				m.help.ShowAll = !m.help.ShowAll
				if m.ready {
					headerHeight := 13
					footerHeight := 3
					if m.showHelp {
						footerHeight = 6
					}
					listHeight := m.height - headerHeight - footerHeight
					if listHeight < 8 {
						listHeight = 8
					}
					m.list.SetSize(m.width-4, listHeight)
				}
				return m, nil
			case key.Matches(msg, m.keys.Restart):
				if strings.Contains(m.serviceState, "RUNNING") {
					m.screen = processingScreen
					m.processing = true
					return m, tea.Batch(
						m.spinner.Tick,
						restartServiceCmd(),
						simulateProgress(),
					)
				}
			case key.Matches(msg, m.keys.ToggleLog):
				// Ir al submenú de logs
				return m.goToLogsMenu()
			case key.Matches(msg, m.keys.Enter):
				return m.handleMenuSelection()
			}

			newListModel, cmd := m.list.Update(msg)
			m.list = newListModel
			cmds = append(cmds, cmd)

		// Manejar submenú de logs
		case logsMenuScreen:
			switch {
			case key.Matches(msg, m.keys.Quit), msg.String() == "esc":
				return m.goToMainMenu()
			case key.Matches(msg, m.keys.Enter):
				return m.handleLogsMenuSelection()
			}

			newListModel, cmd := m.list.Update(msg)
			m.list = newListModel
			cmds = append(cmds, cmd)

		// Manejar visor de logs
		case logsViewScreen:
			switch msg.String() {
			case "esc", "q":
				// Simplemente volver al menú de logs
				m.logsConnected = false
				return m.goToLogsMenu()
			case "r": // Refresh manual
				lines := readLogsFromFile(100)
				m.logLines = lines
				m.logSize = getLogFileInfo()
				m.viewport.SetContent(strings.Join(lines, "\n"))
				m.viewport.GotoBottom()
				m.statusMessage = "[OK] Logs actualizados"
				return m, tailLogsCmd()
			}

			// Scroll del viewport
			newVP, cmd := m.viewport.Update(msg)
			m.viewport = newVP
			cmds = append(cmds, cmd)

		case confirmScreen:
			switch msg.String() {
			case "s", "S":
				m.screen = processingScreen
				m.processing = true
				return m, tea.Batch(
					m.spinner.Tick,
					m.confirmCallback,
					simulateProgress(),
				)
			case "n", "N", "esc":
				m.screen = menuScreen
				return m, checkServiceStatusCmd()
			}

		case resultScreen:
			if msg.String() == "enter" || msg.String() == "esc" {
				// Volver al menú apropiado
				if m.previousScreen == logsMenuScreen {
					return m.goToLogsMenu()
				}
				m.screen = menuScreen
				m.result = ""
				return m, checkServiceStatusCmd()
			}

		case processingScreen:
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
		}

	case spinner.TickMsg:
		if m.processing {
			newSpinner, cmd := m.spinner.Update(msg)
			m.spinner = newSpinner
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

// Ir al submenú de logs
func (m model) goToLogsMenu() (model, tea.Cmd) {
	logsItems := getLogsMenuItems()
	listItems := make([]list.Item, len(logsItems))
	for i, item := range logsItems {
		listItems[i] = item
	}

	m.list.SetItems(listItems)
	m.list.Title = "Gestión de Logs"
	m.menuItems = logsItems
	m.screen = logsMenuScreen
	m.previousScreen = logsMenuScreen

	// Intentar obtener estado de logs
	go func() {
		if conn, err := connectToService(); err == nil {
			defer conn.Close(websocket.StatusNormalClosure, "")
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			_ = wsjson.Write(ctx, conn, map[string]string{"tipo": "logStatus"})
		}
	}()

	return m, nil
}

// Volver al menú principal
func (m model) goToMainMenu() (model, tea.Cmd) {
	listItems := make([]list.Item, len(m.mainMenuItems))
	for i, item := range m.mainMenuItems {
		listItems[i] = item
	}

	m.list.SetItems(listItems)
	m.list.Title = "Menu Principal"
	m.menuItems = m.mainMenuItems
	m.screen = menuScreen
	m.previousScreen = menuScreen

	return m, checkServiceStatusCmd()
}

// Manejar selección del submenú de logs
func (m model) handleLogsMenuSelection() (model, tea.Cmd) {
	selected := m.list.SelectedItem().(menuItem)

	switch selected.title {
	case "Ver Logs en Vivo":
		// Leer logs iniciales del archivo
		lines := readLogsFromFile(100)
		m.logLines = lines
		m.viewport.SetContent(strings.Join(lines, "\n"))
		m.viewport.GotoBottom()
		m.logSize = getLogFileInfo()
		m.screen = logsViewScreen
		m.statusMessage = ""
		return m, tailLogsCmd()

	case "Abrir Archivo de Logs":
		viewLogs()
		m.statusMessage = "Abriendo archivo de logs..."
		return m, nil

	case "Abrir Ubicación":
		logDir := filepath.Join(os.Getenv("PROGRAMDATA"), getServiceName())
		_ = exec.Command("explorer", logDir).Start()
		m.statusMessage = "Abriendo carpeta de logs..."
		return m, nil

	case "Limpiar Logs (Flush)":
		m.confirmAction = "¿Limpiar archivo de logs?\n(se mantienen últimas 50 líneas)"
		m.confirmCallback = flushLogsCmd()
		m.previousScreen = logsMenuScreen
		m.screen = confirmScreen
		return m, nil

	case "Toggle Logs Detallados":
		newModel, cmd := m.sendLogCommand("logConfig", !m.logVerbose)
		newModel.statusMessage = fmt.Sprintf("[OK] Verbose cambiado a: %v", newModel.logVerbose)
		return newModel, cmd

	case "Volver":
		return m.goToMainMenu()
	}

	return m, nil
}

// Manejo de selección del menú
func (m model) handleMenuSelection() (model, tea.Cmd) {
	selected := m.list.SelectedItem().(menuItem)

	if !selected.enabled(m.serviceState) {
		m.screen = resultScreen
		m.success = false
		m.result = fmt.Sprintf("[X] Opcion no disponible: %s", selected.title)
		return m, nil
	}

	switch selected.title {
	case "Salir":
		return m, tea.Quit
	case "Ver Estado":
		m.screen = resultScreen
		m.result = fmt.Sprintf("Estado del servicio: %s", getStatusDisplay(m.serviceState))
		m.success = true
		return m, nil
	case "Gestionar Logs":
		return m.goToLogsMenu()
	case "Instalar Servicio", "Desinstalar Servicio":
		m.confirmAction = fmt.Sprintf("Confirma %s?", selected.title)
		m.confirmCallback = selected.action()
		m.screen = confirmScreen
		return m, nil
	default:
		m.screen = processingScreen
		m.processing = true
		return m, tea.Batch(
			m.spinner.Tick,
			selected.action(),
			simulateProgress(),
		)
	}
}

// Vistas

func (m model) View() string {
	if !m.ready {
		return "Inicializando..."
	}

	switch m.screen {
	case menuScreen:
		return m.viewMenu()
	case processingScreen:
		return m.viewProcessing()
	case resultScreen:
		return m.viewResult()
	case confirmScreen:
		return m.viewConfirm()
	case logsMenuScreen:
		return m.viewLogsMenu()
	case logsViewScreen:
		return m.viewLogsLive()
	default:
		return ""
	}
}

// Update the viewMenu to show environment info
func (m model) viewMenu() string {
	var b strings.Builder

	b.WriteString(bannerStyle.Render(banner) + "\n")

	envIndicator := "[TEST]"
	if BuildEnvironment == "prod" {
		envIndicator = "[PROD]"
	}

	status := getStatusDisplay(m.serviceState)
	statusBar := statusBarStyle.Render(
		fmt.Sprintf("%s Estado: %s | [T] %s",
			envIndicator, status, time.Now().Format("15:04:05")))
	b.WriteString(statusBar + "\n\n")

	b.WriteString(m.list.View())

	if m.showHelp {
		b.WriteString("\n" + m.help.View(m.keys))
	} else {
		helpText := infoStyle.Render("? ayuda")
		quitText := errorStyle.Render("q salir")
		restartText := warningStyle.Render("r reiniciar")
		b.WriteString("\n" + helpText + " • " + restartText + " • " + quitText)
	}

	if m.statusMessage != "" {
		b.WriteString("\n" + infoStyle.Render(m.statusMessage))
	}

	return b.String()
}

// Vista del submenú de logs
func (m model) viewLogsMenu() string {
	var b strings.Builder

	b.WriteString(bannerStyle.Render(banner) + "\n")

	// Status bar con info de logs
	verboseStatus := "OFF"
	if m.logVerbose {
		verboseStatus = "ON"
	}

	statusBar := statusBarStyle.Render(
		fmt.Sprintf("[#] LOGS | Verbose: %s | Tamaño: %s",
			verboseStatus, formatBytes(m.logSize)))
	b.WriteString(statusBar + "\n\n")

	b.WriteString(m.list.View())

	b.WriteString("\n" + infoStyle.Render("[ESC] Volver al menú principal"))

	if m.statusMessage != "" {
		b.WriteString("\n" + successStyle.Render(m.statusMessage))
	}

	return b.String()
}

// Vista del visor de logs en vivo
func (m model) viewLogsLive() string {
	var b strings.Builder

	// Header compacto
	header := lipgloss.NewStyle().
		Background(darkColor).
		Foreground(primaryColor).
		Bold(true).
		Padding(0, 1).
		Width(m.width).
		Render(fmt.Sprintf("[#] LOGS EN VIVO | Tamaño: %s | Líneas: %d",
			formatBytes(m.logSize), len(m.logLines)))

	b.WriteString(header + "\n")

	// Viewport con logs
	viewportStyle := logViewerStyle.Width(m.width - 2).Height(m.viewport.Height)
	b.WriteString(viewportStyle.Render(m.viewport.View()) + "\n")

	// Footer con controles simplificados
	footer := infoStyle.Render("[ESC/Q] Volver") + " • " +
		warningStyle.Render("[R] Refresh") + " • " +
		normalStyle.Render("[↑↓] Scroll") + " • " +
		successStyle.Render("Auto-refresh:  1s")
	b.WriteString(footer)

	if m.statusMessage != "" {
		b.WriteString("\n" + infoStyle.Render(m.statusMessage))
	}

	return b.String()
}

// Formatear bytes a unidades legibles
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func (m model) viewProcessing() string {
	var b strings.Builder

	b.WriteString(bannerStyle.Render(banner) + "\n\n")

	spinnerView := lipgloss.JoinHorizontal(
		lipgloss.Left,
		m.spinner.View(),
		" Procesando operacion...",
	)
	b.WriteString(spinnerView + "\n\n")

	if m.progressPercent > 0 {
		b.WriteString(m.progress.ViewAs(m.progressPercent))
		b.WriteString(fmt.Sprintf("\n%.0f%% completado", m.progressPercent*100))
	}

	pulseStyle := lipgloss.NewStyle().Foreground(secondaryColor)
	if m.animationFrame%10 < 5 {
		pulseStyle = pulseStyle.Bold(true)
	}
	b.WriteString("\n\n" + pulseStyle.Render("[~] Por favor espere..."))

	return b.String()
}

func (m model) viewConfirm() string {
	var b strings.Builder

	b.WriteString(bannerStyle.Render(banner) + "\n\n")

	confirmBox := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(warningColor).
		Padding(1, 2).
		Width(m.width - 10).
		Align(lipgloss.Center)

	content := fmt.Sprintf("[!] CONFIRMACION\n\n%s\n\n", m.confirmAction)
	content += successStyle.Render("[S]i") + "    " + errorStyle.Render("[N]o")

	b.WriteString(confirmBox.Render(content))

	return b.String()
}

func (m model) viewResult() string {
	var b strings.Builder

	b.WriteString(bannerStyle.Render(banner) + "\n\n")

	var boxStyle lipgloss.Style
	if m.success {
		boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(successColor).
			Padding(1, 2).
			Width(m.width - 10)
	} else {
		boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(errorColor).
			Padding(1, 2).
			Width(m.width - 10)
	}

	b.WriteString(boxStyle.Render(m.result))
	b.WriteString("\n\n" + infoStyle.Render("Presione Enter para continuar..."))

	return b.String()
}

// Comandos de servicio
func restartServiceCmd() tea.Cmd {
	svcName := getServiceName()
	return func() tea.Msg {
		stopCmd := exec.Command("sc", "stop", svcName)
		_ = stopCmd.Run()
		time.Sleep(2 * time.Second)
		startCmd := exec.Command("sc", "start", svcName)
		if output, err := startCmd.CombinedOutput(); err != nil {
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("Error al reiniciar: %s", string(output)),
			}
		}
		return operationDoneMsg{
			success: true,
			message: "[OK] Servicio reiniciado correctamente",
		}
	}
}

func getStatusDisplay(state string) string {
	if strings.Contains(state, "RUNNING") {
		return "[+] EN EJECUCION"
	} else if strings.Contains(state, "STOPPED") {
		return "[.] DETENIDO"
	} else if strings.Contains(state, "NOT INSTALLED") {
		return "[-] NO INSTALADO"
	}
	return "[?] DESCONOCIDO"
}

// Update service commands to use dynamic service name
func installServiceCmd() tea.Cmd {
	return func() tea.Msg {
		if !isAdmin() {
			return operationDoneMsg{
				success: false,
				message: "[!] Se requieren permisos de administrador.",
			}
		}

		svcName := getServiceName()
		targetDir := filepath.Join(os.Getenv("ProgramFiles"), svcName)
		targetPath := filepath.Join(targetDir, svcName+".exe")

		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("[X] Error al crear directorio: %v", err),
			}
		}

		// Use the appropriate embedded service based on environment
		if err := os.WriteFile(targetPath, getEmbeddedService(), 0755); err != nil {
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("[X] Error al extraer servicio: %v", err),
			}
		}

		cmd := exec.Command("sc", "create", svcName,
			fmt.Sprintf("binPath=%s", targetPath),
			"start=auto",
			fmt.Sprintf("DisplayName=%s", getServiceDisplayName()))

		if output, err := cmd.CombinedOutput(); err != nil {
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("[X] Error al crear servicio: %s", string(output)),
			}
		}

		_ = exec.Command("sc", "description", svcName,
			fmt.Sprintf("%s - Ambiente: %s", serviceDescription, strings.ToUpper(BuildEnvironment))).Run()
		_ = exec.Command("sc", "failure", svcName,
			"reset=86400",
			"actions=restart/5000/restart/5000/restart/5000").Run()
		_ = exec.Command("sc", "start", svcName).Run()

		envMsg := "PRODUCCION (0.0.0.0:8765)"
		if BuildEnvironment == "test" {
			envMsg = "TEST (localhost:8765)"
		}

		return operationDoneMsg{
			success: true,
			message: fmt.Sprintf("[OK] Servicio %s instalado - %s", svcName, envMsg),
		}
	}
}

func uninstallServiceCmd() tea.Cmd {
	svcName := getServiceName()
	return func() tea.Msg {
		if !isAdmin() {
			return operationDoneMsg{
				success: false,
				message: "[!] Se requieren permisos de administrador.",
			}
		}
		_ = exec.Command("sc", "stop", svcName).Run()
		time.Sleep(2 * time.Second)
		cmd := exec.Command("sc", "delete", svcName)
		if output, err := cmd.CombinedOutput(); err != nil {
			if strings.Contains(string(output), "1060") {
				return operationDoneMsg{
					success: false,
					message: "[-] El servicio no esta instalado",
				}
			}
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("[X] Error al eliminar servicio: %s", string(output)),
			}
		}
		targetDir := filepath.Join(os.Getenv("ProgramFiles"), svcName)
		_ = os.RemoveAll(targetDir)
		return operationDoneMsg{
			success: true,
			message: "[OK] Servicio desinstalado completamente",
		}
	}
}

func startServiceCmd() tea.Cmd {
	svcName := getServiceName()
	return func() tea.Msg {
		cmd := exec.Command("sc", "start", svcName)
		if output, err := cmd.CombinedOutput(); err != nil {
			outputStr := string(output)
			if strings.Contains(outputStr, "1056") {
				return operationDoneMsg{
					success: false,
					message: "[!] El servicio ya esta en ejecucion",
				}
			} else if strings.Contains(outputStr, "1060") {
				return operationDoneMsg{
					success: false,
					message: "[-] El servicio no esta instalado",
				}
			}
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("[X] Error al iniciar: %s", outputStr),
			}
		}
		return operationDoneMsg{
			success: true,
			message: "[>] Servicio iniciado correctamente",
		}
	}
}

func stopServiceCmd() tea.Cmd {
	svcName := getServiceName()
	return func() tea.Msg {
		cmd := exec.Command("sc", "stop", svcName)
		if output, err := cmd.CombinedOutput(); err != nil {
			outputStr := string(output)
			if strings.Contains(outputStr, "1062") {
				return operationDoneMsg{
					success: false,
					message: "[!] El servicio no esta en ejecucion",
				}
			}
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("[X] Error al detener: %s", outputStr),
			}
		}
		return operationDoneMsg{
			success: true,
			message: "[.] Servicio detenido correctamente",
		}
	}
}

func checkServiceStatus() string {
	cmd := exec.Command("sc", "query", getServiceName())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "NOT INSTALLED"
	}
	outputStr := string(output)
	if strings.Contains(outputStr, "RUNNING") {
		return "RUNNING"
	} else if strings.Contains(outputStr, "STOPPED") {
		return "STOPPED"
	}
	return "UNKNOWN"
}

func isAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

func viewLogs() {
	logPath := filepath.Join(os.Getenv("PROGRAMDATA"), getServiceName(), getServiceName()+".log")
	_ = exec.Command("notepad.exe", logPath).Start()
}

func main() {
	if !isAdmin() {
		fmt.Println(errorStyle.Render("[!] Permisos de Administrador Requeridos"))
		fmt.Println(infoStyle.Render("\n[i] Instrucciones:"))
		fmt.Println("1. Cierre esta ventana")
		fmt.Println("2. Clic derecho -> Ejecutar como administrador")
		fmt.Println("\nPresione Enter para salir...")
		_, _ = fmt.Scanln()
		os.Exit(1)
	}

	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}

// readLogResponse lee mensajes del WebSocket hasta encontrar una respuesta de logs válida.
// Descarta mensajes de peso (strings simples) que llegan del broadcaster.
func readLogResponse(ctx context.Context, conn *websocket.Conn) (map[string]interface{}, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		_, data, err := conn.Read(ctx)
		if err != nil {
			return nil, err
		}

		// Intentar parsear como JSON
		var response map[string]interface{}
		if err := json.Unmarshal(data, &response); err != nil {
			// No es JSON válido (probablemente un peso como "0.000 kg")
			// Continuar leyendo hasta obtener respuesta de logs
			continue
		}

		// Verificar que es una respuesta de logs (no mensaje de ambiente inicial)
		if tipo, ok := response["tipo"].(string); ok {
			switch tipo {
			case "logLines", "logStatus", "logFlushResult":
				return response, nil
			case "ambiente":
				// Descartar mensaje inicial de ambiente, seguir leyendo
				continue
			}
		}

		// Respuesta JSON pero no es de logs, continuar
		continue
	}
}

// readLogsFromFile lee las últimas N líneas del archivo de logs directamente
func readLogsFromFile(n int) []string {
	logPath := filepath.Join(os.Getenv("PROGRAMDATA"), getServiceName(), getServiceName()+".log")

	file, err := os.Open(logPath)
	if err != nil {
		return []string{fmt.Sprintf("[!] No se pudo abrir archivo: %v", err)}
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return []string{fmt.Sprintf("[!] Error al obtener info del archivo: %v", err)}
	}

	size := stat.Size()
	if size == 0 {
		return []string{"[i] Archivo de logs vacío"}
	}

	// Leer últimos 64KB máximo (suficiente para ~1000 líneas)
	bufSize := int64(64 * 1024)
	if size < bufSize {
		bufSize = size
	}

	buf := make([]byte, bufSize)
	_, err = file.Seek(size-bufSize, 0)
	if err != nil {
		return []string{fmt.Sprintf("[!] Error seek: %v", err)}
	}

	_, err = file.Read(buf)
	if err != nil {
		return []string{fmt.Sprintf("[!] Error leyendo:  %v", err)}
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

// getLogFileInfo obtiene tamaño del archivo de logs
func getLogFileInfo() int64 {
	logPath := filepath.Join(os.Getenv("PROGRAMDATA"), getServiceName(), getServiceName()+".log")
	info, err := os.Stat(logPath)
	if err != nil {
		return 0
	}
	return info.Size()
}
