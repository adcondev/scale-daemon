package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	embedded "github.com/adcondev/scale-daemon"
)

// Variables injected by Taskfile (ldflags)
var (
	BuildEnvironment = "remote"
	BuildDate        = "unknown"
	BuildTime        = "unknown"

	// INJECTED NAMES
	ServiceName        = "BasculaServicio"     // Registry Name
	ServiceDisplayName = "Servicio de Bascula" // Services.msc Name
	ServiceExeName     = "BasculaServicio.exe" // Filename on Disk
)

const (
	serviceDescription = "Servicio WebSocket y Serial para bascula"
)

// Serializar binario embebido según entorno
func getEmbeddedService() []byte {
	return embedded.BasculaServicio
}

// Environment colors
func getEnvironmentColors() (primary, secondary lipgloss.Color) {
	if BuildEnvironment == "local" {
		// LOCAL: Verde y Amarillo
		return "#00FF40", "#FFD700"
	}
	// REMOTE: Rojo y Azul
	return "#FF0040", "#0080FF"
}

func getBanner() string {
	envLabel := "REMOTE"
	if BuildEnvironment == "local" {
		envLabel = "LOCAL"
	}

	return fmt.Sprintf(`
╔═════════════════════════════════════════════╗
║             SCALE DAEMON v1.3.0             ║
║                                             ║
║     ____     /                _             ║
║    | __ )  __ _ ___  ___ _  _| | __ _       ║
║    |  _ \ / _' / __|/ __| || | |/ _' |      ║
║    | |_) | (_| \__ \ (__| || | | (_| |      ║
║    |____/ \__,_|___/\___|\_,_|_|\__,_|      ║
║                                             ║
║           Instalador de Servicio            ║
║           (C) 2025 Red2000 - %s         ║
╚═════════════════════════════════════════════╝`,
		envLabel,
	)
}

// Estilos con más colores y variedad
var (
	primaryColor, secondaryColor = getEnvironmentColors()
	darkColor                    = lipgloss.Color("#1a1b26")
	lightColor                   = lipgloss.Color("#c0caf5")
	warningColor                 = lipgloss.Color("#ff9e64")
	errorColor                   = lipgloss.Color("#f7768e")
	successColor                 = lipgloss.Color("#9ece6a")
	infoColor                    = lipgloss.Color("#7aa2f7")

	banner = getBanner()

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
	Up      key.Binding
	Down    key.Binding
	Enter   key.Binding
	Help    key.Binding
	Quit    key.Binding
	Restart key.Binding
}

func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Help, k.Quit}
}

func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Enter, k.Restart},
		{k.Help, k.Quit},
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
	logsMenuScreen
)

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
	previousScreen  screen
	mainMenuItems   []menuItem
}

// Items del submenú de logs (simplified)
func getLogsMenuItems() []menuItem {
	return []menuItem{
		{
			title:       "Abrir Archivo de Logs",
			description: "Abre el archivo .log en el editor",
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
			title:       "Volver",
			description: "Regresar al menú principal",
			icon:        "[<]",
			action:      nil,
			enabled:     func(s string) bool { return true },
		},
	}
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Pulse
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	p := progress.New(
		progress.WithScaledGradient("#CC0033", "#33A0FF"),
		progress.WithWidth(40),
		progress.WithoutPercentage(),
	)

	h := help.New()
	h.Styles.ShortKey = lipgloss.NewStyle().Foreground(secondaryColor)
	h.Styles.ShortDesc = lipgloss.NewStyle().Foreground(lightColor)
	h.Styles.FullKey = lipgloss.NewStyle().Foreground(primaryColor)
	h.Styles.FullDesc = lipgloss.NewStyle().Foreground(lightColor)

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
			description: "Abrir archivo o carpeta de logs",
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

	listItems := make([]list.Item, len(items))
	for i, item := range items {
		listItems[i] = item
	}

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = selectedStyle
	delegate.Styles.SelectedDesc = selectedStyle.Faint(true)
	delegate.Styles.NormalTitle = normalStyle
	delegate.Styles.NormalDesc = normalStyle.Faint(true)
	delegate.Styles.DimmedTitle = disabledStyle
	delegate.Styles.DimmedDesc = disabledStyle.Faint(true)

	l := list.New(listItems, delegate, 80, 20)
	l.Title = "Menu Principal"
	l.Styles.Title = titleStyle
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.SetShowHelp(false)

	return model{
		screen:        menuScreen,
		list:          l,
		menuItems:     items,
		mainMenuItems: items,
		spinner:       s,
		progress:      p,
		help:          h,
		keys:          keys,
		serviceState:  checkServiceStatus(),
		ready:         false,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		checkServiceStatusCmd(),
		animationCmd(),
	)
}

