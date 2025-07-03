package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/galamiram/nadctl/internal/version"
	"github.com/galamiram/nadctl/nadapi"
	"github.com/galamiram/nadctl/spotify"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

// Tab represents different application tabs
type Tab int

const (
	TabDevice Tab = iota
	TabSpotify
	TabSettings
	TabLogs
)

// TabInfo holds information about each tab
type TabInfo struct {
	ID       Tab
	Name     string
	Icon     string
	ShortKey string
}

// App represents the main TUI application
type App struct {
	keys           keyMap
	help           help.Model
	device         *nadapi.Device
	connected      bool
	connecting     bool
	status         DeviceStatus
	message        string
	messageType    MessageType
	width          int
	height         int
	lastUpdate     time.Time
	autoRefresh    bool
	volumeBar      progress.Model
	brightnessBar  progress.Model
	spinner        string
	spinnerIndex   int
	volumeInput    textinput.Model
	inputMode      bool          // true when in volume input mode
	adjustMode     bool          // true when in volume adjustment mode
	pendingVolume  float64       // pending volume change
	originalVolume float64       // original volume before adjustment
	adjustTimer    *time.Timer   // timer for auto-commit
	commandQueue   *CommandQueue // queue for async command processing
	processing     bool          // true when a command is being processed
	resultChan     chan tea.Msg  // channel for results from background processing

	// Tab system
	currentTab  Tab       // current active tab
	tabs        []TabInfo // available tabs
	tabsEnabled bool      // whether tab system is enabled

	// Spotify integration
	spotifyClient    *spotify.Client
	spotifyConnected bool
	spotifyState     *spotify.PlaybackState
	spotifyEnabled   bool // whether Spotify panel is visible
	spotifyProgress  progress.Model
	// Spotify authentication
	spotifyAuthMode  bool // true when waiting for auth code input
	spotifyAuthInput textinput.Model
	spotifyAuthURL   string // URL to show user for authentication
	// Spotify device management
	spotifyDevices         []spotify.Device // available Spotify devices
	spotifyDeviceSelection int              // currently selected device index
	spotifyDeviceMode      bool             // true when in device selection mode
	spotifyDevicesLoaded   bool             // true when device list has been loaded

	// Demo mode (no NAD device required)
	demoMode bool // true when running in demo mode

	// Logs panel
	logEntries    []LogEntry // stored log entries for display
	logScrollPos  int        // current scroll position in logs
	maxLogEntries int        // maximum number of log entries to keep
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

// LogEntry represents a single log entry for display in the logs tab
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	Fields    map[string]interface{}
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// CommandType represents different types of commands that can be queued
type CommandType int

const (
	CmdPowerToggle CommandType = iota
	CmdMuteToggle
	CmdVolumeSet
	CmdVolumeUp
	CmdVolumeDown
	CmdSourceNext
	CmdSourcePrev
	CmdBrightnessUp
	CmdBrightnessDown
	CmdRefreshStatus
	CmdDiscoverDevices
	CmdConnectDevice
	// Spotify commands
	CmdSpotifyPlayPause
	CmdSpotifyNext
	CmdSpotifyPrev
	CmdSpotifyVolumeUp
	CmdSpotifyVolumeDown
	CmdSpotifyToggleShuffle
	CmdSpotifyRefresh
	CmdSpotifyAuth
	CmdSpotifyDisconnect
	// New Spotify device commands
	CmdSpotifyListDevices
	CmdSpotifyTransferDevice
)

// QueuedCommand represents a command in the queue
type QueuedCommand struct {
	Type      CommandType
	Params    map[string]interface{}
	ID        string
	Timestamp time.Time
}

// CommandQueue manages the queue of commands to execute
type CommandQueue struct {
	commands []QueuedCommand
	mutex    sync.RWMutex
	running  bool
}

// NewCommandQueue creates a new command queue
func NewCommandQueue() *CommandQueue {
	return &CommandQueue{
		commands: make([]QueuedCommand, 0),
		running:  false,
	}
}

// Add adds a command to the queue, replacing similar commands for efficiency
func (cq *CommandQueue) Add(cmd QueuedCommand) {
	cq.mutex.Lock()
	defer cq.mutex.Unlock()

	// For volume commands, replace existing volume commands instead of queuing
	if cmd.Type == CmdVolumeSet || cmd.Type == CmdVolumeUp || cmd.Type == CmdVolumeDown {
		// Remove existing volume commands
		filtered := make([]QueuedCommand, 0)
		for _, existing := range cq.commands {
			if existing.Type != CmdVolumeSet && existing.Type != CmdVolumeUp && existing.Type != CmdVolumeDown {
				filtered = append(filtered, existing)
			}
		}
		cq.commands = filtered
	}

	// For refresh commands, only keep the latest one
	if cmd.Type == CmdRefreshStatus {
		filtered := make([]QueuedCommand, 0)
		for _, existing := range cq.commands {
			if existing.Type != CmdRefreshStatus {
				filtered = append(filtered, existing)
			}
		}
		cq.commands = filtered
	}

	cq.commands = append(cq.commands, cmd)
}

// Next returns the next command to execute and removes it from the queue
func (cq *CommandQueue) Next() (QueuedCommand, bool) {
	cq.mutex.Lock()
	defer cq.mutex.Unlock()

	if len(cq.commands) == 0 {
		return QueuedCommand{}, false
	}

	cmd := cq.commands[0]
	cq.commands = cq.commands[1:]
	return cmd, true
}

// Len returns the number of commands in the queue
func (cq *CommandQueue) Len() int {
	cq.mutex.RLock()
	defer cq.mutex.RUnlock()
	return len(cq.commands)
}

// Clear removes all commands from the queue
func (cq *CommandQueue) Clear() {
	cq.mutex.Lock()
	defer cq.mutex.Unlock()
	cq.commands = make([]QueuedCommand, 0)
}

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
	VolumeSet  key.Binding
	Refresh    key.Binding
	Discover   key.Binding
	Help       key.Binding
	Quit       key.Binding
	// Tab navigation
	NextTab key.Binding
	PrevTab key.Binding
	Tab1    key.Binding
	Tab2    key.Binding
	Tab3    key.Binding
	Tab4    key.Binding
	// Spotify controls
	SpotifyToggle     key.Binding
	SpotifyPlayPause  key.Binding
	SpotifyNext       key.Binding
	SpotifyPrev       key.Binding
	SpotifyVolUp      key.Binding
	SpotifyVolDown    key.Binding
	SpotifyShuffle    key.Binding
	SpotifyAuth       key.Binding
	SpotifyDisconnect key.Binding
	// Log controls
	LogScrollUp   key.Binding
	LogScrollDown key.Binding
	// New Spotify device controls
	SpotifyDevices      key.Binding
	SpotifyTransfer     key.Binding
	SpotifyDeviceUp     key.Binding
	SpotifyDeviceDown   key.Binding
	SpotifyDeviceSelect key.Binding
}

// ShortHelp returns the key bindings to be shown in the mini help view
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Power, k.Mute, k.VolumeUp, k.VolumeDown, k.SpotifyPlayPause, k.Help, k.Quit}
}

// FullHelp returns the key bindings to be shown in the full help view
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Power, k.Mute, k.VolumeUp, k.VolumeDown, k.VolumeSet},
		{k.Left, k.Right, k.Up, k.Down},
		{k.SpotifyToggle, k.SpotifyPlayPause, k.SpotifyNext, k.SpotifyPrev},
		{k.SpotifyAuth, k.SpotifyDisconnect, k.Refresh, k.Discover, k.Help, k.Quit},
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
		key.WithHelp("+", "volume adjust/up"),
	),
	VolumeDown: key.NewBinding(
		key.WithKeys("-"),
		key.WithHelp("-", "volume adjust/down"),
	),
	VolumeSet: key.NewBinding(
		key.WithKeys("s"),
		key.WithHelp("s", "set volume"),
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
	// Tab navigation
	NextTab: key.NewBinding(key.WithKeys("tab"), key.WithHelp("tab", "next tab")),
	PrevTab: key.NewBinding(key.WithKeys("shift+tab"), key.WithHelp("shift+tab", "previous tab")),
	Tab1:    key.NewBinding(key.WithKeys("1"), key.WithHelp("1", "select tab 1")),
	Tab2:    key.NewBinding(key.WithKeys("2"), key.WithHelp("2", "select tab 2")),
	Tab3:    key.NewBinding(key.WithKeys("3"), key.WithHelp("3", "select tab 3")),
	Tab4:    key.NewBinding(key.WithKeys("4"), key.WithHelp("4", "select tab 4")),
	// Spotify controls
	SpotifyToggle:     key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "toggle Spotify")),
	SpotifyPlayPause:  key.NewBinding(key.WithKeys("space"), key.WithHelp("space", "play/pause Spotify")),
	SpotifyNext:       key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "next Spotify")),
	SpotifyPrev:       key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "prev Spotify")),
	SpotifyVolUp:      key.NewBinding(key.WithKeys("+"), key.WithHelp("+", "volume up Spotify")),
	SpotifyVolDown:    key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "volume down Spotify")),
	SpotifyShuffle:    key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "shuffle Spotify")),
	SpotifyAuth:       key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "authenticate Spotify")),
	SpotifyDisconnect: key.NewBinding(key.WithKeys("x"), key.WithHelp("x", "disconnect Spotify")),
	// Log controls
	LogScrollUp:   key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "scroll up")),
	LogScrollDown: key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "scroll down")),
	// New Spotify device controls
	SpotifyDevices:      key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "list Spotify devices")),
	SpotifyTransfer:     key.NewBinding(key.WithKeys("y"), key.WithHelp("y", "transfer Spotify device")),
	SpotifyDeviceUp:     key.NewBinding(key.WithKeys("+"), key.WithHelp("+", "increase Spotify device volume")),
	SpotifyDeviceDown:   key.NewBinding(key.WithKeys("-"), key.WithHelp("-", "decrease Spotify device volume")),
	SpotifyDeviceSelect: key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "select Spotify device")),
}

// NewApp creates a new TUI application
func NewApp() *App {
	volumeBar := progress.New(progress.WithDefaultGradient())
	brightnessBar := progress.New(progress.WithDefaultGradient())
	spotifyProgress := progress.New(progress.WithDefaultGradient())

	// Initialize volume input
	volumeInput := textinput.New()
	volumeInput.Placeholder = "Enter volume (-80 to +10)"
	volumeInput.CharLimit = 6
	volumeInput.Width = 25

	// Initialize Spotify auth input
	spotifyAuthInput := textinput.New()
	spotifyAuthInput.Placeholder = "Paste authorization code here"
	spotifyAuthInput.CharLimit = 200
	spotifyAuthInput.Width = 50

	// Initialize Spotify client if configured
	var spotifyClient *spotify.Client
	clientID := viper.GetString("spotify.client_id")
	redirectURL := viper.GetString("spotify.redirect_url")

	if clientID != "" {
		if redirectURL == "" {
			redirectURL = "http://localhost:8888/callback"
		}
		spotifyClient = spotify.NewClient(clientID, redirectURL)
		log.Debug("Spotify client initialized with PKCE flow (no client secret needed)")
	} else {
		log.Debug("Spotify not configured - set spotify.client_id in config (no client secret needed for PKCE)")
	}

	return &App{
		keys:           keys,
		help:           help.New(),
		autoRefresh:    true,
		message:        "Starting NAD Controller...",
		messageType:    MessageInfo,
		volumeBar:      volumeBar,
		brightnessBar:  brightnessBar,
		spinner:        spinnerFrames[0],
		spinnerIndex:   0,
		volumeInput:    volumeInput,
		inputMode:      false,
		adjustMode:     false,
		pendingVolume:  0,
		originalVolume: 0,
		adjustTimer:    nil,
		commandQueue:   NewCommandQueue(),
		processing:     false,
		resultChan:     make(chan tea.Msg, 10), // Buffered channel
		// Tab system
		currentTab:  TabDevice,
		tabsEnabled: true,
		tabs: []TabInfo{
			{ID: TabDevice, Name: "Device", Icon: "🎛️", ShortKey: "1"},
			{ID: TabSpotify, Name: "Spotify", Icon: "🎵", ShortKey: "2"},
			{ID: TabSettings, Name: "Settings", Icon: "⚙️", ShortKey: "3"},
			{ID: TabLogs, Name: "Logs", Icon: "��", ShortKey: "4"},
		},
		// Spotify
		spotifyClient:    spotifyClient,
		spotifyConnected: false,
		spotifyState:     nil,
		spotifyEnabled:   true, // Show Spotify panel by default
		spotifyProgress:  spotifyProgress,
		// Spotify auth
		spotifyAuthMode:  false,
		spotifyAuthInput: spotifyAuthInput,
		spotifyAuthURL:   "",
		// Spotify device management
		spotifyDevices:         make([]spotify.Device, 0),
		spotifyDeviceSelection: 0,
		spotifyDeviceMode:      false,
		spotifyDevicesLoaded:   false,
		// Demo mode (no NAD device required)
		demoMode: false,
		// Logs panel
		logEntries:    make([]LogEntry, 0),
		logScrollPos:  0,
		maxLogEntries: 1000, // Keep last 1000 log entries
	}
}

// Init initializes the application
func (a *App) Init() tea.Cmd {
	// Start the command processor goroutine
	go a.processCommands()

	return tea.Batch(
		a.connectToDevice(),
		a.tickCmd(),
		a.listenForResults(),
	)
}

// listenForResults listens for results from the background processor
func (a *App) listenForResults() tea.Cmd {
	return func() tea.Msg {
		return <-a.resultChan
	}
}

