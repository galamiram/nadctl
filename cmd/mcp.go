package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/galamiram/nadctl/nadapi"
	"github.com/galamiram/nadctl/spotify"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const mcpDiscoverTimeout = 5 * time.Second

// mcpCmd represents the mcp command
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP server for NAD device control",
	Long: `Start a Model Context Protocol (MCP) server that allows LLMs to control NAD audio devices.

The MCP server provides tools for:
- Power control (on/off/toggle/status)
- Volume control (set/adjust/mute)
- Source selection and navigation
- Display brightness adjustment
- Device discovery and status
- Spotify device casting and playback control

Example usage with Cursor or other MCP-compatible AI tools:
  nadctl mcp

Environment variables:
  NAD_IP: IP address of the NAD device (default: auto-discover)
  NAD_PORT: Port of the NAD device (default: 30001)
  SPOTIFY_CLIENT_ID: Spotify client ID for device casting (optional)`,
	Run: func(cmd *cobra.Command, args []string) {
		runMCPServer()
	},
}

func init() {
	rootCmd.AddCommand(mcpCmd)
	mcpCmd.Flags().String("device-ip", "", "IP address of the NAD device")
	mcpCmd.Flags().String("device-port", "30001", "Port of the NAD device")
	viper.BindPFlag("mcp.device_ip", mcpCmd.Flags().Lookup("device-ip"))
	viper.BindPFlag("mcp.device_port", mcpCmd.Flags().Lookup("device-port"))
}

func runMCPServer() {
	// Create MCP server
	s := server.NewMCPServer(
		"NAD Audio Controller",
		"1.3.0",
		server.WithToolCapabilities(true),
		server.WithResourceCapabilities(true, true), // readable and writable
		server.WithPromptCapabilities(true),
	)

	// Register tools
	registerNADTools(s)
	registerSpotifyTools(s)

	// Register resources
	registerNADResources(s)

	// Register prompts
	registerNADPrompts(s)

	// Start the server using stdio
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("MCP Server error: %v\n", err)
		os.Exit(1)
	}
}

func getDevice() (*nadapi.Device, error) {
	deviceIP := viper.GetString("mcp.device_ip")
	devicePort := viper.GetString("mcp.device_port")

	// Use environment variables if flags not set
	if deviceIP == "" {
		deviceIP = os.Getenv("NAD_IP")
	}
	if devicePort == "" {
		if envPort := os.Getenv("NAD_PORT"); envPort != "" {
			devicePort = envPort
		}
	}

	// Auto-discover if no IP provided
	if deviceIP == "" {
		devices, err := nadapi.DiscoverDevices(mcpDiscoverTimeout)
		if err != nil {
			return nil, fmt.Errorf("device discovery failed: %v", err)
		}
		if len(devices) == 0 {
			return nil, fmt.Errorf("no NAD devices found on network")
		}
		deviceIP = devices[0].IP
		if devices[0].Port != "" {
			devicePort = devices[0].Port
		}
	}

	return nadapi.New(deviceIP, devicePort)
}

func getMCPSpotifyClient() (*spotify.Client, error) {
	clientID := viper.GetString("spotify.client_id")
	if clientID == "" {
		clientID = os.Getenv("SPOTIFY_CLIENT_ID")
	}

	if clientID == "" {
		return nil, fmt.Errorf("Spotify client ID not configured. Set it in config file or SPOTIFY_CLIENT_ID environment variable")
	}

	redirectURL := viper.GetString("spotify.redirect_url")
	if redirectURL == "" {
		redirectURL = "http://localhost:8888/callback"
	}

	client := spotify.NewClient(clientID, redirectURL)

	if !client.IsConnected() {
		return nil, fmt.Errorf("not connected to Spotify. Please authenticate first using the TUI or CLI")
	}

	return client, nil
}

