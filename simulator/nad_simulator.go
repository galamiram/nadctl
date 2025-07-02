package simulator

import (
	"bufio"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// NADSimulator simulates a NAD receiver for testing
type NADSimulator struct {
	listener    net.Listener
	state       *DeviceState
	stateMutex  sync.RWMutex
	connections map[net.Conn]bool
	connMutex   sync.RWMutex
	running     bool
	stopChan    chan bool
}

// DeviceState holds the simulated device state
type DeviceState struct {
	Power      string  // "On" or "Off"
	Volume     float64 // Volume in dB (-80 to +10)
	Source     string  // Current input source
	Mute       string  // "On" or "Off"
	Brightness int     // Display brightness (0-3)
	Model      string  // Device model
}

// NewNADSimulator creates a new NAD device simulator
func NewNADSimulator() *NADSimulator {
	return &NADSimulator{
		state: &DeviceState{
			Power:      "Off",
			Volume:     -30.0,
			Source:     "Stream",
			Mute:       "Off",
			Brightness: 2,
			Model:      "NAD T 758 V3i",
		},
		connections: make(map[net.Conn]bool),
		stopChan:    make(chan bool),
	}
}

// Start begins the simulator server
func (sim *NADSimulator) Start(port string) error {
	if port == "" {
		port = "30001"
	}

	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		return fmt.Errorf("failed to start simulator: %v", err)
	}

	sim.listener = listener
	sim.running = true

	log.WithField("port", port).Info("🎵 NAD Simulator started")
	log.Info("📱 Connect your TUI with: nadctl tui --config simulator.yaml")
	log.Info("🔧 Or set NAD_IP=127.0.0.1 environment variable")

	go sim.acceptConnections()
	return nil
}

// Stop shuts down the simulator
func (sim *NADSimulator) Stop() error {
	if !sim.running {
		return nil
	}

	sim.running = false
	close(sim.stopChan)

	// Close all connections
	sim.connMutex.Lock()
	for conn := range sim.connections {
		conn.Close()
	}
	sim.connMutex.Unlock()

	// Close listener
	if sim.listener != nil {
		sim.listener.Close()
	}

	log.Info("NAD Simulator stopped")
	return nil
}

// acceptConnections handles incoming connections
func (sim *NADSimulator) acceptConnections() {
	for sim.running {
		conn, err := sim.listener.Accept()
		if err != nil {
			if sim.running {
				log.WithError(err).Error("Failed to accept connection")
			}
			continue
		}

		log.WithField("client", conn.RemoteAddr()).Info("Client connected")

		sim.connMutex.Lock()
		sim.connections[conn] = true
		sim.connMutex.Unlock()

		go sim.handleConnection(conn)
	}
}

// handleConnection processes commands from a client
func (sim *NADSimulator) handleConnection(conn net.Conn) {
	defer func() {
		sim.connMutex.Lock()
		delete(sim.connections, conn)
		sim.connMutex.Unlock()
		conn.Close()
		log.WithField("client", conn.RemoteAddr()).Info("Client disconnected")
	}()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	buffer := make([]byte, 1024)

	for sim.running {
		// Set a short read timeout to handle commands without newlines
		conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))

		// Read available data
		n, err := reader.Read(buffer)
		if err != nil {
			// Check if it's just a timeout (no data available)
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				// Continue waiting for data
				continue
			}
			// Real error or EOF
			if err.Error() != "EOF" && sim.running {
				log.WithError(err).Debug("Error reading from client")
			}
			break
		}

		if n == 0 {
			continue
		}

		// Process the received data
		data := string(buffer[:n])
		commands := sim.extractCommands(data)

		for _, command := range commands {
			if command == "" {
				continue
			}

			log.WithFields(log.Fields{
				"client":  conn.RemoteAddr(),
				"command": command,
			}).Debug("Received command")

			response := sim.processCommand(command)

			if response != "" {
				_, err := writer.WriteString(response + "\r\n")
				if err != nil {
					log.WithError(err).Error("Failed to write response")
					return
				}
				writer.Flush()

				log.WithFields(log.Fields{
					"client":   conn.RemoteAddr(),
					"response": response,
				}).Debug("Sent response")
			}
		}

		// Small delay to prevent busy loop
		time.Sleep(10 * time.Millisecond)
	}
}