// Update handles messages and updates the application state
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.help.Width = msg.Width

	case tea.MouseMsg:
		// Filter out scroll wheel events to prevent them from being converted to arrow keys
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			// Ignore scroll wheel events completely
			return a, nil
		}
		// Allow other mouse events to pass through (if needed in the future)
		return a, nil

	case tea.KeyMsg:
		// Debug: log key presses to help diagnose issues
		log.WithFields(log.Fields{
			"key":  msg.String(),
			"type": msg.Type.String(),
		}).Debug("Key pressed")

		// Handle input mode (volume setting)
		if a.inputMode {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				// Process volume input
				volumeStr := a.volumeInput.Value()
				if volumeStr != "" {
					if volume, err := strconv.ParseFloat(volumeStr, 64); err == nil {
						// Validate volume range
						if volume >= -80 && volume <= 10 {
							a.inputMode = false
							a.volumeInput.Reset()
							a.setMessage("Setting volume...", MessageInfo)
							return a, a.setSpecificVolume(volume)
						} else {
							a.setMessage("Volume must be between -80 and +10 dB", MessageError)
							return a, nil
						}
					} else {
						a.setMessage("Invalid volume format. Use numbers like -20 or 5.5", MessageError)
						return a, nil
					}
				}
				a.inputMode = false
				a.volumeInput.Reset()
				return a, nil

			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				// Cancel volume input
				a.inputMode = false
				a.volumeInput.Reset()
				a.setMessage("Volume input cancelled", MessageInfo)
				return a, nil

			default:
				// Update text input
				var cmd tea.Cmd
				a.volumeInput, cmd = a.volumeInput.Update(msg)
				return a, cmd
			}
		}

		// Handle Spotify authentication mode
		if a.spotifyAuthMode {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				// Process Spotify auth code
				authCode := a.spotifyAuthInput.Value()
				if authCode != "" && a.spotifyClient != nil {
					a.spotifyAuthMode = false
					a.spotifyAuthInput.Reset()
					a.setMessage("Authenticating with Spotify...", MessageInfo)
					return a, a.completeSpotifyAuth(authCode)
				}
				a.spotifyAuthMode = false
				a.spotifyAuthInput.Reset()
				return a, nil

			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				// Cancel Spotify auth
				a.spotifyAuthMode = false
				a.spotifyAuthInput.Reset()
				a.setMessage("Spotify authentication cancelled", MessageInfo)
				return a, nil

			default:
				// Update text input
				var cmd tea.Cmd
				a.spotifyAuthInput, cmd = a.spotifyAuthInput.Update(msg)
				return a, cmd
			}
		}

		// Spotify authentication is now automatic via callback server - no manual input needed

		// Handle volume adjustment mode
		if a.adjustMode {
			switch {
			case key.Matches(msg, key.NewBinding(key.WithKeys("enter"))):
				// Commit volume adjustment immediately
				return a, a.commitVolumeAdjustment()

			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				// Cancel volume adjustment
				return a, a.cancelVolumeAdjustment()

			case key.Matches(msg, a.keys.VolumeUp):
				// Enter volume adjustment mode and increase volume
				a.enterVolumeAdjustMode()
				a.adjustPendingVolume(1.0)
				return a, a.resetAdjustTimer()

			case key.Matches(msg, a.keys.VolumeDown):
				// Enter volume adjustment mode and decrease volume
				a.enterVolumeAdjustMode()
				a.adjustPendingVolume(-1.0)
				return a, a.resetAdjustTimer()

			// Allow Spotify controls to work even in volume adjustment mode
			case key.Matches(msg, a.keys.SpotifyPlayPause):
				if a.spotifyClient != nil {
					return a, a.spotifyPlayPause()
				} else {
					a.setMessage("Spotify not configured", MessageWarning)
					return a, nil
				}

			case key.Matches(msg, a.keys.SpotifyNext):
				if a.spotifyClient != nil {
					return a, a.spotifyNext()
				} else {
					a.setMessage("Spotify not configured", MessageWarning)
					return a, nil
				}

			case key.Matches(msg, a.keys.SpotifyPrev):
				if a.spotifyClient != nil {
					return a, a.spotifyPrev()
				} else {
					a.setMessage("Spotify not configured", MessageWarning)
					return a, nil
				}

			case key.Matches(msg, a.keys.SpotifyShuffle):
				if a.spotifyClient != nil {
					return a, a.spotifyToggleShuffle()
				} else {
					a.setMessage("Spotify not configured", MessageWarning)
					return a, nil
				}

			default:
				// Any other key cancels adjustment mode
				return a, a.cancelVolumeAdjustment()
			}
		}

		// Handle basic keys that should always work
		switch {
		case key.Matches(msg, a.keys.Quit):
			// Perform comprehensive cleanup
			if err := a.Cleanup(); err != nil {
				log.WithError(err).Debug("Errors occurred during cleanup")
			}
			return a, tea.Quit

		case key.Matches(msg, a.keys.Help):
			a.help.ShowAll = !a.help.ShowAll
			return a, nil

		// Tab navigation
		case key.Matches(msg, a.keys.NextTab):
			a.nextTab()
			return a, nil

		case key.Matches(msg, a.keys.PrevTab):
			a.prevTab()
			return a, nil

		case key.Matches(msg, a.keys.Tab1):
			a.setTab(TabDevice)
			return a, nil

		case key.Matches(msg, a.keys.Tab2):
			a.setTab(TabSpotify)
			return a, nil

		case key.Matches(msg, a.keys.Tab3):
			a.setTab(TabSettings)
			return a, nil

		case key.Matches(msg, a.keys.Tab4):
			a.setTab(TabLogs)
			return a, nil

		case key.Matches(msg, a.keys.Discover):
			// Discovery should work even when not connected
			return a, a.discoverDevices()

		case key.Matches(msg, a.keys.Refresh):
			// Allow refresh even when not connected (will show appropriate message)
			if a.connected {
				return a, a.refreshStatus()
			} else {
				if a.demoMode {
					a.setMessage("Demo mode - NAD device status refresh disabled", MessageInfo)
				} else {
					a.setMessage("Not connected to device", MessageWarning)
				}
				return a, nil
			}

		// Spotify controls (always available if client exists)
		case key.Matches(msg, a.keys.SpotifyToggle):
			a.spotifyEnabled = !a.spotifyEnabled
			if a.spotifyEnabled {
				a.setMessage("Spotify panel enabled", MessageInfo)
			} else {
				a.setMessage("Spotify panel disabled", MessageInfo)
			}
			return a, nil

		case key.Matches(msg, a.keys.SpotifyPlayPause):
			if a.spotifyClient != nil {
				return a, a.spotifyPlayPause()
			} else {
				a.setMessage("Spotify not configured", MessageWarning)
				return a, nil
			}

		case key.Matches(msg, a.keys.SpotifyNext):
			if a.spotifyClient != nil {
				return a, a.spotifyNext()
			} else {
				a.setMessage("Spotify not configured", MessageWarning)
				return a, nil
			}

		case key.Matches(msg, a.keys.SpotifyPrev):
			if a.spotifyClient != nil {
				return a, a.spotifyPrev()
			} else {
				a.setMessage("Spotify not configured", MessageWarning)
				return a, nil
			}

		case key.Matches(msg, a.keys.SpotifyShuffle):
			if a.spotifyClient != nil {
				return a, a.spotifyToggleShuffle()
			} else {
				a.setMessage("Spotify not configured", MessageWarning)
				return a, nil
			}

		case key.Matches(msg, a.keys.SpotifyAuth):
			if a.spotifyClient != nil {
				return a, a.startSpotifyAuth()
			} else {
				a.setMessage("Spotify not configured", MessageWarning)
				return a, nil
			}

		case key.Matches(msg, a.keys.SpotifyDisconnect):
			if a.spotifyClient != nil && a.spotifyClient.IsConnected() {
				return a, a.spotifyDisconnect()
			} else {
				a.setMessage("Spotify not connected", MessageWarning)
				return a, nil
			}

		// New Spotify device management controls
		case key.Matches(msg, a.keys.SpotifyDevices):
			if a.spotifyClient != nil && a.spotifyClient.IsConnected() {
				return a, a.spotifyListDevices()
			} else {
				a.setMessage("Spotify not connected", MessageWarning)
				return a, nil
			}

		case key.Matches(msg, a.keys.SpotifyTransfer):
			if a.spotifyClient != nil && a.spotifyClient.IsConnected() {
				if !a.spotifyDeviceMode {
					// Enter device selection mode - always refresh devices first
					a.spotifyDeviceMode = true
					a.setMessage("Loading Spotify devices...", MessageInfo)
					return a, a.spotifyListDevices()
				} else {
					// Already in device mode, transfer to selected device
					return a, a.spotifyTransferToSelected()
				}
			} else {
				a.setMessage("Spotify not connected", MessageWarning)
				return a, nil
			}

		case key.Matches(msg, a.keys.SpotifyDeviceSelect):
			if a.spotifyDeviceMode && len(a.spotifyDevices) > 0 {
				return a, a.spotifyTransferToSelected()
			}
			return a, nil
		}

		// Handle device control keys (only when connected)
		if a.connected {
			switch {
			case key.Matches(msg, a.keys.Power):
				return a, a.togglePower()

			case key.Matches(msg, a.keys.Mute):
				return a, a.toggleMute()

			case key.Matches(msg, a.keys.VolumeUp):
				// Enter volume adjustment mode and increase volume
				a.enterVolumeAdjustMode()
				a.adjustPendingVolume(1.0)
				return a, a.resetAdjustTimer()

			case key.Matches(msg, a.keys.VolumeDown):
				// Enter volume adjustment mode and decrease volume
				a.enterVolumeAdjustMode()
				a.adjustPendingVolume(-1.0)
				return a, a.resetAdjustTimer()

			case key.Matches(msg, a.keys.VolumeSet):
				// Enter volume input mode
				a.inputMode = true
				a.volumeInput.Focus()
				a.setMessage("Enter volume level (-80 to +10 dB):", MessageInfo)
				return a, textinput.Blink

			case key.Matches(msg, a.keys.Left):
				return a, a.prevSource()

			case key.Matches(msg, a.keys.Right):
				return a, a.nextSource()

			case key.Matches(msg, a.keys.Up):
				// Handle differently if we're in the logs tab
				if a.currentTab == TabLogs {
					a.scrollLogsUp()
					return a, nil
				}
				// Handle device selection navigation
				if a.spotifyDeviceMode && a.currentTab == TabSpotify {
					a.spotifyDeviceSelectionUp()
					return a, nil
				}
				return a, a.brightnessUp()

			case key.Matches(msg, a.keys.Down):
				// Handle differently if we're in the logs tab
				if a.currentTab == TabLogs {
					a.scrollLogsDown()
					return a, nil
				}
				// Handle device selection navigation
				if a.spotifyDeviceMode && a.currentTab == TabSpotify {
					a.spotifyDeviceSelectionDown()
					return a, nil
				}
				return a, a.brightnessDown()

			// Add Esc key handling for device selection mode
			case key.Matches(msg, key.NewBinding(key.WithKeys("esc"))):
				if a.spotifyDeviceMode {
					a.spotifyDeviceMode = false
					a.setMessage("Device selection cancelled", MessageInfo)
					return a, nil
				}
				// Perform comprehensive cleanup
				if err := a.Cleanup(); err != nil {
					log.WithError(err).Debug("Errors occurred during cleanup")
				}
				return a, tea.Quit
			}
		} else {
			// Give feedback when trying to use device controls while not connected
			switch {
			case key.Matches(msg, a.keys.Power),
				key.Matches(msg, a.keys.Mute),
				key.Matches(msg, a.keys.VolumeUp),
				key.Matches(msg, a.keys.VolumeDown),
				key.Matches(msg, a.keys.VolumeSet),
				key.Matches(msg, a.keys.Left),
				key.Matches(msg, a.keys.Right):
				if a.demoMode {
					a.setMessage("Demo mode - NAD device controls disabled (try Spotify controls with 'a')", MessageInfo)
				} else {
					a.setMessage("Connect to device first (press 'd' to discover)", MessageWarning)
				}
				return a, nil
			case key.Matches(msg, a.keys.Up):
				// Handle logs scrolling even when not connected
				if a.currentTab == TabLogs {
					a.scrollLogsUp()
					return a, nil
				}
				if a.demoMode {
					a.setMessage("Demo mode - NAD device controls disabled (try Spotify controls with 'a')", MessageInfo)
				} else {
					a.setMessage("Connect to device first (press 'd' to discover)", MessageWarning)
				}
				return a, nil
			case key.Matches(msg, a.keys.Down):
				// Handle logs scrolling even when not connected
				if a.currentTab == TabLogs {
					a.scrollLogsDown()
					return a, nil
				}
				if a.demoMode {
					a.setMessage("Demo mode - NAD device controls disabled (try Spotify controls with 'a')", MessageInfo)
				} else {
					a.setMessage("Connect to device first (press 'd' to discover)", MessageWarning)
				}
				return a, nil
			}
		}

	case messageMsg:
		a.setMessage(msg.text, msg.msgType)
		return a, a.listenForResults()

	case tickMsg:
		// Update spinner
		a.spinnerIndex = (a.spinnerIndex + 1) % len(spinnerFrames)
		a.spinner = spinnerFrames[a.spinnerIndex]

		if a.connected && a.autoRefresh && time.Since(a.lastUpdate) > 10*time.Second {
			a.queueCommand(CmdRefreshStatus, nil)
		}

		// Also refresh Spotify status periodically
		if a.spotifyClient != nil && a.spotifyClient.IsConnected() && time.Since(a.lastUpdate) > 5*time.Second {
			a.queueCommand(CmdSpotifyRefresh, nil)
		}

		return a, a.tickCmd()

	case volumeAdjustTimeoutMsg:
		// Auto-commit volume adjustment after timeout
		if a.adjustMode {
			return a, a.commitVolumeAdjustment()
		}
		return a, nil

	case deviceConnectedMsg:
		a.device = msg.device
		a.connected = true
		a.connecting = false
		a.status.IP = msg.device.IP.String()
		a.setMessage("Connected to NAD device!", MessageSuccess)
		// Queue a status refresh after successful connection
		a.queueCommand(CmdRefreshStatus, nil)
		return a, a.listenForResults()

	case deviceErrorMsg:
		a.connected = false
		a.connecting = false
		a.setMessage(fmt.Sprintf("Connection failed: %v", msg.err), MessageError)
		return a, a.listenForResults()

	case statusUpdateMsg:
		a.status = msg.status
		a.lastUpdate = time.Now()
		// Update progress bars
		a.volumeBar.SetPercent((a.status.Volume + 80) / 90)          // Volume range -80 to +10
		a.brightnessBar.SetPercent(float64(a.status.Brightness) / 3) // Brightness 0-3
		return a, a.listenForResults()

	case spotifyUpdateMsg:
		a.spotifyState = msg.state
		a.spotifyConnected = a.spotifyClient != nil && a.spotifyClient.IsConnected()

		// If state is nil but client is connected, this means we should refresh
		if a.spotifyState == nil && a.spotifyConnected {
			// Queue a Spotify refresh to get current playback state
			a.queueCommand(CmdSpotifyRefresh, nil)
		}

		// Update Spotify progress bar
		if a.spotifyState != nil && a.spotifyState.Track.Duration > 0 {
			progressPercent := float64(a.spotifyState.Track.Progress) / float64(a.spotifyState.Track.Duration)
			a.spotifyProgress.SetPercent(progressPercent)
		}
		return a, a.listenForResults()

	case spotifyDevicesUpdateMsg:
		a.spotifyDevices = msg.devices
		a.spotifyDevicesLoaded = true
		// Reset selection to first device or maintain current selection if valid
		if len(a.spotifyDevices) > 0 {
			if a.spotifyDeviceSelection >= len(a.spotifyDevices) {
				a.spotifyDeviceSelection = 0
			}
		} else {
			a.spotifyDeviceSelection = 0
		}

		if len(a.spotifyDevices) == 0 {
			a.setMessage("No Spotify devices found. Make sure Spotify is open on at least one device.", MessageWarning)
			a.spotifyDeviceMode = false // Exit device mode if no devices
		} else {
			if a.spotifyDeviceMode {
				// Show device selection mode message after successful refresh
				selectedDevice := a.spotifyDevices[a.spotifyDeviceSelection]
				a.setMessage(fmt.Sprintf("Device selection mode - Selected: %s (%s) - Use ↑↓ to navigate, Enter to select, Esc to cancel", selectedDevice.Name, selectedDevice.Type), MessageInfo)
			} else {
				a.setMessage(fmt.Sprintf("Found %d Spotify device(s)", len(a.spotifyDevices)), MessageSuccess)
			}
		}
		return a, a.listenForResults()
	}

	return a, nil
}