func registerNADTools(s *server.MCPServer) {
	// Power Control Tools
	s.AddTool(
		mcp.NewTool("nad_power_on", mcp.WithDescription("Turn on the NAD audio device")),
		handlePowerOn,
	)

	s.AddTool(
		mcp.NewTool("nad_power_off", mcp.WithDescription("Turn off the NAD audio device")),
		handlePowerOff,
	)

	s.AddTool(
		mcp.NewTool("nad_power_toggle", mcp.WithDescription("Toggle power state of the NAD audio device")),
		handlePowerToggle,
	)

	s.AddTool(
		mcp.NewTool("nad_power_status", mcp.WithDescription("Get current power state of the NAD audio device")),
		handlePowerStatus,
	)

	// Volume Control Tools
	s.AddTool(
		mcp.NewTool("nad_volume_set",
			mcp.WithDescription("Set NAD device volume to a specific level"),
			mcp.WithNumber("volume",
				mcp.Required(),
				mcp.Description("Volume level in dB (typically -80 to +10)"),
			),
		),
		handleVolumeSet,
	)

	s.AddTool(
		mcp.NewTool("nad_volume_up", mcp.WithDescription("Increase NAD device volume")),
		handleVolumeUp,
	)

	s.AddTool(
		mcp.NewTool("nad_volume_down", mcp.WithDescription("Decrease NAD device volume")),
		handleVolumeDown,
	)

	s.AddTool(
		mcp.NewTool("nad_volume_status", mcp.WithDescription("Get current volume level")),
		handleVolumeStatus,
	)

	s.AddTool(
		mcp.NewTool("nad_mute_toggle", mcp.WithDescription("Toggle mute state of the NAD device")),
		handleMuteToggle,
	)

	s.AddTool(
		mcp.NewTool("nad_mute_status", mcp.WithDescription("Get current mute status")),
		handleMuteStatus,
	)

	// Source Control Tools
	s.AddTool(
		mcp.NewTool("nad_source_set",
			mcp.WithDescription("Set NAD device input source"),
			mcp.WithString("source",
				mcp.Required(),
				mcp.Description("Input source name"),
				mcp.Enum("Stream", "Wireless", "TV", "Phono", "Coax1", "Coax2", "Opt1", "Opt2"),
			),
		),
		handleSourceSet,
	)

	s.AddTool(
		mcp.NewTool("nad_source_next", mcp.WithDescription("Switch to next input source")),
		handleSourceNext,
	)

	s.AddTool(
		mcp.NewTool("nad_source_previous", mcp.WithDescription("Switch to previous input source")),
		handleSourcePrevious,
	)

	s.AddTool(
		mcp.NewTool("nad_source_status", mcp.WithDescription("Get current input source")),
		handleSourceStatus,
	)

	s.AddTool(
		mcp.NewTool("nad_source_list", mcp.WithDescription("List all available input sources")),
		handleSourceList,
	)

	// Brightness Control Tools
	s.AddTool(
		mcp.NewTool("nad_brightness_set",
			mcp.WithDescription("Set NAD device display brightness"),
			mcp.WithNumber("level",
				mcp.Required(),
				mcp.Description("Brightness level (0-3, where 0 is dimmest, 3 is brightest)"),
			),
		),
		handleBrightnessSet,
	)

	s.AddTool(
		mcp.NewTool("nad_brightness_up", mcp.WithDescription("Increase display brightness")),
		handleBrightnessUp,
	)

	s.AddTool(
		mcp.NewTool("nad_brightness_down", mcp.WithDescription("Decrease display brightness")),
		handleBrightnessDown,
	)

	s.AddTool(
		mcp.NewTool("nad_brightness_status", mcp.WithDescription("Get current brightness level")),
		handleBrightnessStatus,
	)

	// Device Discovery and Info Tools
	s.AddTool(
		mcp.NewTool("nad_discover", mcp.WithDescription("Discover NAD devices on the network")),
		handleDiscover,
	)

	s.AddTool(
		mcp.NewTool("nad_device_info", mcp.WithDescription("Get information about the connected NAD device")),
		handleDeviceInfo,
	)

	s.AddTool(
		mcp.NewTool("nad_device_status", mcp.WithDescription("Get comprehensive status of the NAD device")),
		handleDeviceStatus,
	)
}

