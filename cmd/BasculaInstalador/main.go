package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adcondev/daemonize-example"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const (
	serviceName        = "BasculaServicio"
	serviceDisplayName = "Servicio de Báscula"
	serviceDescription = "Servicio WebSocket y Serial para báscula"
)

// Estilos con más colores y variedad
var (
	// Colores principales de la aplicación
	primaryColor   = lipgloss.Color("170") // Verde menta
	secondaryColor = lipgloss.Color("205") // Rosa
	accentColor    = lipgloss.Color("86")  // Cyan
	warningColor   = lipgloss.Color("214") // Naranja
	errorColor     = lipgloss.Color("196") // Rojo
	successColor   = lipgloss.Color("46")  // Verde brillante

	// Banner ASCII art
	banner = `
╔══════════════════════════════════════════╗
║  ____                     _              ║
║ | __ )  __ _ ___  ___ _  _| | __ _       ║
║ |  _ \ / _' / __|/ __| || | |/ _' |      ║
║ | |_) | (_| \__ \ (__| || | | (_| |      ║
║ |____/ \__,_|___/\___|\_,_|_|\__,_|      ║
║                                          ║
║        Instalador de Servicio v1.0       ║
╚══════════════════════════════════════════╝`

	bannerStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	menuStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			MarginTop(1).
			MarginBottom(1)

	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(primaryColor).
			Bold(true).
			Padding(0, 1)

	normalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252"))

	disabledStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Faint(true)

	successStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	warningStyle = lipgloss.NewStyle().
			Foreground(warningColor).
			Bold(true)
)

type screen int

const (
	menuScreen screen = iota
	installingScreen
	resultScreen
	confirmScreen
)

type model struct {
	screen          screen
	menuCursor      int
	menuOptions     []string
	spinner         spinner.Model
	progress        progress.Model
	processing      bool
	result          string
	success         bool
	serviceState    string
	width           int
	height          int
	confirmAction   string
	confirmCallback tea.Cmd
	progressPercent float64
}

func initialModel() model {
	// Configurar spinner animado
	s := spinner.New()
	s.Spinner = spinner.Points
	s.Style = lipgloss.NewStyle().Foreground(primaryColor)

	// Configurar barra de progreso
	p := progress.New(progress.WithDefaultGradient())

	return model{
		screen: menuScreen,
		menuOptions: []string{
			"📦 Instalar Servicio",
			"🗑️ Desinstalar Servicio",
			"▶️ Iniciar Servicio",
			"⏹️ Detener Servicio",
			"📊 Ver Estado",
			"📝 Ver Logs",
			"❌ Salir",
		},
		spinner:      s,
		progress:     p,
		serviceState: checkServiceStatus(),
	}
}

type tickMsg time.Time
type serviceCheckMsg string
type progressMsg float64
type operationDoneMsg struct {
	success bool
	message string
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		checkServiceStatusCmd(),
	)
}

// Comando para actualizar el estado del servicio periódicamente
func checkServiceStatusCmd() tea.Cmd {
	return func() tea.Msg {
		return serviceCheckMsg(checkServiceStatus())
	}
}

// Comando para simular progreso en operaciones largas
func simulateProgress() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(100 * time.Millisecond)
		return progressMsg(0.1)
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Ajustar el ancho de la barra de progreso
		m.progress.Width = msg.Width - 4
		return m, nil

	case serviceCheckMsg:
		m.serviceState = string(msg)
		return m, nil

	case progressMsg:
		// Actualizar el progreso de la instalación
		m.progressPercent += float64(msg)
		if m.progressPercent >= 1.0 {
			m.progressPercent = 1.0
		}
		return m, nil

	case operationDoneMsg:
		m.processing = false
		m.result = msg.message
		m.success = msg.success
		m.screen = resultScreen
		m.progressPercent = 0
		return m, checkServiceStatusCmd()

	case tea.KeyMsg:
		// Si está procesando, solo permitir Ctrl+C para salir
		if m.processing {
			if msg.String() == "ctrl+c" {
				return m, tea.Quit
			}
			return m, nil
		}

		switch m.screen {
		case menuScreen:
			switch msg.String() {
			case "ctrl+c", "q":
				return m, tea.Quit
			case "up", "k":
				if m.menuCursor > 0 {
					m.menuCursor--
				} else {
					// Wrap around - ir al final si estamos al principio
					m.menuCursor = len(m.menuOptions) - 1
				}
			case "down", "j":
				if m.menuCursor < len(m.menuOptions)-1 {
					m.menuCursor++
				} else {
					// Wrap around - ir al principio si estamos al final
					m.menuCursor = 0
				}
			case "enter":
				return m.handleMenuSelection()
			}

		case confirmScreen:
			switch msg.String() {
			case "s", "S", "y", "Y":
				// Confirmar acción
				m.screen = installingScreen
				m.processing = true
				return m, tea.Batch(
					m.spinner.Tick,
					m.confirmCallback,
					simulateProgress(),
				)
			case "n", "N", "esc":
				// Cancelar y volver al menú
				m.screen = menuScreen
				m.confirmAction = ""
				m.confirmCallback = nil
				return m, checkServiceStatusCmd()
			}

		case resultScreen:
			switch msg.String() {
			case "enter", "esc":
				m.screen = menuScreen
				m.result = ""
				return m, checkServiceStatusCmd()
			}
		}

	case spinner.TickMsg:
		if m.processing {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, tea.Batch(cmd, simulateProgress())
		}
	}

	return m, nil
}