// extractCommands extracts individual commands from received data
func (sim *NADSimulator) extractCommands(data string) []string {
	var commands []string

	// Clean up the data
	data = strings.TrimSpace(data)
	if data == "" {
		return commands
	}

	// Split by common delimiters
	parts := strings.FieldsFunc(data, func(c rune) bool {
		return c == '\n' || c == '\r' || c == '\000'
	})

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check if it's a valid command pattern
		if strings.HasSuffix(part, "?") ||
			strings.Contains(part, "=") ||
			strings.HasSuffix(part, "+") ||
			strings.HasSuffix(part, "-") {
			commands = append(commands, part)
		}
	}

	return commands
}

// processCommand handles NAD protocol commands
func (sim *NADSimulator) processCommand(command string) string {
	sim.stateMutex.Lock()
	defer sim.stateMutex.Unlock()

	command = strings.TrimSpace(command)

	// Handle queries (commands ending with ?)
	if strings.HasSuffix(command, "?") {
		return sim.handleQuery(command)
	}

	// Handle set commands (commands with =)
	if strings.Contains(command, "=") {
		return sim.handleSet(command)
	}

	// Handle toggle commands
	return sim.handleToggle(command)
}

// handleQuery processes query commands
func (sim *NADSimulator) handleQuery(command string) string {
	switch command {
	case "Main.Power?":
		return fmt.Sprintf("Main.Power=%s", sim.state.Power)

	case "Main.Volume?":
		return fmt.Sprintf("Main.Volume=%.1f", sim.state.Volume)

	case "Main.Source?":
		return fmt.Sprintf("Main.Source=%s", sim.state.Source)

	case "Main.Mute?":
		return fmt.Sprintf("Main.Mute=%s", sim.state.Mute)

	case "Main.Brightness?":
		return fmt.Sprintf("Main.Brightness=%d", sim.state.Brightness)

	case "Main.Model?":
		return fmt.Sprintf("Main.Model=%s", sim.state.Model)

	default:
		log.WithField("command", command).Warn("Unknown query command")
		return ""
	}
}