func registerSpotifyTools(s *server.MCPServer) {
	// Spotify Device Management Tools
	s.AddTool(
		mcp.NewTool("spotify_devices_list", mcp.WithDescription("List all available Spotify Connect devices (Chromecast, computers, speakers, phones, etc.)")),
		handleSpotifyDevicesList,
	)

	s.AddTool(
		mcp.NewTool("spotify_transfer_playback",
			mcp.WithDescription("Transfer Spotify playback to a specific device"),
			mcp.WithString("device_identifier",
				mcp.Required(),
				mcp.Description("Device name (partial match) or device index (1, 2, 3, etc.)"),
			),
			mcp.WithBoolean("play",
				mcp.Description("Whether to start playing immediately after transfer (default: false)"),
			),
		),
		handleSpotifyTransferPlayback,
	)

	// Spotify Playback Control Tools
	s.AddTool(
		mcp.NewTool("spotify_play", mcp.WithDescription("Start or resume Spotify playback on current device")),
		handleSpotifyPlay,
	)

	s.AddTool(
		mcp.NewTool("spotify_pause", mcp.WithDescription("Pause Spotify playback")),
		handleSpotifyPause,
	)

	s.AddTool(
		mcp.NewTool("spotify_next", mcp.WithDescription("Skip to next track in Spotify")),
		handleSpotifyNext,
	)

	s.AddTool(
		mcp.NewTool("spotify_previous", mcp.WithDescription("Skip to previous track in Spotify")),
		handleSpotifyPrevious,
	)

	s.AddTool(
		mcp.NewTool("spotify_volume_set",
			mcp.WithDescription("Set Spotify volume level"),
			mcp.WithNumber("volume",
				mcp.Required(),
				mcp.Description("Volume level (0-100 percent)"),
			),
		),
		handleSpotifyVolumeSet,
	)

	s.AddTool(
		mcp.NewTool("spotify_shuffle_toggle", mcp.WithDescription("Toggle Spotify shuffle mode on/off")),
		handleSpotifyShuffleToggle,
	)

	s.AddTool(
		mcp.NewTool("spotify_status", mcp.WithDescription("Get current Spotify playback status and device information")),
		handleSpotifyStatus,
	)
}

// Power Control Handlers
func handlePowerOn(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	if err := device.PowerOn(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to power on: %v", err)), nil
	}

	return mcp.NewToolResultText("NAD device powered on successfully"), nil
}

func handlePowerOff(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	if err := device.PowerOff(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to power off: %v", err)), nil
	}

	return mcp.NewToolResultText("NAD device powered off successfully"), nil
}

func handlePowerToggle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	if err := device.PowerToggle(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to toggle power: %v", err)), nil
	}

	// Get new state
	state, err := device.GetPowerState()
	if err != nil {
		return mcp.NewToolResultText("Power toggled successfully"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Power toggled. Current state: %s", state)), nil
}

func handlePowerStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	state, err := device.GetPowerState()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get power state: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Power state: %s", state)), nil
}

// Volume Control Handlers
func handleVolumeSet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	volume, err := request.RequireFloat("volume")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid volume parameter: %v", err)), nil
	}

	if err := device.SetVolume(volume); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to set volume: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Volume set to %.1f dB", volume)), nil
}

func handleVolumeUp(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	if err := device.TuneVolume(nadapi.DirectionUp); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to increase volume: %v", err)), nil
	}

	// Get new volume
	vol, err := device.GetVolume()
	if err != nil {
		return mcp.NewToolResultText("Volume increased"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Volume increased to %s dB", vol)), nil
}

func handleVolumeDown(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	if err := device.TuneVolume(nadapi.DirectionDown); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to decrease volume: %v", err)), nil
	}

	// Get new volume
	vol, err := device.GetVolume()
	if err != nil {
		return mcp.NewToolResultText("Volume decreased"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Volume decreased to %s dB", vol)), nil
}

func handleVolumeStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	vol, err := device.GetVolume()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get volume: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Current volume: %s dB", vol)), nil
}

func handleMuteToggle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	if err := device.ToggleMute(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to toggle mute: %v", err)), nil
	}

	// Get new mute status
	status, err := device.GetMuteStatus()
	if err != nil {
		return mcp.NewToolResultText("Mute toggled"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Mute toggled. Current status: %s", status)), nil
}

func handleMuteStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	status, err := device.GetMuteStatus()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get mute status: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Mute status: %s", status)), nil
}

// Source Control Handlers
func handleSourceSet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	source, err := request.RequireString("source")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid source parameter: %v", err)), nil
	}

	if err := device.SetSource(source); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to set source: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Input source set to %s", source)), nil
}

func handleSourceNext(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	newSource, err := device.ToggleSource(nadapi.DirectionUp)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to switch to next source: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Switched to next source: %s", newSource)), nil
}

func handleSourcePrevious(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	newSource, err := device.ToggleSource(nadapi.DirectionDown)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to switch to previous source: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Switched to previous source: %s", newSource)), nil
}

func handleSourceStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	source, err := device.GetSource()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get source: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Current source: %s", source)), nil
}