func (m model) handleMenuSelection() (model, tea.Cmd) {
	// Verificar si la opción está deshabilitada antes de procesar
	if !m.isOptionEnabled(m.menuCursor) {
		// Mostrar mensaje de error si la opción está deshabilitada
		m.screen = resultScreen
		m.success = false
		m.result = m.getDisabledReason(m.menuCursor)
		return m, nil
	}

	switch m.menuCursor {
	case 0: // Instalar
		m.confirmAction = "¿Está seguro que desea INSTALAR el servicio?"
		m.confirmCallback = installServiceCmd()
		m.screen = confirmScreen
		return m, nil

	case 1: // Desinstalar
		m.confirmAction = "¿Está seguro que desea DESINSTALAR el servicio?\n⚠️ Esto eliminará completamente el servicio del sistema."
		m.confirmCallback = uninstallServiceCmd()
		m.screen = confirmScreen
		return m, nil

	case 2: // Iniciar
		m.screen = installingScreen
		m.processing = true
		return m, tea.Batch(
			m.spinner.Tick,
			startServiceCmd(),
		)

	case 3: // Detener
		m.screen = installingScreen
		m.processing = true
		return m, tea.Batch(
			m.spinner.Tick,
			stopServiceCmd(),
		)

	case 4: // Ver Estado
		m.screen = resultScreen
		m.result = fmt.Sprintf("Estado actual del servicio: %s", m.getStatusDisplay())
		m.success = true
		return m, nil

	case 5: // Ver Logs
		viewLogs()
		m.screen = resultScreen
		m.result = "Abriendo archivo de logs en el bloc de notas..."
		m.success = true
		return m, nil

	case 6: // Salir
		return m, tea.Quit
	}
	return m, nil
}

// Verificar si una opción del menú está habilitada según el estado del servicio
func (m model) isOptionEnabled(index int) bool {
	switch index {
	case 0: // Instalar - solo si NO está instalado
		return strings.Contains(m.serviceState, "NOT INSTALLED")
	case 1: // Desinstalar - solo si está instalado
		return !strings.Contains(m.serviceState, "NOT INSTALLED")
	case 2: // Iniciar - solo si está detenido
		return strings.Contains(m.serviceState, "STOPPED")
	case 3: // Detener - solo si está ejecutándose
		return strings.Contains(m.serviceState, "RUNNING")
	default:
		return true
	}
}

// Obtener razón por la cual una opción está deshabilitada
func (m model) getDisabledReason(index int) string {
	switch index {
	case 0: // Instalar
		return "❌ El servicio ya está instalado"
	case 1: // Desinstalar
		return "❌ El servicio no está instalado"
	case 2: // Iniciar
		if strings.Contains(m.serviceState, "RUNNING") {
			return "❌ El servicio ya está en ejecución"
		}
		return "❌ El servicio debe estar instalado primero"
	case 3: // Detener
		return "❌ El servicio no está en ejecución"
	default:
		return "❌ Opción no disponible"
	}
}

// Obtener representación visual del estado
func (m model) getStatusDisplay() string {
	if strings.Contains(m.serviceState, "RUNNING") {
		return "✅ EN EJECUCIÓN"
	} else if strings.Contains(m.serviceState, "STOPPED") {
		return "⏸️ DETENIDO"
	} else if strings.Contains(m.serviceState, "NOT INSTALLED") {
		return "❌ NO INSTALADO"
	}
	return "❓ DESCONOCIDO"
}