// View renders the application
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "Loading..."
	}

	var sections []string

	// Header - make it span the full width
	header := a.renderHeader()
	sections = append(sections, header)

	// Tab bar (if tabs enabled)
	if a.tabsEnabled {
		tabBar := a.renderTabBar()
		sections = append(sections, tabBar)
	}

	// Calculate available height for main content
	// Reserve space for header (2 lines), tabs (2 lines), message (3 lines), help (2-4 lines), and margins
	headerHeight := 4 // Header takes about 4 lines with margins
	tabHeight := 2    // Tab bar height
	if !a.tabsEnabled {
		tabHeight = 0
	}
	messageHeight := 3                                                          // Message area
	helpHeight := 4                                                             // Help area
	reservedHeight := headerHeight + tabHeight + messageHeight + helpHeight + 2 // +2 for margins
	availableHeight := a.height - reservedHeight

	// Ensure we have minimum height
	if availableHeight < 10 {
		availableHeight = 10
	}

	// Main content based on current tab
	var mainContent string
	switch a.currentTab {
	case TabDevice:
		mainContent = a.renderDeviceTab(availableHeight)
	case TabSpotify:
		mainContent = a.renderSpotifyTab(availableHeight)
	case TabSettings:
		mainContent = a.renderSettingsTab(availableHeight)
	case TabLogs:
		mainContent = a.renderLogsTab(availableHeight)
	default:
		mainContent = a.renderDeviceTab(availableHeight)
	}

	sections = append(sections, mainContent)

	// Volume input (if in input mode)
	if a.inputMode {
		inputWidth := a.width - 20
		if inputWidth > 60 {
			inputWidth = 60
		}

		inputStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			Margin(1, 1).
			Width(inputWidth)

		volumeInputPanel := inputStyle.Render(
			labelStyle.Render("Set Volume") + "\n\n" +
				a.volumeInput.View() + "\n\n" +
				mutedTextStyle.Render("Press Enter to confirm, Esc to cancel"),
		)
		sections = append(sections, volumeInputPanel)
	}

	// Spotify authentication (if in auth mode)
	if a.spotifyAuthMode {
		authWidth := a.width - 20
		if authWidth > 80 {
			authWidth = 80
		}

		authStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(accentColor).
			Padding(1, 2).
			Margin(1, 1).
			Width(authWidth)

		var statusMessage string
		if strings.Contains(a.message, "manually open") {
			// Browser opening failed, show URL
			statusMessage = warningTextStyle.Render("⚠️  Browser opening failed") + "\n" +
				mutedTextStyle.Render("Please manually open:") + "\n" +
				primaryTextStyle.Render(a.spotifyAuthURL) + "\n"
		} else {
			// Browser opened successfully
			statusMessage = successTextStyle.Render("✓ Browser opened automatically!") + "\n"
		}

		spotifyAuthPanel := authStyle.Render(
			labelStyle.Render("🎵 Spotify Authentication") + "\n\n" +
				statusMessage + "\n" +
				mutedTextStyle.Render("1. Authorize the app in your browser") + "\n" +
				mutedTextStyle.Render("2. After authorization, you'll be redirected to:") + "\n" +
				mutedTextStyle.Render("   http://localhost:8888/callback?code=...") + "\n" +
				mutedTextStyle.Render("3. Copy the 'code' parameter value and paste below:") + "\n\n" +
				a.spotifyAuthInput.View() + "\n\n" +
				mutedTextStyle.Render("Press Enter to confirm, Esc to cancel"),
		)
		sections = append(sections, spotifyAuthPanel)
	}

	// Volume adjustment mode indicator
	if a.adjustMode {
		adjustWidth := a.width - 20
		if adjustWidth > 60 {
			adjustWidth = 60
		}

		adjustStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(warningColor).
			Padding(1, 2).
			Margin(1, 1).
			Width(adjustWidth)

		adjustPanel := adjustStyle.Render(
			labelStyle.Render("Volume Adjustment Mode") + "\n\n" +
				fmt.Sprintf("Current: %.1f dB", a.pendingVolume) + "\n" +
				mutedTextStyle.Render("Use +/- to adjust, Enter to apply, Esc to cancel") + "\n" +
				mutedTextStyle.Render("Auto-applies after 2 seconds of inactivity"),
		)
		sections = append(sections, adjustPanel)
	}

	// Message area - make it responsive
	if a.message != "" {
		sections = append(sections, a.renderMessage())
	}

	// Help - make it span the available width
	sections = append(sections, a.renderHelp())

	// Join all sections and ensure it fits in terminal height
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)

	// Truncate content if it exceeds terminal height
	lines := strings.Split(content, "\n")
	if len(lines) > a.height-1 {
		lines = lines[:a.height-1]
		content = strings.Join(lines, "\n")
	}

	return content
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
			Width(36)

	connectedPanelStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(successColor).
				Padding(1, 2).
				Margin(0, 1, 1, 0).
				Width(36)

	errorPanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(errorColor).
			Padding(1, 2).
			Margin(0, 1, 1, 0).
			Width(36)

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

	// Use the full terminal width minus small margins
	titleWidth := a.width - 4
	if titleWidth < 50 {
		titleWidth = 50
	}

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(primaryColor).
		Padding(0, 2).
		Margin(0, 0, 1, 0).
		Bold(true).
		Width(titleWidth).
		Align(lipgloss.Center)

	header := headerStyle.Render(title)
	sub := headerStyle.Render(subtitle)

	return lipgloss.JoinVertical(lipgloss.Center, header, sub)
}

func (a *App) renderTabBar() string {
	// Use the full width for tabs
	totalWidth := a.width - 4 // Leave small margins
	tabWidth := totalWidth / len(a.tabs)

	// Ensure minimum tab width
	if tabWidth < 12 {
		tabWidth = 12
	}

	var tabs []string

	for _, tab := range a.tabs {
		var style lipgloss.Style

		if tab.ID == a.currentTab {
			// Active tab style
			style = lipgloss.NewStyle().
				Foreground(lipgloss.Color("15")).
				Background(primaryColor).
				Padding(0, 1).
				Margin(0, 0).
				Width(tabWidth).
				Align(lipgloss.Center).
				Bold(true)
		} else {
			// Inactive tab style
			style = lipgloss.NewStyle().
				Foreground(mutedColor).
				Background(bgColor).
				Padding(0, 1).
				Margin(0, 0).
				Width(tabWidth).
				Align(lipgloss.Center)
		}

		tabContent := fmt.Sprintf("%s %s (%s)", tab.Icon, tab.Name, tab.ShortKey)
		tabs = append(tabs, style.Render(tabContent))
	}

	// Center the tab bar
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// Add small margins to center it
	return lipgloss.NewStyle().
		Margin(0, 2).
		Render(tabBar)
}

func (a *App) renderDeviceTab(availableHeight int) string {
	// Calculate responsive panel width based on terminal width
	// Use most of the available width, leaving small margins
	totalWidth := a.width - 4      // Leave 4 chars total margin (2 on each side)
	panelWidth := totalWidth/2 - 2 // Split into two columns with gap

	// Ensure minimum viable width
	if panelWidth < 25 {
		// Terminal too narrow for side-by-side, stack vertically
		panelWidth = totalWidth
		return a.renderDeviceTabVertical(availableHeight, panelWidth)
	}

	leftPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Margin(0, 1, 1, 0).
		Width(panelWidth)

	leftConnectedPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(successColor).
		Padding(1, 2).
		Margin(0, 1, 1, 0).
		Width(panelWidth)

	leftErrorPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(errorColor).
		Padding(1, 2).
		Margin(0, 1, 1, 0).
		Width(panelWidth)

	rightPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Margin(0, 1, 1, 0).
		Width(panelWidth)

	var leftPanels []string
	var rightPanels []string
	leftHeight := 0
	rightHeight := 0

	// LEFT COLUMN - Connection and Device Info

	// Connection Status Panel (highest priority)
	var connectionPanel string
	if a.connecting {
		status := fmt.Sprintf("%s Connecting...", a.spinner)
		connectionPanel = leftPanelStyle.Render(
			labelStyle.Render("Connection Status") + "\n\n" +
				warningTextStyle.Render(status),
		)
	} else if a.connected {
		status := fmt.Sprintf("🟢 Connected to %s", a.status.IP)
		connectionPanel = leftConnectedPanelStyle.Render(
			labelStyle.Render("Connection Status") + "\n\n" +
				successTextStyle.Render(status),
		)
	} else {
		status := "🔴 Disconnected\n\nPress 'd' to discover devices"
		connectionPanel = leftErrorPanelStyle.Render(
			labelStyle.Render("Connection Status") + "\n\n" +
				errorTextStyle.Render(status),
		)
	}

	panelHeight := strings.Count(connectionPanel, "\n") + 1
	if leftHeight+panelHeight <= availableHeight {
		leftPanels = append(leftPanels, connectionPanel)
		leftHeight += panelHeight
	}

	// Device Info Panel (if connected and space available)
	if a.connected && a.status.Model != "" && leftHeight < availableHeight-8 {
		deviceInfo := leftPanelStyle.Render(
			labelStyle.Render("Device Information") + "\n\n" +
				fmt.Sprintf("Model: %s\n", valueStyle.Render(a.status.Model)) +
				fmt.Sprintf("IP: %s", valueStyle.Render(a.status.IP)),
		)

		panelHeight = strings.Count(deviceInfo, "\n") + 2 // +2 for spacing
		if leftHeight+panelHeight <= availableHeight {
			leftPanels = append(leftPanels, deviceInfo)
			leftHeight += panelHeight
		}
	}

	// Command Queue Status Panel (if space available)
	if leftHeight < availableHeight-8 {
		queueLen := a.commandQueue.Len()
		var queueStatus string
		var queuePanel string

		if queueLen > 0 {
			if a.processing {
				queueStatus = fmt.Sprintf("🔄 Processing (%d queued)", queueLen)
			} else {
				queueStatus = fmt.Sprintf("⏳ %d commands queued", queueLen)
			}
			queuePanel = leftPanelStyle.Render(
				labelStyle.Render("Command Queue") + "\n\n" +
					accentTextStyle.Render(queueStatus),
			)
		} else {
			if a.processing {
				queueStatus = "🔄 Processing..."
			} else {
				queueStatus = "✓ Queue empty"
			}
			queuePanel = leftPanelStyle.Render(
				labelStyle.Render("Command Queue") + "\n\n" +
					mutedTextStyle.Render(queueStatus),
			)
		}

		panelHeight = strings.Count(queuePanel, "\n") + 2 // +2 for spacing
		if leftHeight+panelHeight <= availableHeight {
			leftPanels = append(leftPanels, queuePanel)
		}
	}

	// RIGHT COLUMN - Device Controls (if connected)

	if a.connected {
		// Power Status Panel (highest priority)
		var powerStatus string
		if a.status.Power == "On" {
			powerStatus = powerOnStyle.Render(" POWER ON ")
		} else {
			powerStatus = powerOffStyle.Render(" POWER OFF ")
		}

		powerPanel := rightPanelStyle.Render(
			labelStyle.Render("Power Status") + "\n\n" +
				powerStatus + "\n\n" +
				mutedTextStyle.Render("Press 'p' to toggle"),
		)

		panelHeight := strings.Count(powerPanel, "\n") + 1
		if rightHeight+panelHeight <= availableHeight {
			rightPanels = append(rightPanels, powerPanel)
			rightHeight += panelHeight
		}

		// Audio Controls Panel (high priority)
		if rightHeight < availableHeight-10 {
			var muteStatus string
			if a.status.Mute == "On" {
				muteStatus = errorTextStyle.Render("🔇 MUTED")
			} else {
				muteStatus = successTextStyle.Render("🔊 UNMUTED")
			}

			// Show pending volume if in adjustment mode, otherwise current volume
			var volumeDisplay string
			var volumeBar string
			if a.adjustMode {
				volumeDisplay = fmt.Sprintf("%.1f dB (adjusting...)", a.pendingVolume)
				volumeBar = a.volumeBar.ViewAs((a.pendingVolume + 80) / 90)
			} else {
				volumeDisplay = a.status.VolumeStr
				volumeBar = a.volumeBar.ViewAs((a.status.Volume + 80) / 90)
			}

			audioPanel := rightPanelStyle.Render(
				labelStyle.Render("Audio Controls") + "\n\n" +
					fmt.Sprintf("Volume: %s\n", valueStyle.Render(volumeDisplay)) +
					volumeBar + "\n\n" +
					fmt.Sprintf("Source: %s\n", valueStyle.Render(a.status.Source)) +
					fmt.Sprintf("Mute: %s", muteStatus),
			)

			panelHeight = strings.Count(audioPanel, "\n") + 2 // +2 for spacing
			if rightHeight+panelHeight <= availableHeight {
				rightPanels = append(rightPanels, audioPanel)
				rightHeight += panelHeight
			}
		}

		// Display Controls Panel (medium priority)
		if rightHeight < availableHeight-8 {
			brightnessBar := a.brightnessBar.ViewAs(float64(a.status.Brightness) / 3)

			displayPanel := rightPanelStyle.Render(
				labelStyle.Render("Display Controls") + "\n\n" +
					fmt.Sprintf("Brightness: %s\n", valueStyle.Render(a.status.BrightnessStr)) +
					brightnessBar + "\n\n" +
					mutedTextStyle.Render("Use ↑↓ keys to adjust"),
			)

			panelHeight = strings.Count(displayPanel, "\n") + 2 // +2 for spacing
			if rightHeight+panelHeight <= availableHeight {
				rightPanels = append(rightPanels, displayPanel)
			}
		}
	} else {
		// Not connected - show help panel
		helpPanel := rightPanelStyle.Render(
			labelStyle.Render("Device Controls") + "\n\n" +
				mutedTextStyle.Render("Connect to a device to see controls") + "\n\n" +
				mutedTextStyle.Render("Available commands:") + "\n" +
				mutedTextStyle.Render("d - Discover devices") + "\n" +
				mutedTextStyle.Render("r - Refresh status") + "\n" +
				mutedTextStyle.Render("? - Toggle help"),
		)
		rightPanels = append(rightPanels, helpPanel)
	}

	// Combine left and right columns
	leftColumn := strings.Join(leftPanels, "\n")
	rightColumn := strings.Join(rightPanels, "\n")

	// Create side-by-side layout
	return lipgloss.JoinHorizontal(lipgloss.Top, leftColumn, rightColumn)
}