func checkServiceStatusCmd() tea.Cmd {
	return tea.Every(2*time.Second, func(t time.Time) tea.Msg {
		return serviceCheckMsg(checkServiceStatus())
	})
}

func animationCmd() tea.Cmd {
	return tea.Every(100*time.Millisecond, func(t time.Time) tea.Msg {
		return animationMsg{}
	})
}

func simulateProgress() tea.Cmd {
	return tea.Every(100*time.Millisecond, func(t time.Time) tea.Msg {
		return progressMsg(0.1)
	})
}

type serviceCheckMsg string
type progressMsg float64
type animationMsg struct{}
type operationDoneMsg struct {
	success bool
	message string
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

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
		m.progress.Width = m.width - 10
		if m.progress.Width < 20 {
			m.progress.Width = 20
		}
		m.help.Width = m.width

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
			case key.Matches(msg, m.keys.Enter):
				return m.handleMenuSelection()
			}

			newListModel, cmd := m.list.Update(msg)
			m.list = newListModel
			cmds = append(cmds, cmd)

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

	return m, nil
}

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

func (m model) handleLogsMenuSelection() (model, tea.Cmd) {
	selected := m.list.SelectedItem().(menuItem)

	switch selected.title {
	case "Abrir Archivo de Logs":
		viewLogs()
		m.statusMessage = "Abriendo archivo de logs..."
		return m, nil

	case "Abrir Ubicación":
		logDir := filepath.Join(os.Getenv("PROGRAMDATA"), ServiceName)
		_ = exec.Command("explorer", logDir).Start()
		m.statusMessage = "Abriendo carpeta de logs..."
		return m, nil

	case "Volver":
		return m.goToMainMenu()
	}

	return m, nil
}

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
	default:
		return ""
	}
}

func (m model) viewMenu() string {
	var b strings.Builder

	b.WriteString(bannerStyle.Render(banner) + "\n")

	envIndicator := "[REMOTE]"
	if BuildEnvironment == "local" {
		envIndicator = "[LOCAL]"
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

func (m model) viewLogsMenu() string {
	var b strings.Builder

	b.WriteString(bannerStyle.Render(banner) + "\n")

	statusBar := statusBarStyle.Render("[#] GESTIÓN DE LOGS")
	b.WriteString(statusBar + "\n\n")

	b.WriteString(m.list.View())

	b.WriteString("\n" + infoStyle.Render("[ESC] Volver al menú principal"))

	if m.statusMessage != "" {
		b.WriteString("\n" + successStyle.Render(m.statusMessage))
	}

	return b.String()
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

// Service commands
func restartServiceCmd() tea.Cmd {
	svcName := ServiceName
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

func installServiceCmd() tea.Cmd {
	return func() tea.Msg {
		if !isAdmin() {
			return operationDoneMsg{
				success: false,
				message: "[!] Se requieren permisos de administrador.",
			}
		}

		// USE INJECTED VARIABLES
		svcName := ServiceName
		targetDir := filepath.Join(os.Getenv("ProgramFiles"), svcName)
		targetPath := filepath.Join(targetDir, ServiceExeName)

		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("[X] Error al crear directorio: %v", err),
			}
		}

		if err := os.WriteFile(targetPath, getEmbeddedService(), 0755); err != nil {
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("[X] Error al extraer servicio: %v", err),
			}
		}

		cmd := exec.Command("sc", "create", svcName,
			fmt.Sprintf("binPath=%s", targetPath),
			"start=auto",
			fmt.Sprintf("DisplayName=%s", ServiceDisplayName))

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

		envMsg := "REMOTE (0.0.0.0:8765)"
		if BuildEnvironment == "local" {
			envMsg = "LOCAL (localhost:8765)"
		}

		return operationDoneMsg{
			success: true,
			message: fmt.Sprintf("[OK] Servicio %s instalado - %s", svcName, envMsg),
		}
	}
}

func uninstallServiceCmd() tea.Cmd {
	svcName := ServiceName
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
	svcName := ServiceName
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
	svcName := ServiceName
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
	cmd := exec.Command("sc", "query", ServiceName)
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
	logPath := filepath.Join(os.Getenv("PROGRAMDATA"), ServiceName, ServiceName+".log")
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