func (m model) View() string {
	switch m.screen {
	case menuScreen:
		return m.viewMenu()
	case installingScreen:
		return m.viewInstalling()
	case resultScreen:
		return m.viewResult()
	case confirmScreen:
		return m.viewConfirm()
	default:
		return ""
	}
}

func (m model) viewMenu() string {
	var b strings.Builder

	// Banner animado
	b.WriteString(bannerStyle.Render(banner) + "\n\n")

	// Estado del servicio con colores
	status := m.getStatusDisplay()
	statusLine := fmt.Sprintf("📊 Estado Actual: %s", status)
	b.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(accentColor).
		Render(statusLine) + "\n")

	// Menú con opciones coloreadas
	menuContent := ""
	for i, option := range m.menuOptions {
		cursor := "  "
		style := normalStyle

		// Verificar si la opción está habilitada
		enabled := m.isOptionEnabled(i)

		if i == m.menuCursor {
			cursor = "▸ "
			if enabled {
				style = selectedStyle
			} else {
				// Opción seleccionada pero deshabilitada
				style = lipgloss.NewStyle().
					Foreground(warningColor).
					Bold(true)
			}
		} else if !enabled {
			style = disabledStyle
		}

		menuContent += fmt.Sprintf("%s%s\n", cursor, style.Render(option))
	}

	b.WriteString(menuStyle.Render(menuContent))

	// Instrucciones con colores
	instructions := "⌨️  ↑/↓: Navegar • Enter: Seleccionar • Q: Salir"
	b.WriteString("\n" + infoStyle.Render(instructions))

	// Agregar indicador de versión
	version := "\n\n" + lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Italic(true).
		Render("v1.0.0 - © 2024 BasculaServicio")
	b.WriteString(version)

	return b.String()
}

func (m model) viewInstalling() string {
	var b strings.Builder

	b.WriteString(bannerStyle.Render(banner) + "\n\n")

	// Animación del spinner
	b.WriteString(m.spinner.View() + " Procesando operación...\n\n")

	// Barra de progreso si hay progreso
	if m.progressPercent > 0 {
		b.WriteString(m.progress.ViewAs(m.progressPercent))
		b.WriteString(fmt.Sprintf("\n%.0f%% completado\n", m.progressPercent*100))
	}

	// Mensaje informativo
	info := "⏳ Por favor espere mientras se completa la operación..."
	b.WriteString("\n" + infoStyle.Render(info))

	return b.String()
}

func (m model) viewConfirm() string {
	var b strings.Builder

	b.WriteString(bannerStyle.Render(banner) + "\n\n")

	// Título de confirmación
	title := warningStyle.Render("⚠️ CONFIRMACIÓN REQUERIDA")
	b.WriteString(title + "\n\n")

	// Mensaje de confirmación
	b.WriteString(m.confirmAction + "\n\n")

	// Opciones
	options := successStyle.Render("[S]í") + " / " + errorStyle.Render("[N]o")
	b.WriteString(options + "\n\n")

	// Instrucciones
	instructions := "Presione S para confirmar, N o ESC para cancelar"
	b.WriteString(infoStyle.Render(instructions))

	return b.String()
}

func (m model) viewResult() string {
	var b strings.Builder

	b.WriteString(bannerStyle.Render(banner) + "\n\n")

	// Mostrar resultado con emoji apropiado
	var resultIcon string
	var resultStyle lipgloss.Style
	if m.success {
		resultIcon = "✅"
		resultStyle = successStyle
	} else {
		resultIcon = "❌"
		resultStyle = errorStyle
	}

	result := fmt.Sprintf("%s %s", resultIcon, m.result)
	b.WriteString(resultStyle.Render(result))

	// Instrucciones
	b.WriteString("\n\n" + infoStyle.Render("Presione Enter para continuar..."))

	return b.String()
}