// renderDeviceTabVertical renders the device tab in vertical layout for narrow terminals
func (a *App) renderDeviceTabVertical(availableHeight int, panelWidth int) string {
	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Margin(0, 1, 1, 0).
		Width(panelWidth)

	connectedPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(successColor).
		Padding(1, 2).
		Margin(0, 1, 1, 0).
		Width(panelWidth)

	errorPanelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(errorColor).
		Padding(1, 2).
		Margin(0, 1, 1, 0).
		Width(panelWidth)

	var panels []string
	currentHeight := 0

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

	panelHeight := strings.Count(connectionPanel, "\n") + 1
	if currentHeight+panelHeight <= availableHeight {
		panels = append(panels, connectionPanel)
		currentHeight += panelHeight
	}

	// Add other panels if space allows and device is connected
	if a.connected && currentHeight < availableHeight-8 {
		// Power and audio in one combined panel for vertical layout
		var powerStatus string
		if a.status.Power == "On" {
			powerStatus = powerOnStyle.Render(" POWER ON ")
		} else {
			powerStatus = powerOffStyle.Render(" POWER OFF ")
		}

		var muteStatus string
		if a.status.Mute == "On" {
			muteStatus = errorTextStyle.Render("🔇 MUTED")
		} else {
			muteStatus = successTextStyle.Render("🔊 UNMUTED")
		}

		controlPanel := panelStyle.Render(
			labelStyle.Render("Device Controls") + "\n\n" +
				fmt.Sprintf("Power: %s\n", powerStatus) +
				fmt.Sprintf("Volume: %s\n", valueStyle.Render(a.status.VolumeStr)) +
				fmt.Sprintf("Source: %s\n", valueStyle.Render(a.status.Source)) +
				fmt.Sprintf("Mute: %s", muteStatus),
		)

		panelHeight = strings.Count(controlPanel, "\n") + 2
		if currentHeight+panelHeight <= availableHeight {
			panels = append(panels, controlPanel)
		}
	}

	return strings.Join(panels, "\n")
}

func (a *App) renderSpotifyTab(availableHeight int) string {
	// Calculate responsive panel width based on terminal width
	// Use most of the available width, leaving small margins
	totalWidth := a.width - 4 // Leave 4 chars total margin (2 on each side)
	panelWidth := totalWidth

	// Ensure minimum viable width
	if panelWidth < 40 {
		panelWidth = 40
	}

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Margin(0, 1, 1, 0).
		Width(panelWidth)

	var panels []string
	currentHeight := 0

	// Spotify Connection Status Panel (highest priority)
	var connectionPanel string
	if a.spotifyClient == nil {
		connectionPanel = panelStyle.Render(
			labelStyle.Render("🎵 Spotify Status") + "\n\n" +
				errorTextStyle.Render("Not Configured") + "\n\n" +
				mutedTextStyle.Render("Configure spotify.client_id in settings\nto enable Spotify integration"),
		)
	} else if !a.spotifyClient.IsConnected() {
		connectionPanel = panelStyle.Render(
			labelStyle.Render("🎵 Spotify Status") + "\n\n" +
				warningTextStyle.Render("Not Connected") + "\n\n" +
				mutedTextStyle.Render("Press 'a' to authenticate with Spotify"),
		)
	} else {
		connectionPanel = panelStyle.Render(
			labelStyle.Render("🎵 Spotify Status") + "\n\n" +
				successTextStyle.Render("✓ Connected") + "\n\n" +
				mutedTextStyle.Render("Press 'x' to disconnect"),
		)
	}

	panelHeight := strings.Count(connectionPanel, "\n") + 1
	if currentHeight+panelHeight <= availableHeight {
		panels = append(panels, connectionPanel)
		currentHeight += panelHeight
	}

	// Spotify Playback Panel (if connected and space available)
	if a.spotifyClient != nil && a.spotifyClient.IsConnected() && currentHeight < availableHeight-15 {
		spotifyPanel := a.renderSpotifyPanel(panelStyle)
		panelHeight = strings.Count(spotifyPanel, "\n") + 2 // +2 for spacing
		if currentHeight+panelHeight <= availableHeight {
			panels = append(panels, spotifyPanel)
			currentHeight += panelHeight
		}
	}

	// Spotify Controls Help Panel (if space available)
	if currentHeight < availableHeight-10 {
		var controlsText string
		if a.spotifyClient != nil && a.spotifyClient.IsConnected() {
			controlsText = mutedTextStyle.Render("Controls:") + "\n" +
				mutedTextStyle.Render("space - Play/Pause") + "\n" +
				mutedTextStyle.Render("n - Next Track") + "\n" +
				mutedTextStyle.Render("b - Previous Track") + "\n" +
				mutedTextStyle.Render("h - Toggle Shuffle") + "\n" +
				mutedTextStyle.Render("y - Device Selection & Cast") + "\n" +
				mutedTextStyle.Render("r - Refresh Status") + "\n" +
				mutedTextStyle.Render("x - Disconnect")
		} else {
			controlsText = mutedTextStyle.Render("Connect to Spotify to see controls")
		}

		controlsPanel := panelStyle.Render(
			labelStyle.Render("Keyboard Shortcuts") + "\n\n" +
				controlsText,
		)

		panelHeight = strings.Count(controlsPanel, "\n") + 2 // +2 for spacing
		if currentHeight+panelHeight <= availableHeight {
			panels = append(panels, controlsPanel)
			currentHeight += panelHeight
		}
	}

	// Spotify Devices Panel (if devices are loaded and space available)
	if a.spotifyDevicesLoaded && currentHeight < availableHeight-8 {
		devicePanel := a.renderSpotifyDevicesPanel(panelStyle)
		panelHeight = strings.Count(devicePanel, "\n") + 2 // +2 for spacing
		if currentHeight+panelHeight <= availableHeight {
			panels = append(panels, devicePanel)
		}
	}

	if len(panels) == 0 {
		// Fallback if no panels fit
		return panelStyle.Render(
			labelStyle.Render("🎵 Spotify") + "\n\n" +
				mutedTextStyle.Render("Terminal too small to display content"),
		)
	}

	return strings.Join(panels, "\n")
}

func (a *App) renderSettingsTab(availableHeight int) string {
	// Calculate responsive panel width based on terminal width
	// Use most of the available width, leaving small margins
	totalWidth := a.width - 4 // Leave 4 chars total margin (2 on each side)
	panelWidth := totalWidth

	// Ensure minimum viable width
	if panelWidth < 40 {
		panelWidth = 40
	}

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Margin(0, 1, 1, 0).
		Width(panelWidth)

	var panels []string
	currentHeight := 0

	// Application Settings Panel
	appSettings := panelStyle.Render(
		labelStyle.Render("⚙️ Application Settings") + "\n\n" +
			fmt.Sprintf("Version: %s\n", valueStyle.Render(version.Version)) +
			fmt.Sprintf("Config File: %s\n", valueStyle.Render("~/.nadctl.yaml")) +
			fmt.Sprintf("Cache File: %s\n", valueStyle.Render("~/.nadctl_cache.json")) +
			fmt.Sprintf("Auto Refresh: %s", valueStyle.Render(fmt.Sprintf("%t", a.autoRefresh))),
	)

	panelHeight := strings.Count(appSettings, "\n") + 1
	if currentHeight+panelHeight <= availableHeight {
		panels = append(panels, appSettings)
		currentHeight += panelHeight
	}

	// Device Settings Panel (if space available)
	if currentHeight < availableHeight-10 {
		var deviceIP string
		if a.connected && a.device != nil {
			deviceIP = a.device.IP.String()
		} else {
			deviceIP = viper.GetString("ip")
			if deviceIP == "" {
				deviceIP = "Auto-discovery"
			}
		}

		deviceSettings := panelStyle.Render(
			labelStyle.Render("🎛️ Device Settings") + "\n\n" +
				fmt.Sprintf("Device IP: %s\n", valueStyle.Render(deviceIP)) +
				fmt.Sprintf("Discovery Enabled: %s\n", valueStyle.Render("true")) +
				fmt.Sprintf("Cache TTL: %s", valueStyle.Render("5 minutes")),
		)

		panelHeight = strings.Count(deviceSettings, "\n") + 2 // +2 for spacing
		if currentHeight+panelHeight <= availableHeight {
			panels = append(panels, deviceSettings)
			currentHeight += panelHeight
		}
	}

	// Spotify Settings Panel (if space available)
	if currentHeight < availableHeight-10 {
		var spotifyStatus string
		clientID := viper.GetString("spotify.client_id")
		if clientID != "" {
			if a.spotifyClient != nil && a.spotifyClient.IsConnected() {
				spotifyStatus = "Connected"
			} else {
				spotifyStatus = "Configured but not connected"
			}
		} else {
			spotifyStatus = "Not configured"
		}

		var clientIDDisplay string
		if clientID != "" {
			// Show only first 8 characters for security
			if len(clientID) > 8 {
				clientIDDisplay = clientID[:8] + "..."
			} else {
				clientIDDisplay = clientID
			}
		} else {
			clientIDDisplay = "Not set"
		}

		spotifySettings := panelStyle.Render(
			labelStyle.Render("🎵 Spotify Settings") + "\n\n" +
				fmt.Sprintf("Status: %s\n", valueStyle.Render(spotifyStatus)) +
				fmt.Sprintf("Client ID: %s\n", valueStyle.Render(clientIDDisplay)) +
				fmt.Sprintf("Redirect URL: %s\n", valueStyle.Render(viper.GetString("spotify.redirect_url"))) +
				fmt.Sprintf("Token Cached: %s", valueStyle.Render(fmt.Sprintf("%t", a.spotifyClient != nil && a.spotifyClient.IsConnected()))),
		)

		panelHeight = strings.Count(spotifySettings, "\n") + 2 // +2 for spacing
		if currentHeight+panelHeight <= availableHeight {
			panels = append(panels, spotifySettings)
			currentHeight += panelHeight
		}
	}

	// Configuration Help Panel (if space available)
	if currentHeight < availableHeight-8 {
		configHelp := panelStyle.Render(
			labelStyle.Render("📝 Configuration") + "\n\n" +
				mutedTextStyle.Render("To configure Spotify:") + "\n" +
				mutedTextStyle.Render("1. Create Spotify app at developer.spotify.com") + "\n" +
				mutedTextStyle.Render("2. Set redirect URI to: http://localhost:8888/callback") + "\n" +
				mutedTextStyle.Render("3. Add to ~/.nadctl.yaml:") + "\n" +
				mutedTextStyle.Render("   spotify:") + "\n" +
				mutedTextStyle.Render("     client_id: your_client_id_here") + "\n\n" +
				mutedTextStyle.Render("To set device IP:") + "\n" +
				mutedTextStyle.Render("   ip: 192.168.1.100"),
		)

		panelHeight = strings.Count(configHelp, "\n") + 2 // +2 for spacing
		if currentHeight+panelHeight <= availableHeight {
			panels = append(panels, configHelp)
		}
	}

	if len(panels) == 0 {
		// Fallback if no panels fit
		return panelStyle.Render(
			labelStyle.Render("⚙️ Settings") + "\n\n" +
				mutedTextStyle.Render("Terminal too small to display content"),
		)
	}

	return strings.Join(panels, "\n")
}