// handleSet processes set commands
func (sim *NADSimulator) handleSet(command string) string {
	parts := strings.SplitN(command, "=", 2)
	if len(parts) != 2 {
		return ""
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	switch key {
	case "Main.Power":
		if value == "On" || value == "Off" {
			oldPower := sim.state.Power
			sim.state.Power = value
			log.WithFields(log.Fields{
				"old": oldPower,
				"new": value,
			}).Info("Power changed")
			return fmt.Sprintf("Main.Power=%s", sim.state.Power)
		}

	case "Main.Volume":
		if vol, err := strconv.ParseFloat(value, 64); err == nil {
			// Clamp volume to realistic range
			if vol < -80 {
				vol = -80
			} else if vol > 10 {
				vol = 10
			}
			oldVol := sim.state.Volume
			sim.state.Volume = vol
			log.WithFields(log.Fields{
				"old": oldVol,
				"new": vol,
			}).Info("Volume changed")
			return fmt.Sprintf("Main.Volume=%.1f", sim.state.Volume)
		}

	case "Main.Source":
		sources := []string{"Stream", "Wireless", "TV", "Phono", "Coax1", "Coax2", "Opt1", "Opt2"}
		for _, source := range sources {
			if strings.EqualFold(value, source) {
				oldSource := sim.state.Source
				sim.state.Source = source
				log.WithFields(log.Fields{
					"old": oldSource,
					"new": source,
				}).Info("Source changed")
				return fmt.Sprintf("Main.Source=%s", sim.state.Source)
			}
		}

	case "Main.Mute":
		if value == "On" || value == "Off" {
			oldMute := sim.state.Mute
			sim.state.Mute = value
			log.WithFields(log.Fields{
				"old": oldMute,
				"new": value,
			}).Info("Mute changed")
			return fmt.Sprintf("Main.Mute=%s", sim.state.Mute)
		}

	case "Main.Brightness":
		if brightness, err := strconv.Atoi(value); err == nil {
			if brightness >= 0 && brightness <= 3 {
				oldBrightness := sim.state.Brightness
				sim.state.Brightness = brightness
				log.WithFields(log.Fields{
					"old": oldBrightness,
					"new": brightness,
				}).Info("Brightness changed")
				return fmt.Sprintf("Main.Brightness=%d", sim.state.Brightness)
			}
		}
	}

	log.WithField("command", command).Warn("Invalid set command")
	return ""
}

// handleToggle processes toggle commands
func (sim *NADSimulator) handleToggle(command string) string {
	switch command {
	case "Main.Power+", "Main.Power-":
		// Toggle power
		if sim.state.Power == "On" {
			sim.state.Power = "Off"
		} else {
			sim.state.Power = "On"
		}
		log.WithField("power", sim.state.Power).Info("Power toggled")
		return fmt.Sprintf("Main.Power=%s", sim.state.Power)

	case "Main.Mute+", "Main.Mute-":
		// Toggle mute
		if sim.state.Mute == "On" {
			sim.state.Mute = "Off"
		} else {
			sim.state.Mute = "On"
		}
		log.WithField("mute", sim.state.Mute).Info("Mute toggled")
		return fmt.Sprintf("Main.Mute=%s", sim.state.Mute)

	case "Main.Volume+":
		// Increase volume
		sim.state.Volume += 1.0
		if sim.state.Volume > 10 {
			sim.state.Volume = 10
		}
		log.WithField("volume", sim.state.Volume).Info("Volume increased")
		return fmt.Sprintf("Main.Volume=%.1f", sim.state.Volume)

	case "Main.Volume-":
		// Decrease volume
		sim.state.Volume -= 1.0
		if sim.state.Volume < -80 {
			sim.state.Volume = -80
		}
		log.WithField("volume", sim.state.Volume).Info("Volume decreased")
		return fmt.Sprintf("Main.Volume=%.1f", sim.state.Volume)

	case "Main.Source+":
		// Next source
		sources := []string{"Stream", "Wireless", "TV", "Phono", "Coax1", "Coax2", "Opt1", "Opt2"}
		currentIndex := 0
		for i, source := range sources {
			if source == sim.state.Source {
				currentIndex = i
				break
			}
		}
		nextIndex := (currentIndex + 1) % len(sources)
		sim.state.Source = sources[nextIndex]
		log.WithField("source", sim.state.Source).Info("Source changed to next")
		return fmt.Sprintf("Main.Source=%s", sim.state.Source)

	case "Main.Source-":
		// Previous source
		sources := []string{"Stream", "Wireless", "TV", "Phono", "Coax1", "Coax2", "Opt1", "Opt2"}
		currentIndex := 0
		for i, source := range sources {
			if source == sim.state.Source {
				currentIndex = i
				break
			}
		}
		prevIndex := (currentIndex - 1 + len(sources)) % len(sources)
		sim.state.Source = sources[prevIndex]
		log.WithField("source", sim.state.Source).Info("Source changed to previous")
		return fmt.Sprintf("Main.Source=%s", sim.state.Source)

	case "Main.Brightness+":
		// Increase brightness
		if sim.state.Brightness < 3 {
			sim.state.Brightness++
		}
		log.WithField("brightness", sim.state.Brightness).Info("Brightness increased")
		return fmt.Sprintf("Main.Brightness=%d", sim.state.Brightness)

	case "Main.Brightness-":
		// Decrease brightness
		if sim.state.Brightness > 0 {
			sim.state.Brightness--
		}
		log.WithField("brightness", sim.state.Brightness).Info("Brightness decreased")
		return fmt.Sprintf("Main.Brightness=%d", sim.state.Brightness)

	default:
		log.WithField("command", command).Warn("Unknown toggle command")
		return ""
	}
}

// GetState returns current device state (for monitoring/debugging)
func (sim *NADSimulator) GetState() DeviceState {
	sim.stateMutex.RLock()
	defer sim.stateMutex.RUnlock()
	return *sim.state
}

// SetState updates device state (for testing)
func (sim *NADSimulator) SetState(state DeviceState) {
	sim.stateMutex.Lock()
	defer sim.stateMutex.Unlock()
	sim.state = &state
}
