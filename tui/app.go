package tui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/galamiram/nadctl/nadapi"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// App represents the main TUI application
type App struct {
	keys          keyMap
	help          help.Model
	device        *nadapi.Device
	connected     bool
	connecting    bool
	status        DeviceStatus
	message       string
	messageType   MessageType
	width         int
	height        int
	lastUpdate    time.Time
	autoRefresh   bool
	volumeBar     progress.Model
	brightnessBar progress.Model
	spinner       string
	spinnerIndex  int
}

// DeviceStatus holds the current device state
type DeviceStatus struct {
	Power         string
	Volume        float64
	VolumeStr     string
	Source        string
	Mute          string
	Brightness    int
	BrightnessStr string
	Model         string
	IP            string
}

// MessageType represents the type of message to display
type MessageType int

const (
	MessageInfo MessageType = iota
	MessageSuccess
	MessageError
	MessageWarning
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// keyMap defines the keyboard shortcuts
type keyMap struct {
	Up         key.Binding
	Down       key.Binding
	Left       key.Binding
	Right      key.Binding
	Power      key.Binding
	Mute       key.Binding
	VolumeUp   key.Binding
	VolumeDown key.Binding
	Refresh    key.Binding
	Discover   key.Binding
	Help       key.Binding
	Quit       key.Binding
}

// ShortHelp returns the key bindings to be shown in the mini help view
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Power, k.Mute, k.VolumeUp, k.VolumeDown, k.Help, k.Quit}
}

// FullHelp returns the key bindings to be shown in the full help view
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Power, k.Mute, k.VolumeUp, k.VolumeDown},
		{k.Left, k.Right, k.Up, k.Down},
		{k.Refresh, k.Discover, k.Help, k.Quit},
	}
}

var keys = keyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "brightness up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "brightness down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "prev source"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "next source"),
	),
	Power: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "toggle power"),
	),
	Mute: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "toggle mute"),
	),
	VolumeUp: key.NewBinding(
		key.WithKeys("+", "="),
		key.WithHelp("+", "volume up"),
	),
	VolumeDown: key.NewBinding(
		key.WithKeys("-"),
		key.WithHelp("-", "volume down"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh status"),
	),
	Discover: key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", "discover devices"),
	),
	Help: key.NewBinding(
		key.WithKeys("?"),
		key.WithHelp("?", "toggle help"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "esc", "ctrl+c"),
		key.WithHelp("q", "quit"),
	),
}

// NewApp creates a new TUI application
func NewApp() *App {
	volumeBar := progress.New(progress.WithDefaultGradient())
	brightnessBar := progress.New(progress.WithDefaultGradient())

	return &App{
		keys:          keys,
		help:          help.New(),
		autoRefresh:   true,
		message:       "Starting NAD Controller...",
		messageType:   MessageInfo,
		volumeBar:     volumeBar,
		brightnessBar: brightnessBar,
		spinner:       spinnerFrames[0],
		spinnerIndex:  0,
	}
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	return tea.Batch(
		a.connectToDevice(),
		a.tickCmd(),
	)
}