func handleSourceList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	sources := nadapi.GetAvailableSources()
	return mcp.NewToolResultText(fmt.Sprintf("Available sources: %s", strings.Join(sources, ", "))), nil
}

// Brightness Control Handlers
func handleBrightnessSet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	level, err := request.RequireFloat("level")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid level parameter: %v", err)), nil
	}

	levelInt := int(level)
	if !nadapi.IsValidBrightnessLevel(levelInt) {
		return mcp.NewToolResultError("Brightness level must be between 0 and 3"), nil
	}

	if err := device.SetBrightness(levelInt); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to set brightness: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Brightness set to level %d", levelInt)), nil
}

func handleBrightnessUp(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	if err := device.ToggleBrightness(nadapi.DirectionUp); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to increase brightness: %v", err)), nil
	}

	// Get new brightness
	brightness, err := device.GetBrightness()
	if err != nil {
		return mcp.NewToolResultText("Brightness increased"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Brightness increased to level %s", brightness)), nil
}

func handleBrightnessDown(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	if err := device.ToggleBrightness(nadapi.DirectionDown); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to decrease brightness: %v", err)), nil
	}

	// Get new brightness
	brightness, err := device.GetBrightness()
	if err != nil {
		return mcp.NewToolResultText("Brightness decreased"), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Brightness decreased to level %s", brightness)), nil
}

func handleBrightnessStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	brightness, err := device.GetBrightness()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get brightness: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Current brightness: level %s", brightness)), nil
}

// Device Discovery and Info Handlers
func handleDiscover(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	devices, err := nadapi.DiscoverDevices(mcpDiscoverTimeout)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Device discovery failed: %v", err)), nil
	}

	if len(devices) == 0 {
		return mcp.NewToolResultText("No NAD devices found on the network"), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d NAD device(s):\n", len(devices)))
	for i, device := range devices {
		result.WriteString(fmt.Sprintf("%d. IP: %s, Model: %s, Port: %s\n",
			i+1, device.IP, device.Model, device.Port))
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleDeviceInfo(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	model, err := device.GetModel()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get device model: %v", err)), nil
	}

	result := fmt.Sprintf("Device Info:\nIP: %s\nPort: %s\nModel: %s",
		device.IP.String(), device.Port, model)

	return mcp.NewToolResultText(result), nil
}

func handleDeviceStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	device, err := getDevice()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to connect to device: %v", err)), nil
	}
	defer device.Disconnect()

	// Gather all status information
	var result strings.Builder
	result.WriteString("NAD Device Status:\n")

	// Power
	if power, err := device.GetPowerState(); err == nil {
		result.WriteString(fmt.Sprintf("Power: %s\n", power))
	}

	// Volume
	if volume, err := device.GetVolume(); err == nil {
		result.WriteString(fmt.Sprintf("Volume: %s dB\n", volume))
	}

	// Mute
	if mute, err := device.GetMuteStatus(); err == nil {
		result.WriteString(fmt.Sprintf("Mute: %s\n", mute))
	}

	// Source
	if source, err := device.GetSource(); err == nil {
		result.WriteString(fmt.Sprintf("Source: %s\n", source))
	}

	// Brightness
	if brightness, err := device.GetBrightness(); err == nil {
		result.WriteString(fmt.Sprintf("Brightness: level %s\n", brightness))
	}

	// Model
	if model, err := device.GetModel(); err == nil {
		result.WriteString(fmt.Sprintf("Model: %s\n", model))
	}

	result.WriteString(fmt.Sprintf("IP: %s, Port: %s", device.IP.String(), device.Port))

	return mcp.NewToolResultText(result.String()), nil
}

func registerNADResources(s *server.MCPServer) {
	// Device status resource
	s.AddResource(
		mcp.NewResource(
			"nad://device/status",
			"NAD Device Status",
			mcp.WithResourceDescription("Real-time status of the NAD audio device"),
			mcp.WithMIMEType("application/json"),
		),
		handleDeviceStatusResource,
	)

	// Available sources resource
	s.AddResource(
		mcp.NewResource(
			"nad://device/sources",
			"Available Input Sources",
			mcp.WithResourceDescription("List of available input sources for the NAD device"),
			mcp.WithMIMEType("application/json"),
		),
		handleSourcesResource,
	)

	// Device capabilities resource
	s.AddResource(
		mcp.NewResource(
			"nad://device/capabilities",
			"Device Capabilities",
			mcp.WithResourceDescription("Capabilities and specifications of the NAD device"),
			mcp.WithMIMEType("application/json"),
		),
		handleCapabilitiesResource,
	)
}

func handleDeviceStatusResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	device, err := getDevice()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to device: %v", err)
	}
	defer device.Disconnect()

	// Gather status
	status := map[string]interface{}{
		"ip":   device.IP.String(),
		"port": device.Port,
	}

	if power, err := device.GetPowerState(); err == nil {
		status["power"] = power
	}
	if volume, err := device.GetVolumeFloat(); err == nil {
		status["volume_db"] = volume
	}
	if mute, err := device.GetMuteStatus(); err == nil {
		status["mute"] = mute
	}
	if source, err := device.GetSource(); err == nil {
		status["source"] = source
	}
	if brightness, err := device.GetBrightnessInt(); err == nil {
		status["brightness"] = brightness
	}
	if model, err := device.GetModel(); err == nil {
		status["model"] = model
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "nad://device/status",
			MIMEType: "application/json",
			Text:     fmt.Sprintf("%+v", status),
		},
	}, nil
}

func handleSourcesResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	sources := nadapi.GetAvailableSources()
	data := map[string]interface{}{
		"sources": sources,
		"count":   len(sources),
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "nad://device/sources",
			MIMEType: "application/json",
			Text:     fmt.Sprintf("%+v", data),
		},
	}, nil
}

func handleCapabilitiesResource(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
	capabilities := map[string]interface{}{
		"volume_range": map[string]interface{}{
			"min":  -80,
			"max":  10,
			"unit": "dB",
		},
		"brightness_range": map[string]interface{}{
			"min": 0,
			"max": 3,
		},
		"sources":      nadapi.GetAvailableSources(),
		"power_states": []string{"On", "Off"},
		"mute_states":  []string{"On", "Off"},
		"operations": []string{
			"power_control", "volume_control", "source_selection",
			"mute_control", "brightness_control", "device_discovery",
		},
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      "nad://device/capabilities",
			MIMEType: "application/json",
			Text:     fmt.Sprintf("%+v", capabilities),
		},
	}, nil
}

func registerNADPrompts(s *server.MCPServer) {
	// Audio setup prompt
	s.AddPrompt(
		mcp.NewPrompt("nad_audio_setup",
			mcp.WithPromptDescription("Help set up optimal audio settings for the NAD device"),
			mcp.WithArgument("listening_type",
				mcp.ArgumentDescription("Type of listening: music, movies, or casual"),
			),
		),
		handleAudioSetupPrompt,
	)

	// Troubleshooting prompt
	s.AddPrompt(
		mcp.NewPrompt("nad_troubleshoot",
			mcp.WithPromptDescription("Help troubleshoot NAD device issues"),
			mcp.WithArgument("issue",
				mcp.ArgumentDescription("Description of the issue you're experiencing"),
			),
		),
		handleTroubleshootPrompt,
	)

	// Quick control prompt
	s.AddPrompt(
		mcp.NewPrompt("nad_quick_control",
			mcp.WithPromptDescription("Quick access to common NAD device operations"),
		),
		handleQuickControlPrompt,
	)
}

func handleAudioSetupPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	listeningType := request.Params.Arguments["listening_type"]
	if listeningType == "" {
		listeningType = "general"
	}

	var recommendations string
	switch strings.ToLower(listeningType) {
	case "music":
		recommendations = `For music listening:
1. Use high-quality sources (Opt1/Opt2 for digital, Phono for vinyl)
2. Set volume to comfortable level (-20 to -10 dB typical)
3. Ensure mute is off
4. Set brightness to personal preference (level 1-2 recommended)`
	case "movies":
		recommendations = `For movie watching:
1. Use TV or Coax inputs for best compatibility
2. Higher volume may be needed (-15 to -5 dB)
3. Ensure clear source connection
4. Consider lower brightness (level 0-1) for dark rooms`
	default:
		recommendations = `General audio setup:
1. Choose appropriate source based on your input
2. Start with moderate volume (-25 dB) and adjust
3. Check that device is powered on and not muted
4. Set comfortable brightness level (0-3)`
	}

	return mcp.NewGetPromptResult(
		fmt.Sprintf("NAD Audio Setup for %s", listeningType),
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent("I need help setting up my NAD audio device for optimal sound."),
			),
			mcp.NewPromptMessage(
				mcp.RoleAssistant,
				mcp.NewTextContent(recommendations),
			),
		},
	), nil
}

func handleTroubleshootPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	issue := request.Params.Arguments["issue"]

	troubleshootingSteps := `NAD Device Troubleshooting:

Common issues and solutions:
1. No sound: Check power state, volume level, mute status, and source selection
2. Low volume: Increase volume, check mute status
3. Wrong input: Use source selection tools to switch inputs
4. Display too dim/bright: Adjust brightness level
5. Device not responding: Check network connection, try device discovery

Available diagnostic tools:
- nad_device_status: Get comprehensive device status
- nad_discover: Find devices on network
- nad_power_status: Check power state
- nad_volume_status: Check current volume
- nad_source_status: Check current input source`

	var content string
	if issue != "" {
		content = fmt.Sprintf("Issue reported: %s\n\n%s", issue, troubleshootingSteps)
	} else {
		content = troubleshootingSteps
	}

	return mcp.NewGetPromptResult(
		"NAD Device Troubleshooting",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent("I'm having issues with my NAD audio device."),
			),
			mcp.NewPromptMessage(
				mcp.RoleAssistant,
				mcp.NewTextContent(content),
			),
		},
	), nil
}

func handleQuickControlPrompt(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	return mcp.NewGetPromptResult(
		"NAD Quick Control",
		[]mcp.PromptMessage{
			mcp.NewPromptMessage(
				mcp.RoleUser,
				mcp.NewTextContent("I want to quickly control my NAD audio device."),
			),
			mcp.NewPromptMessage(
				mcp.RoleAssistant,
				mcp.NewTextContent(`Quick NAD controls available:

Power: nad_power_on, nad_power_off, nad_power_toggle
Volume: nad_volume_up, nad_volume_down, nad_volume_set
Sources: nad_source_next, nad_source_previous, nad_source_set
Mute: nad_mute_toggle
Brightness: nad_brightness_up, nad_brightness_down, nad_brightness_set
Status: nad_device_status

Just tell me what you'd like to do and I'll use the appropriate tool!`),
			),
		},
	), nil
}

func handleSpotifyDevicesList(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := getMCPSpotifyClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get Spotify client: %v", err)), nil
	}

	devices, err := client.GetAvailableDevices()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get devices: %v", err)), nil
	}

	if len(devices) == 0 {
		return mcp.NewToolResultText("No Spotify Connect devices found. Make sure you have Spotify open on at least one device."), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d Spotify Connect device(s):\n", len(devices)))
	for i, device := range devices {
		status := ""
		if device.IsActive {
			status = " (Active)"
		}
		if device.IsRestricted {
			status += " (Restricted)"
		}

		// Device type icon
		icon := getDeviceTypeIcon(device.Type)

		result.WriteString(fmt.Sprintf("%d. %s %s (%s) - Volume: %d%%%s\n",
			i+1, icon, device.Name, device.Type, device.VolumePercent, status))
	}

	return mcp.NewToolResultText(result.String()), nil
}

func handleSpotifyTransferPlayback(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := getMCPSpotifyClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get Spotify client: %v", err)), nil
	}

	deviceIdentifier, err := request.RequireString("device_identifier")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid device_identifier parameter: %v", err)), nil
	}

	// The play parameter is optional, defaults to false
	play := false
	// Note: Since play is optional and boolean handling is complex in MCP,
	// we'll document that users should set play=true in the tool call if they want to start playing

	// Get available devices to find the target device
	devices, err := client.GetAvailableDevices()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get available devices: %v", err)), nil
	}

	if len(devices) == 0 {
		return mcp.NewToolResultError("No devices found. Make sure you have Spotify open on at least one device."), nil
	}

	var selectedDevice *spotify.Device

	// Try to parse as index first
	if index, err := strconv.Atoi(deviceIdentifier); err == nil {
		if index < 1 || index > len(devices) {
			return mcp.NewToolResultError(fmt.Sprintf("Invalid device index %d. Available devices: 1-%d", index, len(devices))), nil
		}
		selectedDevice = &devices[index-1]
	} else {
		// Search by name (case-insensitive partial match)
		deviceIdentifier = strings.ToLower(deviceIdentifier)
		for _, device := range devices {
			if strings.Contains(strings.ToLower(device.Name), deviceIdentifier) {
				selectedDevice = &device
				break
			}
		}
	}

	if selectedDevice == nil {
		return mcp.NewToolResultError(fmt.Sprintf("Device '%s' not found. Use spotify_devices_list to see available devices.", deviceIdentifier)), nil
	}

	// Check if already active
	if selectedDevice.IsActive {
		return mcp.NewToolResultText(fmt.Sprintf("Device '%s' is already the active playback device.", selectedDevice.Name)), nil
	}

	// Check if device is restricted
	if selectedDevice.IsRestricted {
		return mcp.NewToolResultError(fmt.Sprintf("Device '%s' does not allow remote control.", selectedDevice.Name)), nil
	}

	// Transfer playback
	err = client.TransferPlaybackToDevice(selectedDevice.ID, play)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to transfer playback to %s: %v", selectedDevice.Name, err)), nil
	}

	playStatus := ""
	if play {
		playStatus = " and started playing"
	}

	return mcp.NewToolResultText(fmt.Sprintf("Successfully transferred playback to %s (%s)%s", selectedDevice.Name, selectedDevice.Type, playStatus)), nil
}