func (a *App) renderLogsTab(availableHeight int) string {
	// Calculate responsive panel width based on terminal width
	// Use most of the available width, leaving small margins
	totalWidth := a.width - 4 // Leave 4 chars total margin (2 on each side)
	panelWidth := totalWidth

	// Ensure minimum viable width
	if panelWidth < 40 {
		panelWidth = 40
	}

	panelStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(1, 2).
		Margin(0, 1, 1, 0).
		Width(panelWidth)

	// Calculate how many log lines we can show
	// Reserve space for panel header, borders, and scroll info
	maxDisplayLines := availableHeight - 6 // 6 lines for header, borders, scroll info

	if maxDisplayLines < 5 {
		maxDisplayLines = 5
	}

	var logContent strings.Builder

	if len(a.logEntries) == 0 {
		logContent.WriteString(mutedTextStyle.Render("No log entries yet.\n\n"))
		logContent.WriteString(mutedTextStyle.Render("Logs will appear here as the application runs.\n"))
		logContent.WriteString(mutedTextStyle.Render("Use ↑↓ to scroll when logs are present."))
	} else {
		// Calculate display range based on scroll position
		totalLogs := len(a.logEntries)

		// Ensure scroll position is valid
		if a.logScrollPos < 0 {
			a.logScrollPos = 0
		}
		if a.logScrollPos >= totalLogs {
			a.logScrollPos = totalLogs - 1
		}

		// Calculate start and end indices for display
		startIdx := a.logScrollPos
		endIdx := startIdx + maxDisplayLines
		if endIdx > totalLogs {
			endIdx = totalLogs
			startIdx = endIdx - maxDisplayLines
			if startIdx < 0 {
				startIdx = 0
			}
		}

		// Display the log entries
		for i := startIdx; i < endIdx; i++ {
			entry := a.logEntries[i]

			// Format timestamp
			timeStr := entry.Timestamp.Format("15:04:05")

			// Color code by log level
			var levelStyle lipgloss.Style
			switch strings.ToUpper(entry.Level) {
			case "ERROR":
				levelStyle = errorTextStyle
			case "WARN", "WARNING":
				levelStyle = warningTextStyle
			case "INFO":
				levelStyle = primaryTextStyle
			case "DEBUG":
				levelStyle = mutedTextStyle
			default:
				levelStyle = mutedTextStyle
			}

			// Format the log line
			levelStr := fmt.Sprintf("[%s]", strings.ToUpper(entry.Level))
			logLine := fmt.Sprintf("%s %s %s",
				mutedTextStyle.Render(timeStr),
				levelStyle.Render(levelStr),
				entry.Message)

			// Add fields if any
			if len(entry.Fields) > 0 {
				var fields []string
				for k, v := range entry.Fields {
					fields = append(fields, fmt.Sprintf("%s=%v", k, v))
				}
				logLine += " " + mutedTextStyle.Render(fmt.Sprintf("{%s}", strings.Join(fields, ", ")))
			}

			logContent.WriteString(logLine + "\n")
		}

		// Add scroll position info
		if totalLogs > maxDisplayLines {
			scrollInfo := fmt.Sprintf("Showing %d-%d of %d (scroll: ↑↓)",
				startIdx+1, endIdx, totalLogs)
			logContent.WriteString("\n" + mutedTextStyle.Render(scrollInfo))
		}
	}

	return panelStyle.Render(
		labelStyle.Render("📜 Application Logs") + "\n\n" +
			logContent.String(),
	)
}

// scrollLogsUp scrolls the logs view up (shows older entries)
func (a *App) scrollLogsUp() {
	if len(a.logEntries) > 0 {
		a.logScrollPos--
		if a.logScrollPos < 0 {
			a.logScrollPos = 0
		}
	}
}

// scrollLogsDown scrolls the logs view down (shows newer entries)
func (a *App) scrollLogsDown() {
	if len(a.logEntries) > 0 {
		a.logScrollPos++
		maxScroll := len(a.logEntries) - 1
		if a.logScrollPos > maxScroll {
			a.logScrollPos = maxScroll
		}
	}
}

// addLogEntry adds a new log entry to the internal log store
func (a *App) addLogEntry(level, message string, fields map[string]interface{}) {
	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Fields:    fields,
	}

	a.logEntries = append(a.logEntries, entry)

	// Limit the number of stored log entries
	if len(a.logEntries) > a.maxLogEntries {
		// Remove oldest entries
		excess := len(a.logEntries) - a.maxLogEntries
		a.logEntries = a.logEntries[excess:]

		// Adjust scroll position accordingly
		a.logScrollPos -= excess
		if a.logScrollPos < 0 {
			a.logScrollPos = 0
		}
	}

	// Auto-scroll to bottom (newest entries) by default
	if a.logScrollPos == len(a.logEntries)-2 || len(a.logEntries) == 1 {
		a.logScrollPos = len(a.logEntries) - 1
	}
}

func (a *App) renderHelpTab(availableHeight int) string {
	if !a.lastUpdate.IsZero() {
		lastUpdateText := mutedTextStyle.Render(fmt.Sprintf("Last update: %s", a.lastUpdate.Format("15:04:05")))
		helpContent := a.help.View(a.keys)
		return lipgloss.JoinVertical(lipgloss.Left, lastUpdateText, helpContent)
	}
	return a.help.View(a.keys)
}