// Update handles messages and updates the application state
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.help.Width = msg.Width

	case tea.KeyMsg:
		// Debug: log key presses to help diagnose issues
		log.WithFields(log.Fields{
			"key":  msg.String(),
			"type": msg.Type.String(),
		}).Debug("Key pressed")

		// Handle basic keys that should always work
		switch {
		case key.Matches(msg, a.keys.Quit):
			return a, tea.Quit

		case key.Matches(msg, a.keys.Help):
			a.help.ShowAll = !a.help.ShowAll
			return a, nil

		case key.Matches(msg, a.keys.Discover):
			// Discovery should work even when not connected
			return a, a.discoverDevices()

		case key.Matches(msg, a.keys.Refresh):
			// Allow refresh even when not connected (will show appropriate message)
			if a.connected {
				return a, a.refreshStatus()
			} else {
				a.setMessage("Not connected to device", MessageWarning)
				return a, nil
			}
		}

		// Handle device control keys (only when connected)
		if a.connected {
			switch {
			case key.Matches(msg, a.keys.Power):
				a.setMessage("Toggling power...", MessageInfo)
				return a, a.togglePower()

			case key.Matches(msg, a.keys.Mute):
				a.setMessage("Toggling mute...", MessageInfo)
				return a, a.toggleMute()

			case key.Matches(msg, a.keys.VolumeUp):
				a.setMessage("Increasing volume...", MessageInfo)
				return a, a.volumeUp()

			case key.Matches(msg, a.keys.VolumeDown):
				a.setMessage("Decreasing volume...", MessageInfo)
				return a, a.volumeDown()

			case key.Matches(msg, a.keys.Left):
				a.setMessage("Changing to previous source...", MessageInfo)
				return a, a.prevSource()

			case key.Matches(msg, a.keys.Right):
				a.setMessage("Changing to next source...", MessageInfo)
				return a, a.nextSource()

			case key.Matches(msg, a.keys.Up):
				a.setMessage("Increasing brightness...", MessageInfo)
				return a, a.brightnessUp()

			case key.Matches(msg, a.keys.Down):
				a.setMessage("Decreasing brightness...", MessageInfo)
				return a, a.brightnessDown()
			}
		} else {
			// Give feedback when trying to use device controls while not connected
			switch {
			case key.Matches(msg, a.keys.Power),
				key.Matches(msg, a.keys.Mute),
				key.Matches(msg, a.keys.VolumeUp),
				key.Matches(msg, a.keys.VolumeDown),
				key.Matches(msg, a.keys.Left),
				key.Matches(msg, a.keys.Right),
				key.Matches(msg, a.keys.Up),
				key.Matches(msg, a.keys.Down):
				a.setMessage("Connect to device first (press 'd' to discover)", MessageWarning)
				return a, nil
			}
		}

	case deviceConnectedMsg:
		a.device = msg.device
		a.connected = true
		a.connecting = false
		a.status.IP = msg.device.IP.String()
		a.setMessage("Connected to NAD device!", MessageSuccess)
		return a, a.refreshStatus()

	case deviceErrorMsg:
		a.connected = false
		a.connecting = false
		a.setMessage(fmt.Sprintf("Connection failed: %v", msg.err), MessageError)

	case statusUpdateMsg:
		a.status = msg.status
		a.lastUpdate = time.Now()
		// Update progress bars
		a.volumeBar.SetPercent((a.status.Volume + 80) / 90)          // Volume range -80 to +10
		a.brightnessBar.SetPercent(float64(a.status.Brightness) / 3) // Brightness 0-3

	case messageMsg:
		a.setMessage(msg.text, msg.msgType)

	case tickMsg:
		// Update spinner
		a.spinnerIndex = (a.spinnerIndex + 1) % len(spinnerFrames)
		a.spinner = spinnerFrames[a.spinnerIndex]

		if a.connected && a.autoRefresh && time.Since(a.lastUpdate) > 10*time.Second {
			return a, tea.Batch(a.refreshStatus(), a.tickCmd())
		}
		return a, a.tickCmd()
	}

	return a, nil
}

// View renders the application
func (a *App) View() string {
	if a.width == 0 {
		return "Loading..."
	}

	var sections []string

	// Header
	sections = append(sections, a.renderHeader())

	// Main content in columns
	leftColumn := a.renderLeftColumn()
	rightColumn := a.renderRightColumn()

	mainContent := lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftColumn,
		rightColumn,
	)
	sections = append(sections, mainContent)

	// Message area
	if a.message != "" {
		sections = append(sections, a.renderMessage())
	}

	// Help
	sections = append(sections, a.renderHelp())

	return lipgloss.JoinVertical(lipgloss.Left, sections...)
}