// Comandos de servicio con manejo de errores mejorado
func installServiceCmd() tea.Cmd {
	return func() tea.Msg {
		if !isAdmin() {
			return operationDoneMsg{
				success: false,
				message: "Se requieren permisos de administrador.\nEjecute el programa como administrador.",
			}
		}

		// Directorio de instalación
		targetDir := filepath.Join(os.Getenv("ProgramFiles"), serviceName)
		targetPath := filepath.Join(targetDir, serviceName+".exe")

		// Crear directorio con manejo de errores
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("Error al crear directorio: %v", err),
			}
		}

		// Escribir el servicio embebido al disco
		if err := os.WriteFile(targetPath, embedded.BasculaServicioExe, 0755); err != nil {
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("Error al extraer servicio: %v", err),
			}
		}

		// Crear servicio con sc.exe
		cmd := exec.Command("sc", "create", serviceName,
			fmt.Sprintf("binPath=%s", targetPath),
			"start=auto",
			fmt.Sprintf("DisplayName=%s", serviceDisplayName))

		if output, err := cmd.CombinedOutput(); err != nil {
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("Error al crear servicio: %s", string(output)),
			}
		}

		// Configurar descripción del servicio
		exec.Command("sc", "description", serviceName, serviceDescription).Run()

		// Configurar recuperación automática ante fallos
		exec.Command("sc", "failure", serviceName,
			"reset=86400",
			"actions=restart/5000/restart/5000/restart/5000").Run()

		// Iniciar el servicio automáticamente
		exec.Command("sc", "start", serviceName).Run()

		return operationDoneMsg{
			success: true,
			message: "Servicio instalado e iniciado correctamente ✨",
		}
	}
}

func uninstallServiceCmd() tea.Cmd {
	return func() tea.Msg {
		if !isAdmin() {
			return operationDoneMsg{
				success: false,
				message: "Se requieren permisos de administrador.\nEjecute el programa como administrador.",
			}
		}

		// Detener el servicio primero
		exec.Command("sc", "stop", serviceName).Run()
		time.Sleep(2 * time.Second)

		// Eliminar el servicio del registro
		cmd := exec.Command("sc", "delete", serviceName)
		if output, err := cmd.CombinedOutput(); err != nil {
			// Verificar si el servicio no existe
			if strings.Contains(string(output), "1060") {
				return operationDoneMsg{
					success: false,
					message: "El servicio no está instalado",
				}
			}
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("Error al eliminar servicio: %s", string(output)),
			}
		}

		// Limpiar archivos de instalación
		targetDir := filepath.Join(os.Getenv("ProgramFiles"), serviceName)
		os.RemoveAll(targetDir)

		return operationDoneMsg{
			success: true,
			message: "Servicio desinstalado completamente 🧹",
		}
	}
}

func startServiceCmd() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sc", "start", serviceName)
		if output, err := cmd.CombinedOutput(); err != nil {
			// Verificar errores específicos
			outputStr := string(output)
			if strings.Contains(outputStr, "1056") {
				return operationDoneMsg{
					success: false,
					message: "El servicio ya está en ejecución",
				}
			} else if strings.Contains(outputStr, "1060") {
				return operationDoneMsg{
					success: false,
					message: "El servicio no está instalado",
				}
			}
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("Error al iniciar: %s", outputStr),
			}
		}

		return operationDoneMsg{
			success: true,
			message: "Servicio iniciado correctamente ▶️",
		}
	}
}

func stopServiceCmd() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("sc", "stop", serviceName)
		if output, err := cmd.CombinedOutput(); err != nil {
			outputStr := string(output)
			if strings.Contains(outputStr, "1062") {
				return operationDoneMsg{
					success: false,
					message: "El servicio no está en ejecución",
				}
			}
			return operationDoneMsg{
				success: false,
				message: fmt.Sprintf("Error al detener: %s", outputStr),
			}
		}

		return operationDoneMsg{
			success: true,
			message: "Servicio detenido correctamente ⏹️",
		}
	}
}

// Verificar el estado actual del servicio Windows
func checkServiceStatus() string {
	cmd := exec.Command("sc", "query", serviceName)
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

// Verificar si el programa se ejecuta con permisos de administrador
func isAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

// Abrir el archivo de logs en el bloc de notas
func viewLogs() {
	logPath := filepath.Join(os.Getenv("PROGRAMDATA"), serviceName, serviceName+".log")
	exec.Command("notepad.exe", logPath).Start()
}

func main() {
	// Verificar permisos de administrador
	if !isAdmin() {
		fmt.Println(errorStyle.Render("⚠️  Este programa requiere permisos de Administrador"))
		fmt.Println(infoStyle.Render("\n📌 Cómo ejecutar como administrador:"))
		fmt.Println("1. Cierre esta ventana")
		fmt.Println("2. Haga clic derecho en el archivo")
		fmt.Println("3. Seleccione 'Ejecutar como administrador'")
		fmt.Println("\nPresione Enter para salir...")
		fmt.Scanln()
		os.Exit(1)
	}

	// Iniciar la aplicación BubbleTea
	p := tea.NewProgram(
		initialModel(),
		tea.WithAltScreen(), // Usar pantalla alternativa para mejor experiencia
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