func (a *App) renderMessage() string {
	var style lipgloss.Style
	var icon string

	// Use most of the terminal width for messages
	messageWidth := a.width - 4 // Leave small margins
	if messageWidth < 40 {
		messageWidth = 40
	}

	switch a.messageType {
	case MessageSuccess:
		style = lipgloss.NewStyle().
			Foreground(successColor).
			Border(lipgloss.NormalBorder()).
			BorderForeground(successColor).
			Padding(0, 1).
			Margin(0, 2).
			Width(messageWidth)
		icon = "✓"
	case MessageError:
		style = lipgloss.NewStyle().
			Foreground(errorColor).
			Border(lipgloss.NormalBorder()).
			BorderForeground(errorColor).
			Padding(0, 1).
			Margin(0, 2).
			Width(messageWidth)
		icon = "✗"
	case MessageWarning:
		style = lipgloss.NewStyle().
			Foreground(warningColor).
			Border(lipgloss.NormalBorder()).
			BorderForeground(warningColor).
			Padding(0, 1).
			Margin(0, 2).
			Width(messageWidth)
		icon = "⚠"
	default:
		style = lipgloss.NewStyle().
			Foreground(primaryColor).
			Border(lipgloss.NormalBorder()).
			BorderForeground(primaryColor).
			Padding(0, 1).
			Margin(0, 2).
			Width(messageWidth)
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

// Command processing methods
func (a *App) processCommands() {
	for {
		// Check if there are commands in the queue
		if cmd, hasCmd := a.commandQueue.Next(); hasCmd {
			a.processing = true
			a.executeCommand(cmd)
			a.processing = false

			// Small delay to prevent overwhelming the device
			time.Sleep(100 * time.Millisecond)
		} else {
			// No commands, wait a bit before checking again
			time.Sleep(50 * time.Millisecond)
		}
	}
}

func (a *App) executeCommand(cmd QueuedCommand) {
	// For device control commands, ensure we have a device
	if cmd.Type != CmdDiscoverDevices && cmd.Type != CmdConnectDevice && cmd.Type != CmdRefreshStatus &&
		cmd.Type != CmdSpotifyPlayPause && cmd.Type != CmdSpotifyNext && cmd.Type != CmdSpotifyPrev &&
		cmd.Type != CmdSpotifyToggleShuffle && cmd.Type != CmdSpotifyRefresh && cmd.Type != CmdSpotifyAuth &&
		cmd.Type != CmdSpotifyDisconnect && a.device == nil {
		a.sendResult(messageMsg{text: "No device connected", msgType: MessageError})
		return
	}

	var err error

	switch cmd.Type {
	case CmdPowerToggle:
		err = a.device.PowerToggle()

	case CmdMuteToggle:
		err = a.device.ToggleMute()

	case CmdVolumeSet:
		if volume, ok := cmd.Params["volume"].(float64); ok {
			err = a.device.SetVolume(volume)
		}

	case CmdVolumeUp:
		err = a.device.TuneVolume(nadapi.DirectionUp)

	case CmdVolumeDown:
		err = a.device.TuneVolume(nadapi.DirectionDown)

	case CmdSourceNext:
		_, err = a.device.ToggleSource(nadapi.DirectionUp)

	case CmdSourcePrev:
		_, err = a.device.ToggleSource(nadapi.DirectionDown)

	case CmdBrightnessUp:
		err = a.device.ToggleBrightness(nadapi.DirectionUp)

	case CmdBrightnessDown:
		err = a.device.ToggleBrightness(nadapi.DirectionDown)

	case CmdRefreshStatus:
		// Refresh status is handled differently
		a.refreshStatusSync()
		return

	case CmdDiscoverDevices:
		a.discoverDevicesSync()
		return

	case CmdConnectDevice:
		if ip, ok := cmd.Params["ip"].(string); ok {
			a.connectToDeviceSync(ip)
		}
		return

	// Spotify command execution
	case CmdSpotifyPlayPause:
		if a.spotifyClient != nil && a.spotifyClient.IsConnected() {
			if a.spotifyState != nil && a.spotifyState.IsPlaying {
				err = a.spotifyClient.Pause()
			} else {
				err = a.spotifyClient.Play()
			}
		} else {
			a.sendResult(messageMsg{text: "Spotify not connected", msgType: MessageError})
			return
		}

	case CmdSpotifyNext:
		if a.spotifyClient != nil && a.spotifyClient.IsConnected() {
			err = a.spotifyClient.Next()
		} else {
			a.sendResult(messageMsg{text: "Spotify not connected", msgType: MessageError})
			return
		}

	case CmdSpotifyPrev:
		if a.spotifyClient != nil && a.spotifyClient.IsConnected() {
			err = a.spotifyClient.Previous()
		} else {
			a.sendResult(messageMsg{text: "Spotify not connected", msgType: MessageError})
			return
		}

	case CmdSpotifyToggleShuffle:
		if a.spotifyClient != nil && a.spotifyClient.IsConnected() {
			err = a.spotifyClient.ToggleShuffle()
		} else {
			a.sendResult(messageMsg{text: "Spotify not connected", msgType: MessageError})
			return
		}

	case CmdSpotifyRefresh:
		// Refresh Spotify status
		a.refreshSpotifySync()
		return

	case CmdSpotifyAuth:
		// Complete Spotify authentication with code
		if code, ok := cmd.Params["code"].(string); ok && a.spotifyClient != nil {
			if err := a.spotifyClient.CompleteAuth(code); err != nil {
				a.sendResult(messageMsg{text: fmt.Sprintf("Spotify auth failed: %v", err), msgType: MessageError})
			} else {
				a.sendResult(messageMsg{text: "Spotify authentication successful!", msgType: MessageSuccess})
				// Refresh Spotify status after successful auth
				a.queueCommand(CmdSpotifyRefresh, nil)
			}
		} else {
			a.sendResult(messageMsg{text: "Invalid Spotify auth code", msgType: MessageError})
		}
		return

	case CmdSpotifyDisconnect:
		if a.spotifyClient != nil {
			if err := a.spotifyClient.Disconnect(); err != nil {
				a.sendResult(messageMsg{text: fmt.Sprintf("Failed to disconnect Spotify: %v", err), msgType: MessageError})
			} else {
				a.sendResult(messageMsg{text: "Disconnected from Spotify", msgType: MessageSuccess})
				// Clear the state and connection status
				a.spotifyConnected = false
				a.spotifyState = nil
			}
		} else {
			a.sendResult(messageMsg{text: "Spotify not configured", msgType: MessageError})
		}
		return

	// New Spotify device commands
	case CmdSpotifyListDevices:
		if a.spotifyClient != nil && a.spotifyClient.IsConnected() {
			devices, err := a.spotifyClient.GetAvailableDevices()
			if err != nil {
				a.sendResult(messageMsg{text: fmt.Sprintf("Failed to get devices: %v", err), msgType: MessageError})
			} else {
				a.sendResult(spotifyDevicesUpdateMsg{devices: devices})
			}
		} else {
			a.sendResult(messageMsg{text: "Spotify not connected", msgType: MessageError})
		}
		return

	case CmdSpotifyTransferDevice:
		if a.spotifyClient != nil && a.spotifyClient.IsConnected() {
			if deviceID, ok := cmd.Params["deviceID"].(string); ok {
				deviceName := cmd.Params["deviceName"].(string)
				err := a.spotifyClient.TransferPlaybackToDevice(deviceID, true) // Start playing after transfer
				if err != nil {
					a.sendResult(messageMsg{text: fmt.Sprintf("Failed to transfer to %s: %v", deviceName, err), msgType: MessageError})
				} else {
					a.sendResult(messageMsg{text: fmt.Sprintf("Successfully transferred to %s", deviceName), msgType: MessageSuccess})
					// Refresh Spotify status after transfer
					a.queueCommand(CmdSpotifyRefresh, nil)
				}
			} else {
				a.sendResult(messageMsg{text: "Invalid device transfer parameters", msgType: MessageError})
			}
		} else {
			a.sendResult(messageMsg{text: "Spotify not connected", msgType: MessageError})
		}
		return
	}

	// Handle communication errors with retry logic
	if err != nil {
		if strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "connection") ||
			strings.Contains(err.Error(), "EOF") {

			log.WithError(err).WithField("command", cmd.Type).Debug("Communication error, attempting reconnection")

			// Try to reconnect and retry
			if newDevice, reconnectErr := nadapi.New(a.device.IP.String(), a.device.Port); reconnectErr == nil {
				// Properly close the old connection first
				if oldDevice := a.device; oldDevice != nil {
					if disconnectErr := oldDevice.Disconnect(); disconnectErr != nil {
						log.WithError(disconnectErr).Debug("Error closing old connection during reconnect")
					} else {
						log.Debug("Successfully closed old connection during reconnect")
					}
				}

				a.device = newDevice
				log.WithField("command", cmd.Type).Debug("Reconnected, retrying command")

				// Retry the command once
				a.executeCommand(cmd)
				return
			} else {
				log.WithError(reconnectErr).WithField("command", cmd.Type).Debug("Failed to reconnect")
			}
		}
		log.WithError(err).WithField("command", cmd.Type).Debug("Command failed")
	} else {
		// Command succeeded, queue a status refresh to show the result
		if cmd.Type != CmdRefreshStatus {
			a.commandQueue.Add(QueuedCommand{
				Type:      CmdRefreshStatus,
				Params:    nil,
				ID:        fmt.Sprintf("refresh-%d", time.Now().UnixNano()),
				Timestamp: time.Now(),
			})
		}
	}
}

func (a *App) queueCommand(cmdType CommandType, params map[string]interface{}) {
	cmd := QueuedCommand{
		Type:      cmdType,
		Params:    params,
		ID:        fmt.Sprintf("%d-%d", cmdType, time.Now().UnixNano()),
		Timestamp: time.Now(),
	}
	a.commandQueue.Add(cmd)
}

// Command functions
func (a *App) connectToDevice() tea.Cmd {
	// In demo mode, skip device connection
	if a.demoMode {
		a.setMessage("Demo mode - no NAD device connection required", MessageInfo)
		return nil
	}

	// Try to get IP from config
	ip := viper.GetString("ip")

	if ip == "" {
		// No IP configured, queue discovery first
		a.queueCommand(CmdDiscoverDevices, nil)
		a.setMessage("Discovering devices...", MessageInfo)
	} else {
		// IP configured, queue direct connection
		a.connecting = true
		params := map[string]interface{}{"ip": ip}
		a.queueCommand(CmdConnectDevice, params)
		a.setMessage("Connecting to device...", MessageInfo)
	}

	return nil
}

func (a *App) refreshStatus() tea.Cmd {
	if a.device == nil {
		a.setMessage("No device connected", MessageError)
		return nil
	}

	a.queueCommand(CmdRefreshStatus, nil)
	a.setMessage("Refreshing status...", MessageInfo)
	return nil
}

func (a *App) togglePower() tea.Cmd {
	a.queueCommand(CmdPowerToggle, nil)
	a.setMessage("Power toggle queued", MessageInfo)
	return nil
}

func (a *App) toggleMute() tea.Cmd {
	a.queueCommand(CmdMuteToggle, nil)
	a.setMessage("Mute toggle queued", MessageInfo)
	return nil
}

func (a *App) volumeUp() tea.Cmd {
	a.queueCommand(CmdVolumeUp, nil)
	a.setMessage("Volume up queued", MessageInfo)
	return nil
}

func (a *App) volumeDown() tea.Cmd {
	a.queueCommand(CmdVolumeDown, nil)
	a.setMessage("Volume down queued", MessageInfo)
	return nil
}

func (a *App) nextSource() tea.Cmd {
	a.queueCommand(CmdSourceNext, nil)
	a.setMessage("Source change queued", MessageInfo)
	return nil
}

func (a *App) prevSource() tea.Cmd {
	a.queueCommand(CmdSourcePrev, nil)
	a.setMessage("Source change queued", MessageInfo)
	return nil
}

func (a *App) brightnessUp() tea.Cmd {
	a.queueCommand(CmdBrightnessUp, nil)
	a.setMessage("Brightness up queued", MessageInfo)
	return nil
}

func (a *App) brightnessDown() tea.Cmd {
	a.queueCommand(CmdBrightnessDown, nil)
	a.setMessage("Brightness down queued", MessageInfo)
	return nil
}

func (a *App) setSpecificVolume(volume float64) tea.Cmd {
	params := map[string]interface{}{"volume": volume}
	a.queueCommand(CmdVolumeSet, params)
	a.setMessage(fmt.Sprintf("Volume %.1f dB queued", volume), MessageInfo)
	return nil
}

// Volume adjustment mode methods
func (a *App) adjustPendingVolume(delta float64) {
	a.pendingVolume += delta
	// Clamp to valid range
	if a.pendingVolume < -80 {
		a.pendingVolume = -80
	}
	if a.pendingVolume > 10 {
		a.pendingVolume = 10
	}
	a.setMessage(fmt.Sprintf("Adjusting volume: %.1f dB (Press Enter to apply, Esc to cancel)", a.pendingVolume), MessageInfo)
}

func (a *App) resetAdjustTimer() tea.Cmd {
	// Stop existing timer if any
	if a.adjustTimer != nil {
		a.adjustTimer.Stop()
	}

	// Start new timer for auto-commit after 2 seconds
	a.adjustTimer = time.NewTimer(2 * time.Second)

	return func() tea.Msg {
		<-a.adjustTimer.C
		return volumeAdjustTimeoutMsg{}
	}
}

func (a *App) commitVolumeAdjustment() tea.Cmd {
	if a.adjustTimer != nil {
		a.adjustTimer.Stop()
		a.adjustTimer = nil
	}

	volume := a.pendingVolume
	a.adjustMode = false
	a.pendingVolume = 0
	a.originalVolume = 0

	params := map[string]interface{}{"volume": volume}
	a.queueCommand(CmdVolumeSet, params)
	a.setMessage(fmt.Sprintf("Volume %.1f dB queued", volume), MessageInfo)
	return nil
}

func (a *App) cancelVolumeAdjustment() tea.Cmd {
	if a.adjustTimer != nil {
		a.adjustTimer.Stop()
		a.adjustTimer = nil
	}

	a.adjustMode = false
	a.pendingVolume = 0
	a.originalVolume = 0
	a.setMessage("Volume adjustment cancelled", MessageInfo)
	return nil
}

func (a *App) enterVolumeAdjustMode() {
	if !a.adjustMode {
		a.adjustMode = true
		a.originalVolume = a.status.Volume
		a.pendingVolume = a.status.Volume
	}
}

func (a *App) discoverDevices() tea.Cmd {
	a.queueCommand(CmdDiscoverDevices, nil)
	a.setMessage("Discovering devices...", MessageInfo)
	return nil
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

type volumeAdjustTimeoutMsg struct{}

type spotifyUpdateMsg struct {
	state *spotify.PlaybackState
}

type spotifyDevicesUpdateMsg struct {
	devices []spotify.Device
}

// refreshStatusSync synchronously updates the device status
func (a *App) refreshStatusSync() {
	if a.device == nil {
		return
	}

	status := DeviceStatus{IP: a.device.IP.String()}

	// Helper function to handle command errors and potential reconnection
	handleError := func(operation string, err error) bool {
		if err == nil {
			return false
		}

		// Check if it's a timeout or connection error
		if strings.Contains(err.Error(), "timeout") ||
			strings.Contains(err.Error(), "connection") ||
			strings.Contains(err.Error(), "EOF") {
			log.WithError(err).WithField("operation", operation).Debug("Communication error detected, attempting reconnection")

			// Try to reconnect
			if reconnectErr := a.device.Disconnect(); reconnectErr != nil {
				log.WithError(reconnectErr).Debug("Error during disconnect for recovery")
			}

			// Create new device connection
			newDevice, reconnectErr := nadapi.New(a.device.IP.String(), a.device.Port)
			if reconnectErr != nil {
				log.WithError(reconnectErr).Debug("Failed to reconnect after communication error")
				return true // Indicate error occurred
			}

			a.device = newDevice
			log.Debug("Successfully reconnected after communication error")
			return false // Retry succeeded
		}

		log.WithError(err).WithField("operation", operation).Debug("Non-recoverable error")
		return true // Indicate error occurred
	}

	// Get power state with error handling
	if power, err := a.device.GetPowerState(); handleError("GetPowerState", err) {
		status.Power = "Unknown"
	} else {
		status.Power = power
	}

	// Get volume with error handling
	if volume, err := a.device.GetVolumeFloat(); handleError("GetVolumeFloat", err) {
		status.Volume = -80
		status.VolumeStr = "Unknown"
	} else {
		status.Volume = volume
		status.VolumeStr = fmt.Sprintf("%.1f dB", volume)
	}

	// Get source with error handling
	if source, err := a.device.GetSource(); handleError("GetSource", err) {
		status.Source = "Unknown"
	} else {
		status.Source = source
	}

	// Get mute status with error handling
	if mute, err := a.device.GetMuteStatus(); handleError("GetMuteStatus", err) {
		status.Mute = "Unknown"
	} else {
		status.Mute = mute
	}

	// Get brightness with error handling
	if brightness, err := a.device.GetBrightnessInt(); handleError("GetBrightnessInt", err) {
		status.Brightness = 0
		status.BrightnessStr = "Unknown"
	} else {
		status.Brightness = brightness
		status.BrightnessStr = strconv.Itoa(brightness)
	}

	// Get model with error handling
	if model, err := a.device.GetModel(); handleError("GetModel", err) {
		status.Model = "Unknown"
	} else {
		status.Model = model
	}

	// Send status update message to UI thread
	a.sendResult(statusUpdateMsg{status: status})
}

func (a *App) connectToDeviceSync(ip string) {
	// Send connecting message
	a.sendResult(messageMsg{text: "Connecting to device...", msgType: MessageInfo})

	// Close existing connection if any
	if a.device != nil {
		if err := a.device.Disconnect(); err != nil {
			log.WithError(err).Debug("Error closing existing connection before new connection")
		} else {
			log.Debug("Successfully closed existing connection before new connection")
		}
		a.device = nil
		a.connected = false
	}

	device, err := nadapi.New(ip, "")
	if err != nil {
		a.sendResult(deviceErrorMsg{err: err})
		return
	}
	a.sendResult(deviceConnectedMsg{device: device})
}

func (a *App) discoverDevicesSync() {
	devices, _, err := nadapi.DiscoverDevicesWithCache(30*time.Second, false, nadapi.DefaultCacheTTL)
	if err != nil {
		a.sendResult(messageMsg{text: fmt.Sprintf("Discovery failed: %v", err), msgType: MessageError})
		return
	}

	if len(devices) == 0 {
		a.sendResult(messageMsg{text: "No NAD devices found on the network", msgType: MessageWarning})
		return
	}

	a.sendResult(messageMsg{text: fmt.Sprintf("Found %d device(s)", len(devices)), msgType: MessageSuccess})

	// If not connected and we found devices, try to connect to the first one
	if !a.connected && len(devices) > 0 {
		a.commandQueue.Add(QueuedCommand{
			Type:      CmdConnectDevice,
			Params:    map[string]interface{}{"ip": devices[0].IP},
			ID:        fmt.Sprintf("connect-%d", time.Now().UnixNano()),
			Timestamp: time.Now(),
		})
	}
}

// sendResult sends a message to the result channel in a non-blocking way
func (a *App) sendResult(msg tea.Msg) {
	select {
	case a.resultChan <- msg:
		// Message sent successfully
	default:
		// Channel is full, drop the message to prevent blocking
		log.Debug("Result channel full, dropping message")
	}
}

// Spotify command methods
func (a *App) spotifyPlayPause() tea.Cmd {
	if a.spotifyClient == nil {
		a.setMessage("Spotify not configured", MessageError)
		return nil
	}
	a.queueCommand(CmdSpotifyPlayPause, nil)
	a.setMessage("Spotify play/pause queued", MessageInfo)
	return nil
}

func (a *App) spotifyNext() tea.Cmd {
	if a.spotifyClient == nil {
		a.setMessage("Spotify not configured", MessageError)
		return nil
	}
	a.queueCommand(CmdSpotifyNext, nil)
	a.setMessage("Spotify next track queued", MessageInfo)
	return nil
}

func (a *App) spotifyPrev() tea.Cmd {
	if a.spotifyClient == nil {
		a.setMessage("Spotify not configured", MessageError)
		return nil
	}
	a.queueCommand(CmdSpotifyPrev, nil)
	a.setMessage("Spotify previous track queued", MessageInfo)
	return nil
}

func (a *App) spotifyToggleShuffle() tea.Cmd {
	if a.spotifyClient == nil {
		a.setMessage("Spotify not configured", MessageError)
		return nil
	}
	a.queueCommand(CmdSpotifyToggleShuffle, nil)
	a.setMessage("Spotify shuffle toggle queued", MessageInfo)
	return nil
}

func (a *App) spotifyDisconnect() tea.Cmd {
	if a.spotifyClient == nil {
		a.setMessage("Spotify not configured", MessageError)
		return nil
	}
	a.queueCommand(CmdSpotifyDisconnect, nil)
	a.setMessage("Disconnecting from Spotify...", MessageInfo)
	return nil
}

func (a *App) refreshSpotifySync() {
	if a.spotifyClient == nil || !a.spotifyClient.IsConnected() {
		return
	}

	// Try to refresh token if needed
	if err := a.spotifyClient.RefreshTokenIfNeeded(); err != nil {
		log.WithError(err).Debug("Failed to refresh Spotify token")
		// Don't disconnect automatically, let user handle it
	}

	state, err := a.spotifyClient.GetPlaybackState()
	if err != nil {
		log.WithError(err).Debug("Failed to get Spotify playback state")
		return
	}

	// Send Spotify update message to UI thread
	a.sendResult(spotifyUpdateMsg{state: &state})
}

func (a *App) renderSpotifyPanel(panelStyle lipgloss.Style) string {
	if a.spotifyState == nil {
		// Show disconnected/no data state
		spotifyPanel := panelStyle.Render(
			labelStyle.Render("Spotify Controls") + "\n\n" +
				fmt.Sprintf("Status: %s\n", valueStyle.Render("Not Connected")) +
				"\n" +
				mutedTextStyle.Render("Press 'space' to play/pause") + "\n" +
				mutedTextStyle.Render("Press 'n' for next, 'b' for prev") + "\n" +
				mutedTextStyle.Render("Press 't' to toggle panel"),
		)
		return spotifyPanel
	}

	// Show connected state with track info
	var statusText string
	if a.spotifyConnected {
		statusText = "Connected"
	} else {
		statusText = "Disconnected"
	}

	var playingText string
	if a.spotifyState.IsPlaying {
		playingText = "▶️ Playing"
	} else {
		playingText = "⏸️ Paused"
	}

	var shuffleText string
	if a.spotifyState.Shuffle {
		shuffleText = "🔀 On"
	} else {
		shuffleText = "➡️ Off"
	}

	// Progress bar for current track
	progressBar := a.spotifyProgress.ViewAs(0.0) // Will be updated in message handler
	if a.spotifyState.Track.Duration > 0 {
		progressPercent := float64(a.spotifyState.Track.Progress) / float64(a.spotifyState.Track.Duration)
		progressBar = a.spotifyProgress.ViewAs(progressPercent)
	}

	spotifyPanel := panelStyle.Render(
		labelStyle.Render("🎵 Spotify Controls") + "\n\n" +
			fmt.Sprintf("Status: %s\n", valueStyle.Render(statusText)) +
			fmt.Sprintf("State: %s\n", valueStyle.Render(playingText)) +
			"\n" +
			fmt.Sprintf("Track: %s\n", valueStyle.Render(formatRTLText(a.spotifyState.Track.Name))) +
			fmt.Sprintf("Artist: %s\n", valueStyle.Render(formatRTLText(a.spotifyState.Track.Artist))) +
			fmt.Sprintf("Album: %s\n", valueStyle.Render(formatRTLText(a.spotifyState.Track.Album))) +
			"\n" +
			fmt.Sprintf("Progress: %s / %s\n",
				valueStyle.Render(formatDuration(a.spotifyState.Track.Progress)),
				valueStyle.Render(formatDuration(a.spotifyState.Track.Duration))) +
			progressBar + "\n\n" +
			fmt.Sprintf("Volume: %s\n", valueStyle.Render(fmt.Sprintf("%d%%", a.spotifyState.Volume))) +
			fmt.Sprintf("Shuffle: %s\n", valueStyle.Render(shuffleText)) +
			fmt.Sprintf("Repeat: %s", valueStyle.Render(a.spotifyState.Repeat)),
	)
	return spotifyPanel
}

func (a *App) renderSpotifyDevicesPanel(panelStyle lipgloss.Style) string {
	if len(a.spotifyDevices) == 0 {
		return panelStyle.Render(
			labelStyle.Render("🎵 Spotify Devices") + "\n\n" +
				mutedTextStyle.Render("No devices found.") + "\n" +
				mutedTextStyle.Render("Make sure Spotify is open on at least one device.") + "\n" +
				mutedTextStyle.Render("Press 'y' to refresh and select devices."),
		)
	}

	var deviceList strings.Builder
	deviceList.WriteString(labelStyle.Render("🎵 Spotify Devices") + "\n\n")

	if a.spotifyDeviceMode {
		deviceList.WriteString(accentTextStyle.Render("Device Selection Mode") + "\n")
		deviceList.WriteString(mutedTextStyle.Render("Use ↑↓ to navigate, Enter to select, Esc to cancel") + "\n\n")
	}

	for i, device := range a.spotifyDevices {
		var deviceLine string
		var icon string

		// Device type icons
		switch strings.ToLower(device.Type) {
		case "computer":
			icon = "💻"
		case "smartphone":
			icon = "📱"
		case "speaker":
			icon = "🔊"
		case "tv":
			icon = "📺"
		case "castaudio", "chromecast_audio":
			icon = "🎵"
		case "castvideo", "chromecast":
			icon = "📺"
		default:
			icon = "🎧"
		}

		// Format device name and status
		deviceName := device.Name
		if len(deviceName) > 25 {
			deviceName = deviceName[:22] + "..."
		}

		var statusInfo string
		if device.IsActive {
			statusInfo = successTextStyle.Render(" (active)")
		} else if device.IsRestricted {
			statusInfo = mutedTextStyle.Render(" (restricted)")
		}

		// Volume info
		volumeInfo := fmt.Sprintf(" %d%%", device.VolumePercent)

		// Selection highlighting
		if a.spotifyDeviceMode && i == a.spotifyDeviceSelection {
			// Highlight selected device
			deviceLine = fmt.Sprintf("▶ %s %s%s %s",
				icon,
				primaryTextStyle.Render(deviceName),
				statusInfo,
				valueStyle.Render(volumeInfo))
		} else {
			// Normal device display
			deviceLine = fmt.Sprintf("  %s %s%s %s",
				icon,
				valueStyle.Render(deviceName),
				statusInfo,
				mutedTextStyle.Render(volumeInfo))
		}

		deviceList.WriteString(deviceLine + "\n")
	}

	// Add help text
	if !a.spotifyDeviceMode {
		deviceList.WriteString("\n" + mutedTextStyle.Render("Press 'y' to refresh devices and enter selection mode"))
	}

	return panelStyle.Render(deviceList.String())
}

// Helper function to format duration
func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0:00"
	}
	mins := int(d.Minutes())
	secs := int(d.Seconds()) % 60
	return fmt.Sprintf("%d:%02d", mins, secs)
}