// Enhanced Styles
var (
	// Colors and color styles
	primaryColor = lipgloss.Color("39")  // Blue
	successColor = lipgloss.Color("46")  // Green
	errorColor   = lipgloss.Color("196") // Red
	warningColor = lipgloss.Color("226") // Yellow
	accentColor  = lipgloss.Color("86")  // Cyan
	mutedColor   = lipgloss.Color("240") // Gray
	bgColor      = lipgloss.Color("235") // Dark Gray

	// Color text styles
	primaryTextStyle = lipgloss.NewStyle().Foreground(primaryColor)
	successTextStyle = lipgloss.NewStyle().Foreground(successColor)
	errorTextStyle   = lipgloss.NewStyle().Foreground(errorColor)
	warningTextStyle = lipgloss.NewStyle().Foreground(warningColor)
	accentTextStyle  = lipgloss.NewStyle().Foreground(accentColor)
	mutedTextStyle   = lipgloss.NewStyle().Foreground(mutedColor)

	// Title and headers
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(primaryColor).
			Padding(0, 2).
			Margin(0, 0, 1, 0).
			Bold(true).
			Width(80).
			Align(lipgloss.Center)

	headerStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true).
			Margin(0, 0, 1, 0)

	// Panels
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			Margin(0, 1, 1, 0).
			Width(38)

	connectedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(successColor).
				Padding(1, 2).
				Margin(0, 1, 1, 0).
				Width(38)

	errorPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(errorColor).
			Padding(1, 2).
			Margin(0, 1, 1, 0).
			Width(38)

	// Status indicators
	onStatusStyle = lipgloss.NewStyle().
			Foreground(successColor).
			Bold(true)

	offStatusStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Bold(true)

	powerOnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(successColor).
			Padding(0, 1).
			Bold(true)

	powerOffStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15")).
			Background(mutedColor).
			Padding(0, 1).
			Bold(true)

	// Messages
	infoMessageStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Border(lipgloss.NormalBorder()).
				BorderForeground(primaryColor).
				Padding(0, 1).
				Margin(0, 1)

	successMessageStyle = lipgloss.NewStyle().
				Foreground(successColor).
				Border(lipgloss.NormalBorder()).
				BorderForeground(successColor).
				Padding(0, 1).
				Margin(0, 1)

	errorMessageStyle = lipgloss.NewStyle().
				Foreground(errorColor).
				Border(lipgloss.NormalBorder()).
				BorderForeground(errorColor).
				Padding(0, 1).
				Margin(0, 1)

	warningMessageStyle = lipgloss.NewStyle().
				Foreground(warningColor).
				Border(lipgloss.NormalBorder()).
				BorderForeground(warningColor).
				Padding(0, 1).
				Margin(0, 1)

	// Labels
	labelStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	valueStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("15"))
)

func (a *App) renderHeader() string {
	title := "🎵 NAD Audio Controller"
	subtitle := "Terminal Interface for Premium Audio Control"

	header := titleStyle.Render(title)
	sub := headerStyle.Render(subtitle)

	return lipgloss.JoinVertical(lipgloss.Center, header, sub)
}

func (a *App) renderLeftColumn() string {
	var content strings.Builder

	// Connection Status Panel
	var connectionPanel string
	if a.connecting {
		status := fmt.Sprintf("%s Connecting...", a.spinner)
		connectionPanel = panelStyle.Render(
			labelStyle.Render("Connection Status") + "\n\n" +
				warningTextStyle.Render(status),
		)
	} else if a.connected {
		status := fmt.Sprintf("🟢 Connected to %s", a.status.IP)
		connectionPanel = connectedPanelStyle.Render(
			labelStyle.Render("Connection Status") + "\n\n" +
				successTextStyle.Render(status),
		)
	} else {
		status := "🔴 Disconnected\n\nPress 'd' to discover devices"
		connectionPanel = errorPanelStyle.Render(
			labelStyle.Render("Connection Status") + "\n\n" +
				errorTextStyle.Render(status),
		)
	}

	content.WriteString(connectionPanel)

	// Device Info Panel (only if connected)
	if a.connected && a.status.Model != "" {
		deviceInfo := panelStyle.Render(
			labelStyle.Render("Device Information") + "\n\n" +
				fmt.Sprintf("Model: %s\n", valueStyle.Render(a.status.Model)) +
				fmt.Sprintf("IP: %s", valueStyle.Render(a.status.IP)),
		)
		content.WriteString("\n" + deviceInfo)
	}

	return content.String()
}

func (a *App) renderRightColumn() string {
	if !a.connected {
		return panelStyle.Render(
			labelStyle.Render("Device Controls") + "\n\n" +
				mutedTextStyle.Render("Connect to a device to see controls"),
		)
	}

	var content strings.Builder

	// Power Status Panel
	var powerStatus string
	if a.status.Power == "On" {
		powerStatus = powerOnStyle.Render(" POWER ON ")
	} else {
		powerStatus = powerOffStyle.Render(" POWER OFF ")
	}

	powerPanel := panelStyle.Render(
		labelStyle.Render("Power Status") + "\n\n" +
			powerStatus + "\n\n" +
			mutedTextStyle.Render("Press 'p' to toggle"),
	)
	content.WriteString(powerPanel)

	// Audio Controls Panel
	var muteStatus string
	if a.status.Mute == "On" {
		muteStatus = errorTextStyle.Render("🔇 MUTED")
	} else {
		muteStatus = successTextStyle.Render("🔊 UNMUTED")
	}

	volumeBar := a.volumeBar.ViewAs(a.status.Volume / 100)
	brightnessBar := a.brightnessBar.ViewAs(float64(a.status.Brightness) / 3)

	audioPanel := panelStyle.Render(
		labelStyle.Render("Audio Controls") + "\n\n" +
			fmt.Sprintf("Volume: %s\n", valueStyle.Render(a.status.VolumeStr)) +
			volumeBar + "\n\n" +
			fmt.Sprintf("Source: %s\n", valueStyle.Render(a.status.Source)) +
			fmt.Sprintf("Mute: %s", muteStatus),
	)
	content.WriteString("\n" + audioPanel)

	// Display Controls Panel
	displayPanel := panelStyle.Render(
		labelStyle.Render("Display Controls") + "\n\n" +
			fmt.Sprintf("Brightness: %s\n", valueStyle.Render(a.status.BrightnessStr)) +
			brightnessBar + "\n\n" +
			mutedTextStyle.Render("Use ↑↓ keys to adjust"),
	)
	content.WriteString("\n" + displayPanel)

	return content.String()
}