func handleSpotifyPlay(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := getMCPSpotifyClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get Spotify client: %v", err)), nil
	}

	if err := client.Play(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to start playback: %v", err)), nil
	}

	return mcp.NewToolResultText("Spotify playback started"), nil
}

func handleSpotifyPause(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := getMCPSpotifyClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get Spotify client: %v", err)), nil
	}

	if err := client.Pause(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to pause playback: %v", err)), nil
	}

	return mcp.NewToolResultText("Spotify playback paused"), nil
}

func handleSpotifyNext(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := getMCPSpotifyClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get Spotify client: %v", err)), nil
	}

	if err := client.Next(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to skip to next track: %v", err)), nil
	}

	return mcp.NewToolResultText("Skipped to next track"), nil
}

func handleSpotifyPrevious(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := getMCPSpotifyClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get Spotify client: %v", err)), nil
	}

	if err := client.Previous(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to skip to previous track: %v", err)), nil
	}

	return mcp.NewToolResultText("Skipped to previous track"), nil
}

func handleSpotifyVolumeSet(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := getMCPSpotifyClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get Spotify client: %v", err)), nil
	}

	volume, err := request.RequireFloat("volume")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Invalid volume parameter: %v", err)), nil
	}

	if volume < 0 || volume > 100 {
		return mcp.NewToolResultError("Volume must be between 0 and 100"), nil
	}

	if err := client.SetVolume(int(volume)); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to set volume: %v", err)), nil
	}

	return mcp.NewToolResultText(fmt.Sprintf("Spotify volume set to %.0f%%", volume)), nil
}

func handleSpotifyShuffleToggle(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := getMCPSpotifyClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get Spotify client: %v", err)), nil
	}

	if err := client.ToggleShuffle(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to toggle shuffle: %v", err)), nil
	}

	return mcp.NewToolResultText("Spotify shuffle mode toggled"), nil
}

func handleSpotifyStatus(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	client, err := getMCPSpotifyClient()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get Spotify client: %v", err)), nil
	}

	state, err := client.GetPlaybackState()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to get Spotify status: %v", err)), nil
	}

	var result strings.Builder
	result.WriteString("Spotify Playback Status:\n")
	result.WriteString(fmt.Sprintf("Track: %s\n", state.Track.Name))
	result.WriteString(fmt.Sprintf("Artist: %s\n", state.Track.Artist))
	result.WriteString(fmt.Sprintf("Album: %s\n", state.Track.Album))
	result.WriteString(fmt.Sprintf("Device: %s\n", state.Device))
	result.WriteString(fmt.Sprintf("Playing: %t\n", state.IsPlaying))
	result.WriteString(fmt.Sprintf("Volume: %d%%\n", state.Volume))
	result.WriteString(fmt.Sprintf("Shuffle: %t\n", state.Shuffle))
	result.WriteString(fmt.Sprintf("Repeat: %s\n", state.Repeat))

	return mcp.NewToolResultText(result.String()), nil
}

// Helper function to get device type icons
func getDeviceTypeIcon(deviceType string) string {
	switch strings.ToLower(deviceType) {
	case "computer":
		return "💻"
	case "smartphone":
		return "📱"
	case "speaker":
		return "🔊"
	case "tv":
		return "📺"
	case "cast_video", "chromecast":
		return "📺"
	case "audio_dongle":
		return "🎧"
	case "game_console":
		return "🎮"
	default:
		return "🎵"
	}
}