// isHebrewText checks if text contains Hebrew characters
func isHebrewText(text string) bool {
	hebrewCount := 0
	totalChars := 0

	for _, r := range text {
		if unicode.IsLetter(r) {
			totalChars++
			// Hebrew Unicode range: U+0590 to U+05FF
			if r >= 0x0590 && r <= 0x05FF {
				hebrewCount++
			}
		}
	}

	// Consider it Hebrew if more than 30% of letters are Hebrew
	return totalChars > 0 && float64(hebrewCount)/float64(totalChars) > 0.3
}

// reverseString reverses a string character by character
func reverseString(s string) string {
	runes := []rune(s)
	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
		runes[i], runes[j] = runes[j], runes[i]
	}
	return string(runes)
}

// formatRTLText properly formats right-to-left text for terminal display
func formatRTLText(text string) string {
	if text == "" {
		return text
	}

	// If it contains Hebrew, apply RTL formatting
	if isHebrewText(text) {
		// For terminals that don't support Unicode bidirectional text well,
		// we'll reverse the Hebrew text manually

		// Split by spaces to handle mixed Hebrew/English text properly
		words := strings.Fields(text)
		var processedWords []string

		for _, word := range words {
			if isHebrewText(word) {
				// Reverse Hebrew words
				processedWords = append(processedWords, reverseString(word))
			} else {
				// Keep non-Hebrew words as-is
				processedWords = append(processedWords, word)
			}
		}

		// Join words back, but in reverse order for RTL reading
		result := make([]string, len(processedWords))
		for i, word := range processedWords {
			result[len(processedWords)-1-i] = word
		}

		return strings.Join(result, " ")
	}

	return text
}

// Spotify authentication methods
func (a *App) startSpotifyAuth() tea.Cmd {
	if a.spotifyClient == nil {
		log.Error("Spotify client is nil - not configured")
		a.setMessage("Spotify not configured", MessageError)
		return nil
	}

	log.Info("Starting Spotify authentication process")
	log.WithField("clientConfigured", a.spotifyClient != nil).Debug("Spotify client configuration check")

	// Generate auth URL and show immediate feedback
	a.spotifyAuthURL = a.spotifyClient.GetAuthURL()
	log.WithField("authURL", a.spotifyAuthURL).Debug("Generated Spotify auth URL")
	a.setMessage("Starting Spotify authentication...", MessageInfo)

	// Start the authentication process asynchronously
	return func() tea.Msg {
		log.Debug("Starting async Spotify authentication process")

		// Start authentication with proper error handling
		log.Info("Starting Spotify authentication with callback")
		if err := a.spotifyClient.AuthenticateWithCallback(3 * time.Minute); err != nil {
			log.WithError(err).Error("Spotify authentication failed")

			// Check if it's a timeout error and provide helpful message
			var errorMsg string
			if strings.Contains(err.Error(), "timeout") {
				errorMsg = "Authentication timed out. Please try again and complete the process faster."
				log.Warn("Spotify authentication timed out")
			} else if strings.Contains(err.Error(), "cancelled") {
				errorMsg = "Authentication was cancelled. Please try again."
				log.Warn("Spotify authentication was cancelled")
			} else if strings.Contains(err.Error(), "denied") {
				errorMsg = "Authentication was denied. Please try again and accept the permissions."
				log.Warn("Spotify authentication was denied")
			} else {
				errorMsg = fmt.Sprintf("Spotify authentication failed: %v", err)
				log.WithError(err).Error("Spotify authentication failed with error")
			}

			return messageMsg{text: errorMsg, msgType: MessageError}
		}

		log.Info("Spotify authentication completed successfully!")
		return messageMsg{text: "Spotify authentication successful! Welcome back to NAD Controller.", msgType: MessageSuccess}
	}
}

func (a *App) completeSpotifyAuth(code string) tea.Cmd {
	// This method is no longer needed since we use the callback server
	// but keeping it for backward compatibility
	if a.spotifyClient == nil {
		a.setMessage("Spotify not configured", MessageError)
		return nil
	}

	// Queue authentication completion
	params := map[string]interface{}{"code": code}
	a.queueCommand(CmdSpotifyAuth, params)
	a.setMessage("Completing Spotify authentication...", MessageInfo)
	return nil
}

// openBrowser opens the default browser with the given URL and attempts to refocus terminal
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start"}
	case "darwin":
		cmd = "open"
	default: // "linux", "freebsd", "openbsd", "netbsd"
		cmd = "xdg-open"
	}
	args = append(args, url)

	err := exec.Command(cmd, args...).Start()
	if err != nil {
		return err
	}

	// Attempt to refocus terminal after a brief delay in background
	go func() {
		// Wait a moment for browser to open and user to see it
		time.Sleep(1 * time.Second) // Reduced from 2 seconds
		log.Debug("Attempting to refocus terminal window")
		refocusTerminal()
	}()

	return nil
}