func (a *App) renderMessage() string {
	var style lipgloss.Style
	var icon string

	switch a.messageType {
	case MessageSuccess:
		style = successMessageStyle
		icon = "✓"
	case MessageError:
		style = errorMessageStyle
		icon = "✗"
	case MessageWarning:
		style = warningMessageStyle
		icon = "⚠"
	default:
		style = infoMessageStyle
		icon = "ℹ"
	}

	return style.Render(fmt.Sprintf("%s %s", icon, a.message))
}

func (a *App) renderHelp() string {
	if !a.lastUpdate.IsZero() {
		lastUpdateText := mutedTextStyle.Render(fmt.Sprintf("Last update: %s", a.lastUpdate.Format("15:04:05")))
		helpContent := a.help.View(a.keys)
		return lipgloss.JoinVertical(lipgloss.Left, lastUpdateText, helpContent)
	}
	return a.help.View(a.keys)
}

func (a *App) setMessage(text string, msgType MessageType) {
	a.message = text
	a.messageType = msgType
}

// Command functions
func (a *App) connectToDevice() tea.Cmd {
	return func() tea.Msg {
		a.connecting = true

		// Try to get IP from config
		ip := viper.GetString("ip")

		var device *nadapi.Device
		var err error

		if ip == "" {
			// Discover devices
			log.Debug("No IP configured, discovering devices...")
			devices, _, discoverErr := nadapi.DiscoverDevicesWithCache(30*time.Second, true, nadapi.DefaultCacheTTL)
			if discoverErr != nil {
				return deviceErrorMsg{err: discoverErr}
			}

			if len(devices) == 0 {
				return deviceErrorMsg{err: fmt.Errorf("no NAD devices found on the network")}
			}

			ip = devices[0].IP
		}

		device, err = nadapi.New(ip, "")
		if err != nil {
			return deviceErrorMsg{err: err}
		}

		return deviceConnectedMsg{device: device}
	}
}

func (a *App) refreshStatus() tea.Cmd {
	return func() tea.Msg {
		if a.device == nil {
			return messageMsg{text: "No device connected", msgType: MessageError}
		}

		status := DeviceStatus{IP: a.device.IP.String()}

		// Get power state
		if power, err := a.device.GetPowerState(); err == nil {
			status.Power = power
		} else {
			status.Power = "Unknown"
		}

		// Get volume
		if volume, err := a.device.GetVolumeFloat(); err == nil {
			status.Volume = volume
			status.VolumeStr = fmt.Sprintf("%.1f dB", volume)
		} else {
			status.Volume = -80
			status.VolumeStr = "Unknown"
		}

		// Get source
		if source, err := a.device.GetSource(); err == nil {
			status.Source = source
		} else {
			status.Source = "Unknown"
		}

		// Get mute status
		if mute, err := a.device.GetMuteStatus(); err == nil {
			status.Mute = mute
		} else {
			status.Mute = "Unknown"
		}

		// Get brightness
		if brightness, err := a.device.GetBrightnessInt(); err == nil {
			status.Brightness = brightness
			status.BrightnessStr = strconv.Itoa(brightness)
		} else {
			status.Brightness = 0
			status.BrightnessStr = "Unknown"
		}

		// Get model
		if model, err := a.device.GetModel(); err == nil {
			status.Model = model
		} else {
			status.Model = "Unknown"
		}

		return statusUpdateMsg{status: status}
	}
}