// refocusTerminal attempts to bring the terminal window back to focus
func refocusTerminal() {
	log.Debug("Attempting to refocus terminal window")

	switch runtime.GOOS {
	case "darwin":
		// Enhanced terminal-agnostic approach for macOS with better debugging
		if err := exec.Command("osascript", "-e", `
			tell application "System Events"
				-- Get current process info for debugging
				set currentPID to do shell script "echo $PPID"
				log "Current PPID: " & currentPID
				
				-- Get all running applications for debugging
				set allApps to name of every application process whose background only is false
				log "Running apps: " & (allApps as string)
				
				-- Try to find the process that contains our current shell
				try
					set parentInfo to do shell script "ps -p " & currentPID & " -o comm="
					set appName to last word of parentInfo
					log "Parent process: " & appName
					
					-- Convert common process names to app names
					if appName contains "Hyper" or appName contains "hyper" then
						set targetApp to "Hyper"
					else if appName contains "code" or appName contains "Code" then
						set targetApp to "Visual Studio Code"
					else if appName contains "cursor" or appName contains "Cursor" then
						set targetApp to "Cursor"
					else if appName contains "Terminal" or appName contains "terminal" then
						set targetApp to "Terminal"
					else if appName contains "iTerm" then
						set targetApp to "iTerm2"
					else
						-- Try to find by process hierarchy - get the actual terminal app
						set targetApp to ""
						try
							-- Check what's running our process
							set psOutput to do shell script "ps -p " & currentPID & " -o ppid="
							set grandParentPID to psOutput as string
							set grandParentInfo to do shell script "ps -p " & grandParentPID & " -o comm="
							log "Grandparent process: " & grandParentInfo
							
							-- Look for terminal apps in the process tree
							set allProcesses to every process whose background only is false
							repeat with proc in allProcesses
								set procName to name of proc
								if procName contains "Hyper" or procName contains "Terminal" or procName contains "iTerm" or procName contains "Code" or procName contains "Cursor" then
									set targetApp to procName
									exit repeat
								end if
							end repeat
						end try
					end if
					
					log "Target app: " & targetApp
					
					-- Activate and bring to front the found app using multiple methods
					if targetApp is not "" then
						log "Attempting to activate: " & targetApp
						
						-- Method 1: Activate the application
						tell application targetApp to activate
						log "Activated app"
						
						-- Method 2: Set the process to frontmost (more forceful)
						delay 0.2
						tell process targetApp to set frontmost to true
						log "Set process frontmost"
						
						-- Method 3: Ensure all windows are visible and raised
						delay 0.2
						tell application targetApp
							set visible to true
							if name of targetApp is "Hyper" then
								-- Hyper-specific: bring all windows forward
								try
									tell process "Hyper"
										set frontmost to true
										-- Try to raise windows
										try
											perform action "AXRaise" of windows
										end try
									end tell
								end try
							end if
						end tell
						log "Set visible and raised windows"
						
						-- Method 4: Force focus using System Events (last resort)
						delay 0.2
						try
							tell process targetApp
								set frontmost to true
								-- Click on the first window to ensure it's active
								try
									if (count of windows) > 0 then
										perform action "AXRaise" of window 1
									end if
								end try
							end tell
							log "Forced focus with System Events"
						end try
						
						return
					end if
				end try
				
				-- Fallback: Try all known terminal/editor apps with enhanced bring-to-front
				log "Falling back to app enumeration"
				set terminalApps to {"Hyper", "HyperTerm", "Cursor", "Visual Studio Code", "Code", "Terminal", "iTerm2", "iTerm", "Alacritty", "Kitty", "Warp", "WezTerm"}
				
				repeat with appName in terminalApps
					try
						if exists process appName then
							log "Found running app: " & appName
							-- Multi-method approach to ensure it comes to front
							tell application appName to activate
							delay 0.2
							tell process appName to set frontmost to true
							delay 0.2
							tell application appName to set visible to true
							-- Extra force for stubborn apps
							delay 0.2
							tell process appName
								set frontmost to true
								try
									if (count of windows) > 0 then
										perform action "AXRaise" of window 1
									end if
								end try
							end tell
							log "Successfully activated: " & appName
							return
						end if
					end try
				end repeat
				
				log "No terminal apps found to activate"
			end tell
		`).Run(); err != nil {
			log.WithError(err).Debug("AppleScript method failed, trying fallback commands")

			// Enhanced fallback: Try common terminal apps with multiple activation methods
			fallbackApps := []string{
				"Hyper", "HyperTerm", "Cursor", "Visual Studio Code",
				"Terminal", "iTerm2", "iTerm", "Alacritty", "Kitty", "Warp",
			}

			for _, app := range fallbackApps {
				// Method 1: Use 'open' to activate
				if err := exec.Command("open", "-a", app).Run(); err == nil {
					log.WithField("app", app).Debug("Successfully activated app with 'open'")

					// Method 2: Use AppleScript to force to front
					time.Sleep(300 * time.Millisecond)
					exec.Command("osascript", "-e", fmt.Sprintf(`
						tell application "System Events"
							try
								tell process "%s" to set frontmost to true
								delay 0.1
								tell process "%s"
									if (count of windows) > 0 then
										try
											perform action "AXRaise" of window 1
										end try
									end if
								end tell
							end try
						end tell
					`, app, app)).Run()

					// Method 3: Additional force activation
					time.Sleep(200 * time.Millisecond)
					exec.Command("osascript", "-e", fmt.Sprintf(`
						tell application "%s"
							activate
							set visible to true
						end tell
					`, app)).Run()

					log.WithField("app", app).Debug("Applied enhanced activation methods")
					return
				}
			}
		} else {
			log.Debug("AppleScript executed successfully")
		}

	case "linux":
		// Terminal-agnostic approach for Linux
		if err := exec.Command("bash", "-c", `
			# Find the window that contains our process
			current_pid=$$
			parent_pid=$(ps -o ppid= -p $current_pid | tr -d ' ')
			
			# Try to find the window by process
			if command -v wmctrl >/dev/null 2>&1; then
				# Get all windows and try to match by process
				wmctrl -l -p | while read -r line; do
					window_pid=$(echo "$line" | awk '{print $3}')
					window_id=$(echo "$line" | awk '{print $1}')
					
					if [ "$window_pid" = "$parent_pid" ] || [ "$window_pid" = "$current_pid" ]; then
						wmctrl -i -a "$window_id"
						exit 0
					fi
				done
			fi
			
			# Fallback: Try by window name
			for app in hyper cursor code terminal gnome-terminal konsole xfce4-terminal; do
				if command -v wmctrl >/dev/null 2>&1; then
					wmctrl -a "$app" 2>/dev/null && exit 0
				fi
				if command -v xdotool >/dev/null 2>&1; then
					xdotool search --name "$app" windowactivate 2>/dev/null && exit 0
				fi
			done
		`).Run(); err != nil {
			log.WithError(err).Debug("Linux refocus failed")
		}

	case "windows":
		// Terminal-agnostic approach for Windows
		exec.Command("powershell", "-WindowStyle", "Hidden", "-Command", `
			Add-Type -TypeDefinition '
				using System;
				using System.Runtime.InteropServices;
				public class Win32 {
					[DllImport("user32.dll")]
					public static extern bool SetForegroundWindow(IntPtr hWnd);
					[DllImport("user32.dll")]
					public static extern bool ShowWindow(IntPtr hWnd, int nCmdShow);
					[DllImport("user32.dll")]
					public static extern bool IsIconic(IntPtr hWnd);
				}
			'
			
			# Try to find terminal/editor windows by process name and window title
			$terminalProcesses = @("Hyper", "HyperTerm", "Cursor", "Code", "cmd", "powershell", "WindowsTerminal", "wt")
			$processes = Get-Process | Where-Object {
				$_.MainWindowTitle -ne "" -and (
					$terminalProcesses -contains $_.ProcessName -or
					$_.MainWindowTitle -like "*Hyper*" -or
					$_.MainWindowTitle -like "*Cursor*" -or
					$_.MainWindowTitle -like "*Visual Studio Code*" -or
					$_.MainWindowTitle -like "*nadctl*" -or
					$_.MainWindowTitle -like "*terminal*"
				)
			} | Sort-Object StartTime -Descending
			
			if ($processes) {
				$process = $processes[0]
				$hwnd = $process.MainWindowHandle
				if ([Win32]::IsIconic($hwnd)) {
					[Win32]::ShowWindow($hwnd, 9)  # SW_RESTORE
				}
				[Win32]::SetForegroundWindow($hwnd)
			}
		`).Run()
	}

	log.Debug("Terminal refocus attempt completed")
}

// diagnoseFocusEnvironment helps understand the current process hierarchy and terminal setup
func diagnoseFocusEnvironment() {
	log.Debug("=== FOCUS ENVIRONMENT DIAGNOSIS ===")

	// Get current process info
	if output, err := exec.Command("ps", "-p", fmt.Sprintf("%d", os.Getpid()), "-o", "pid,ppid,comm").Output(); err == nil {
		log.Debug("Current process info:")
		log.Debug(string(output))
	}

	// Get parent process info
	if output, err := exec.Command("sh", "-c", "ps -p $PPID -o pid,ppid,comm").Output(); err == nil {
		log.Debug("Parent process info:")
		log.Debug(string(output))
	}

	// Get grandparent process info
	if output, err := exec.Command("sh", "-c", "ps -p $(ps -p $PPID -o ppid= | tr -d ' ') -o pid,ppid,comm").Output(); err == nil {
		log.Debug("Grandparent process info:")
		log.Debug(string(output))
	}

	// List all terminal-like processes
	if output, err := exec.Command("ps", "aux").Output(); err == nil {
		lines := strings.Split(string(output), "\n")
		log.Debug("Terminal-like processes found:")
		for _, line := range lines {
			lower := strings.ToLower(line)
			if strings.Contains(lower, "hyper") || strings.Contains(lower, "terminal") ||
				strings.Contains(lower, "iterm") || strings.Contains(lower, "cursor") ||
				strings.Contains(lower, "code") {
				log.Debug(line)
			}
		}
	}

	// Check what's in the foreground (macOS specific)
	if runtime.GOOS == "darwin" {
		if output, err := exec.Command("osascript", "-e", `
			tell application "System Events"
				set frontApp to name of first application process whose frontmost is true
				return frontApp
			end tell
		`).Output(); err == nil {
			log.WithField("frontmostApp", strings.TrimSpace(string(output))).Debug("Current frontmost application")
		}

		// List all visible apps
		if output, err := exec.Command("osascript", "-e", `
			tell application "System Events"
				set visibleApps to name of every application process whose background only is false
				return visibleApps as string
			end tell
		`).Output(); err == nil {
			log.WithField("visibleApps", strings.TrimSpace(string(output))).Debug("All visible applications")
		}
	}

	log.Debug("=== END DIAGNOSIS ===")
}

// Tab navigation methods
func (a *App) nextTab() {
	if !a.tabsEnabled {
		return
	}
	currentIndex := a.getCurrentTabIndex()
	nextIndex := (currentIndex + 1) % len(a.tabs)
	a.currentTab = a.tabs[nextIndex].ID
	a.setMessage(fmt.Sprintf("Switched to %s tab", a.tabs[nextIndex].Name), MessageInfo)
}

func (a *App) prevTab() {
	if !a.tabsEnabled {
		return
	}
	currentIndex := a.getCurrentTabIndex()
	prevIndex := (currentIndex - 1 + len(a.tabs)) % len(a.tabs)
	a.currentTab = a.tabs[prevIndex].ID
	a.setMessage(fmt.Sprintf("Switched to %s tab", a.tabs[prevIndex].Name), MessageInfo)
}

func (a *App) setTab(tab Tab) {
	if !a.tabsEnabled {
		return
	}
	if tab != a.currentTab {
		a.currentTab = tab
		for _, tabInfo := range a.tabs {
			if tabInfo.ID == tab {
				a.setMessage(fmt.Sprintf("Switched to %s tab", tabInfo.Name), MessageInfo)
				break
			}
		}
	}
}

func (a *App) getCurrentTabIndex() int {
	for i, tab := range a.tabs {
		if tab.ID == a.currentTab {
			return i
		}
	}
	return 0
}

// SetDemoMode enables or disables demo mode
func (a *App) SetDemoMode(enabled bool) {
	a.demoMode = enabled
	if enabled {
		log.Debug("Demo mode enabled - NAD device operations will be simulated")
	}
}

// Cleanup gracefully closes all connections and resources
func (a *App) Cleanup() error {
	log.Debug("Starting application cleanup")

	var errors []error

	// Close NAD device connection
	if a.device != nil {
		if err := a.device.Disconnect(); err != nil {
			log.WithError(err).Debug("Error disconnecting from NAD device during cleanup")
			errors = append(errors, fmt.Errorf("NAD device disconnect: %w", err))
		} else {
			log.Debug("Successfully disconnected from NAD device during cleanup")
		}
		a.device = nil
		a.connected = false
	}

	// Stop Spotify callback server
	if a.spotifyClient != nil {
		if err := a.spotifyClient.StopCallbackServer(); err != nil {
			log.WithError(err).Debug("Error stopping Spotify callback server during cleanup")
			errors = append(errors, fmt.Errorf("Spotify server stop: %w", err))
		} else {
			log.Debug("Successfully stopped Spotify callback server during cleanup")
		}
	}

	// Close result channel
	if a.resultChan != nil {
		close(a.resultChan)
		a.resultChan = nil
		log.Debug("Closed result channel during cleanup")
	}

	// Stop any running timers
	if a.adjustTimer != nil {
		a.adjustTimer.Stop()
		a.adjustTimer = nil
		log.Debug("Stopped adjust timer during cleanup")
	}

	log.Debug("Application cleanup completed")

	// Return combined error if any occurred
	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	return nil
}

// Spotify device management methods
func (a *App) spotifyListDevices() tea.Cmd {
	if a.spotifyClient == nil || !a.spotifyClient.IsConnected() {
		a.setMessage("Spotify not connected", MessageError)
		return nil
	}
	a.queueCommand(CmdSpotifyListDevices, nil)
	a.setMessage("Loading Spotify devices...", MessageInfo)
	return nil
}

func (a *App) spotifyTransferToSelected() tea.Cmd {
	if a.spotifyClient == nil || !a.spotifyClient.IsConnected() {
		a.setMessage("Spotify not connected", MessageError)
		return nil
	}

	if len(a.spotifyDevices) == 0 {
		a.setMessage("No devices available", MessageWarning)
		return nil
	}

	if a.spotifyDeviceSelection < 0 || a.spotifyDeviceSelection >= len(a.spotifyDevices) {
		a.setMessage("Invalid device selection", MessageError)
		return nil
	}

	selectedDevice := a.spotifyDevices[a.spotifyDeviceSelection]

	// Check if already active
	if selectedDevice.IsActive {
		a.setMessage(fmt.Sprintf("'%s' is already the active device", selectedDevice.Name), MessageWarning)
		a.spotifyDeviceMode = false
		return nil
	}

	params := map[string]interface{}{
		"deviceID":   selectedDevice.ID,
		"deviceName": selectedDevice.Name,
	}
	a.queueCommand(CmdSpotifyTransferDevice, params)
	a.setMessage(fmt.Sprintf("Transferring playback to '%s'...", selectedDevice.Name), MessageInfo)
	a.spotifyDeviceMode = false
	return nil
}

func (a *App) spotifyDeviceSelectionUp() {
	if len(a.spotifyDevices) == 0 {
		return
	}
	a.spotifyDeviceSelection--
	if a.spotifyDeviceSelection < 0 {
		a.spotifyDeviceSelection = len(a.spotifyDevices) - 1
	}
	selectedDevice := a.spotifyDevices[a.spotifyDeviceSelection]
	a.setMessage(fmt.Sprintf("Selected: %s (%s)", selectedDevice.Name, selectedDevice.Type), MessageInfo)
}

func (a *App) spotifyDeviceSelectionDown() {
	if len(a.spotifyDevices) == 0 {
		return
	}
	a.spotifyDeviceSelection++
	if a.spotifyDeviceSelection >= len(a.spotifyDevices) {
		a.spotifyDeviceSelection = 0
	}
	selectedDevice := a.spotifyDevices[a.spotifyDeviceSelection]
	a.setMessage(fmt.Sprintf("Selected: %s (%s)", selectedDevice.Name, selectedDevice.Type), MessageInfo)
}