func (a *App) togglePower() tea.Cmd {
	return func() tea.Msg {
		if err := a.device.PowerToggle(); err != nil {
			return messageMsg{text: fmt.Sprintf("Failed to toggle power: %v", err), msgType: MessageError}
		}
		return tea.Batch(
			func() tea.Msg { return messageMsg{text: "Power toggled", msgType: MessageSuccess} },
			a.refreshStatus(),
		)()
	}
}

func (a *App) toggleMute() tea.Cmd {
	return func() tea.Msg {
		if err := a.device.ToggleMute(); err != nil {
			return messageMsg{text: fmt.Sprintf("Failed to toggle mute: %v", err), msgType: MessageError}
		}
		return tea.Batch(
			func() tea.Msg { return messageMsg{text: "Mute toggled", msgType: MessageSuccess} },
			a.refreshStatus(),
		)()
	}
}

func (a *App) volumeUp() tea.Cmd {
	return func() tea.Msg {
		if err := a.device.TuneVolume(nadapi.DirectionUp); err != nil {
			return messageMsg{text: fmt.Sprintf("Failed to increase volume: %v", err), msgType: MessageError}
		}
		return tea.Batch(
			func() tea.Msg { return messageMsg{text: "Volume increased", msgType: MessageSuccess} },
			a.refreshStatus(),
		)()
	}
}

func (a *App) volumeDown() tea.Cmd {
	return func() tea.Msg {
		if err := a.device.TuneVolume(nadapi.DirectionDown); err != nil {
			return messageMsg{text: fmt.Sprintf("Failed to decrease volume: %v", err), msgType: MessageError}
		}
		return tea.Batch(
			func() tea.Msg { return messageMsg{text: "Volume decreased", msgType: MessageSuccess} },
			a.refreshStatus(),
		)()
	}
}

func (a *App) nextSource() tea.Cmd {
	return func() tea.Msg {
		if _, err := a.device.ToggleSource(nadapi.DirectionUp); err != nil {
			return messageMsg{text: fmt.Sprintf("Failed to change source: %v", err), msgType: MessageError}
		}
		return tea.Batch(
			func() tea.Msg { return messageMsg{text: "Source changed to next", msgType: MessageSuccess} },
			a.refreshStatus(),
		)()
	}
}

func (a *App) prevSource() tea.Cmd {
	return func() tea.Msg {
		if _, err := a.device.ToggleSource(nadapi.DirectionDown); err != nil {
			return messageMsg{text: fmt.Sprintf("Failed to change source: %v", err), msgType: MessageError}
		}
		return tea.Batch(
			func() tea.Msg { return messageMsg{text: "Source changed to previous", msgType: MessageSuccess} },
			a.refreshStatus(),
		)()
	}
}

func (a *App) brightnessUp() tea.Cmd {
	return func() tea.Msg {
		if err := a.device.ToggleBrightness(nadapi.DirectionUp); err != nil {
			return messageMsg{text: fmt.Sprintf("Failed to increase brightness: %v", err), msgType: MessageError}
		}
		return tea.Batch(
			func() tea.Msg { return messageMsg{text: "Brightness increased", msgType: MessageSuccess} },
			a.refreshStatus(),
		)()
	}
}

func (a *App) brightnessDown() tea.Cmd {
	return func() tea.Msg {
		if err := a.device.ToggleBrightness(nadapi.DirectionDown); err != nil {
			return messageMsg{text: fmt.Sprintf("Failed to decrease brightness: %v", err), msgType: MessageError}
		}
		return tea.Batch(
			func() tea.Msg { return messageMsg{text: "Brightness decreased", msgType: MessageSuccess} },
			a.refreshStatus(),
		)()
	}
}

func (a *App) discoverDevices() tea.Cmd {
	return func() tea.Msg {
		devices, _, err := nadapi.DiscoverDevicesWithCache(30*time.Second, false, nadapi.DefaultCacheTTL)
		if err != nil {
			return messageMsg{text: fmt.Sprintf("Discovery failed: %v", err), msgType: MessageError}
		}

		if len(devices) == 0 {
			return messageMsg{text: "No NAD devices found on the network", msgType: MessageWarning}
		}

		return messageMsg{text: fmt.Sprintf("Found %d device(s)", len(devices)), msgType: MessageSuccess}
	}
}

func (a *App) tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Messages
type deviceConnectedMsg struct {
	device *nadapi.Device
}

type deviceErrorMsg struct {
	err error
}

type statusUpdateMsg struct {
	status DeviceStatus
}

type messageMsg struct {
	text    string
	msgType MessageType
}

type tickMsg struct{}
