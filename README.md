# nadctl

A lightweight application for controlling NAD amplifiers with **Spotify device selection and casting** capabilities. It contains both a **Terminal User Interface (TUI)**, **Command Line Interface (CLI)**, **Model Context Protocol (MCP) Server**, and a **device simulator** with automatic device discovery and caching.

## 🚀 Installation

### via Homebrew (macOS & Linux)

```bash
# Add the tap
brew tap galamiram/tap

# Install nadctl
brew install nadctl

# Verify installation
nadctl --help
```

### via Script (All Platforms)

```bash
# One-liner install
curl -sf https://raw.githubusercontent.com/galamiram/nadctl/main/install.sh | sh

# Or download and run manually
wget https://raw.githubusercontent.com/galamiram/nadctl/main/install.sh
chmod +x install.sh
./install.sh
```

### via GitHub Releases

1. Go to [Releases](https://github.com/galamiram/nadctl/releases)
2. Download the appropriate binary for your platform
3. Extract and move to your PATH:

```bash
# Example for macOS
tar -xzf nadctl_v1.0.0_darwin_amd64.tar.gz
sudo mv nadctl /usr/local/bin/
```

### from Source

```bash
# Requires Go 1.21+
git clone https://github.com/galamiram/nadctl.git
cd nadctl

# Build with make (recommended - includes version injection)
make build

# Or build manually
go build -o nadctl .

# To inject version from VERSION file
make build
# This is equivalent to:
# go build -ldflags "-X github.com/galamiram/nadctl/internal/version.Version=$(cat VERSION | tr -d '\n')" -o nadctl .
```

## Features
- **Terminal User Interface (TUI)**: Interactive real-time control with visual feedback
- **🎵 Spotify Device Casting**: Discover and cast to Chromecast, computers, speakers and other Spotify Connect devices
- **Model Context Protocol (MCP) Server**: LLM integration for AI-powered audio control
- **Device Simulator**: Built-in NAD device simulator for testing without hardware
- **Automatic Device Discovery**: Automatically finds NAD devices on your network
- **Smart Caching**: Caches discovery results for faster subsequent operations (5-minute TTL)
- **Command Line Interface**: Control your NAD device via CLI commands
- **Multiple Device Support**: Handles multiple devices on the network
- **Configuration Support**: Use config files or environment variables

## Model Context Protocol (MCP) Server

**NEW**: Control your NAD device using AI assistants like Cursor, Claude Desktop, or any MCP-compatible LLM tool!

The MCP server allows LLMs to control your NAD audio device through a standardized protocol. This enables natural language control of your audio system.

### Quick Start with MCP

1. **Start the MCP server:**
   ```bash
   # Auto-discover device
   nadctl mcp
   
   # Or specify device IP
   NAD_IP=192.168.1.100 nadctl mcp
   
   # Or use command flags
   nadctl mcp --device-ip 192.168.1.100
   ```

2. **Configure your AI tool** (see sections below for specific tools)

3. **Start controlling with natural language:**
   - "Turn on my NAD device"
   - "Set volume to -25 dB"
   - "Switch to TV input"
   - "What's the current status of my audio system?"

### Available MCP Tools

The MCP server provides these tools for LLM use:

#### Power Control
- `nad_power_on` - Turn on the device
- `nad_power_off` - Turn off the device  
- `nad_power_toggle` - Toggle power state
- `nad_power_status` - Get current power state

#### Volume Control
- `nad_volume_set` - Set specific volume level (dB)
- `nad_volume_up` - Increase volume
- `nad_volume_down` - Decrease volume
- `nad_volume_status` - Get current volume
- `nad_mute_toggle` - Toggle mute state
- `nad_mute_status` - Get mute status

#### Source Control
- `nad_source_set` - Set input source (Stream, TV, etc.)
- `nad_source_next` - Switch to next source
- `nad_source_previous` - Switch to previous source
- `nad_source_status` - Get current source
- `nad_source_list` - List available sources

#### Brightness Control
- `nad_brightness_set` - Set display brightness (0-3)
- `nad_brightness_up` - Increase brightness
- `nad_brightness_down` - Decrease brightness
- `nad_brightness_status` - Get current brightness

#### Device Information
- `nad_discover` - Find NAD devices on network
- `nad_device_info` - Get device information
- `nad_device_status` - Get comprehensive device status

#### 🎯 Spotify Device Casting & Control
- `spotify_devices_list` - List all available Spotify Connect devices (Chromecast, computers, speakers, phones)
- `spotify_transfer_playback` - Transfer Spotify playback to a specific device by name or index
- `spotify_play` - Start or resume Spotify playback
- `spotify_pause` - Pause Spotify playback
- `spotify_next` - Skip to next track
- `spotify_previous` - Skip to previous track
- `spotify_volume_set` - Set Spotify volume (0-100%)
- `spotify_shuffle_toggle` - Toggle shuffle mode
- `spotify_status` - Get current Spotify playback status and device info

### MCP Resources

The server also provides these data resources:

- `nad://device/status` - Real-time device status (JSON)
- `nad://device/sources` - Available input sources (JSON)
- `nad://device/capabilities` - Device capabilities and specifications (JSON)

### MCP Prompts

Pre-configured conversation starters:

- `nad_audio_setup` - Audio setup assistance
- `nad_troubleshoot` - Device troubleshooting help
- `nad_quick_control` - Quick control access

### Setup with Cursor

1. **Get the path to your nadctl binary:**
   ```bash
   which nadctl  # If installed via Homebrew or script
   # or if built from source: pwd && ls -la nadctl
   ```

2. **Open Cursor Settings:**
   - Click the settings cog (⚙️) in the top right
   - Navigate to "MCP" section

3. **Add MCP Server:**
   - Click "Add MCP Server"
   - Name: `NAD Audio Controller`
   - Path: `/opt/homebrew/bin/nadctl` (or your path from step 1)
   - Args: `["mcp"]`
   - Environment Variables (optional):
     - `NAD_IP`: `192.168.1.100` (your device IP)

4. **Save and Test:**
   - Click "Save"
   - Click "Refresh"
   - Look for green dot indicating successful connection

### Setup with Claude Desktop

1. **Find or create Claude Desktop config file:**
   ```bash
   # macOS
   ~/Library/Application Support/Claude/claude_desktop_config.json
   
   # Windows
   %APPDATA%\Claude\claude_desktop_config.json
   
   # Linux
   ~/.config/Claude/claude_desktop_config.json
   ```

2. **Add this configuration:**
   ```json
   {
     "mcpServers": {
       "nad-controller": {
         "command": "/opt/homebrew/bin/nadctl",
         "args": ["mcp"],
         "env": {
           "NAD_IP": "192.168.1.100"
         }
       }
     }
   }
   ```

3. **Restart Claude Desktop**

### Setup with Other MCP Tools

For other MCP-compatible tools, use this general configuration:

- **Command:** `/opt/homebrew/bin/nadctl` (or your installation path)
- **Arguments:** `["mcp"]`
- **Transport:** stdio
- **Environment Variables:**
  - `NAD_IP`: Your device IP (optional, will auto-discover if not set)
  - `NAD_PORT`: Device port (optional, defaults to 30001)
  - `SPOTIFY_CLIENT_ID`: Your Spotify client ID for device casting (optional, see [SPOTIFY_SETUP.md](SPOTIFY_SETUP.md))

**Note for Spotify Features:** To use Spotify device casting and control tools, you need to:
1. Set up a Spotify application and get your Client ID (see [SPOTIFY_SETUP.md](SPOTIFY_SETUP.md))
2. Configure your Client ID in `~/.nadctl.yaml` or set the `SPOTIFY_CLIENT_ID` environment variable
3. Authenticate using the TUI (`nadctl tui`) or CLI (`nadctl spotify connect`) first

### Example Usage with AI

Once configured, you can control your NAD device and Spotify naturally:

**User:** "Turn on my audio system and set it to TV input"

**AI:** I'll turn on your NAD device and switch to TV input.

*[Uses nad_power_on and nad_source_set tools]*

Power: On ✓
Source: TV ✓

**User:** "What's the current volume level?"

**AI:** *[Uses nad_volume_status tool]*

The current volume is -30 dB.

**User:** "That's too quiet, make it a bit louder"

**AI:** *[Uses nad_volume_up tool]*

I've increased the volume to -25 dB. The audio should be louder now.

**User:** "Show me my Spotify devices and cast to my living room Chromecast"

**AI:** *[Uses spotify_devices_list and spotify_transfer_playback tools]*

Available Spotify devices:
1. 💻 MacBook Pro (Active) - Volume: 75%
2. 📺 Living Room Chromecast - Volume: 80% 
3. 🔊 Kitchen Speaker - Volume: 60%
4. 📱 iPhone - Volume: 50% (Restricted)

Successfully transferred playback to Living Room Chromecast (cast_video)

**User:** "Start playing music and increase the Spotify volume"

**AI:** *[Uses spotify_play and spotify_volume_set tools]*

Spotify playback started
Spotify volume set to 85%

### Troubleshooting MCP

1. **Server won't start:**
   - Check that nadctl binary is executable
   - Verify device IP/connectivity if specified
   - Check logs in your AI tool

2. **Tools not working:**
   - Ensure NAD device is on the network
   - Test regular nadctl commands first
   - Check device IP and port settings

3. **Connection issues:**
   - Verify config file syntax (JSON)
   - Check file paths are absolute
   - Restart your AI tool after config changes

## Supported Devices
- NAD C338
- NAD T 758 V3i (simulated)

## Usage

### Terminal User Interface (TUI)

Launch the interactive terminal interface:

```bash
# Auto-discover and connect to first NAD device
nadctl tui

# Connect to specific device
NAD_IP=192.168.1.100 nadctl tui

# Use with simulator
NAD_IP=127.0.0.1 nadctl tui
```

#### TUI Controls:
- **p** - Toggle power on/off
- **m** - Toggle mute
- **+/-** - Volume up/down
- **←/→** - Previous/next source
- **↑/↓** - Brightness up/down
- **r** - Refresh device status
- **d** - Discover devices
- **?** - Show help
- **q** - Quit

#### Spotify Controls (when configured):
- **t** - Toggle Spotify panel visibility
- **space** - Play/pause current Spotify track
- **n** - Next Spotify track
- **b** - Previous Spotify track
- **s** - Toggle shuffle mode
- **y** - List Spotify devices and select device to cast to
- **↑/↓** - Navigate device selection (when in device mode)
- **Enter** - Cast to selected device

#### TUI Features:
- 🔴🟢 Real-time connection status indicators
- 📊 Visual progress bars for volume and brightness
- 🎨 Color-coded status messages
- ⚡ Auto-refresh every 10 seconds
- 🖥️ Multi-panel layout with device information
- 🎵 **Spotify integration** with now playing info, playback controls, and **device casting**
- 📱 **Spotify Connect Device Management**: Visual device selection with type icons (💻 🔊 📺 📱 🎵 🎧)

### Device Simulator

For testing without real NAD hardware:

```bash
# Start simulator on default port (30001)
nadctl simulator

# Start on custom port
nadctl simulator --port 8080

# In another terminal, connect TUI to simulator
NAD_IP=127.0.0.1 nadctl tui

# Or test with CLI commands
NAD_IP=127.0.0.1 nadctl power
NAD_IP=127.0.0.1 nadctl volume up
```

#### Simulator Features:
- 🎵 Complete NAD protocol simulation
- 📊 Realistic device state management
- 🔧 Multiple client connection support
- ⚙️ Configurable device properties
- 📝 Debug logging for development

### Automatic Discovery with Caching
By default, `nadctl` will automatically scan your network for NAD devices and cache the results:

```bash
# First run: scans network and caches results
nadctl power

# Subsequent runs: uses cached results (much faster)
nadctl volume up

# Discover all devices on network
nadctl discover

# Force refresh cache by rescanning network
nadctl discover --refresh

# Show cache status
nadctl discover --show-cache

# Clear cache
nadctl --clear-cache
```

### Cache Management

```bash
# Disable cache for a single command
nadctl --no-cache power

# Clear cache and exit
nadctl --clear-cache

# Force refresh discovery cache
nadctl discover --refresh

# View cache status and information
nadctl discover --show-cache
```

### Manual Configuration
You can specify a device IP address in several ways:

```bash
# Environment variable
export NAD_IP=192.168.1.100
nadctl power

# Config file (~/.nadctl.yaml)
ip: 192.168.1.100
```

### Available Commands

```bash
# Terminal User Interface
nadctl tui                         # Launch interactive TUI

# Model Context Protocol Server
nadctl mcp                         # Start MCP server for LLM integration
nadctl mcp --device-ip 192.168.1.100  # Start MCP server with specific device

# Device simulator
nadctl simulator                   # Start NAD device simulator
nadctl simulator --port 8080      # Start simulator on custom port

# Version information
nadctl version                     # Show version information

# Device discovery
nadctl discover                    # List all NAD devices on network
nadctl discover --refresh          # Force network rescan
nadctl discover --show-cache       # Show cached devices
nadctl discover --timeout 60s     # Set discovery timeout

# Power control
nadctl power                       # Toggle power on/off

# Volume control
nadctl volume                      # Show current volume
nadctl volume set -20              # Set volume to -20 dB (recommended for negative)
nadctl volume -- -20               # Alternative syntax for negative volumes
nadctl volume 0                    # Set volume to 0 dB (reference level)
nadctl volume up                   # Increase volume by 1 dB
nadctl volume down                 # Decrease volume by 1 dB

# Volume range is typically -80 to +10 dB

# Source control
nadctl source                      # Show current source
nadctl source list                 # List all available sources
nadctl source Stream               # Set source to Stream
nadctl source tv                   # Set source to TV (case-insensitive)
nadctl source next                 # Switch to next source
nadctl source prev                 # Switch to previous source

# Available sources: Stream, Wireless, TV, Phono, Coax1, Coax2, Opt1, Opt2

# Spotify device casting (when configured)
nadctl spotify devices             # List available Spotify Connect devices
nadctl spotify transfer "Chromecast"  # Cast to device by name
nadctl spotify transfer 1          # Cast to device by index number
nadctl spotify transfer --play 1   # Cast to device and start playing

# Mute control
nadctl mute                        # Toggle mute

# Display brightness
nadctl dim                         # Show current brightness
nadctl dim 0                       # Set brightness to 0 (display off)
nadctl dim 2                       # Set brightness to 2 (medium)
nadctl dim up                      # Increase brightness
nadctl dim down                    # Decrease brightness
nadctl dim list                    # List all available levels

# Brightness levels: 0 (off), 1 (low), 2 (medium), 3 (high)
```

### Configuration

Create a config file at `~/.nadctl.yaml`:

```yaml
ip: 192.168.1.100
```

Or use environment variables:
```bash
export NAD_IP=192.168.1.100
export NAD_DEBUG=true
```

### Development & Testing

#### Available Make Targets

```bash
make build         # Build with version injection from VERSION file
make version       # Build and show version
make test          # Run all tests
make demo          # Build and run TUI in demo mode
make install       # Build and install to /usr/local/bin
make clean         # Clean build artifacts
```

#### Using the Simulator
The built-in simulator is perfect for development and testing:

```bash
# Terminal 1: Start simulator
nadctl simulator

# Terminal 2: Test TUI
NAD_IP=127.0.0.1 nadctl tui

# Terminal 3: Test CLI commands
NAD_IP=127.0.0.1 nadctl power
NAD_IP=127.0.0.1 nadctl volume set -25
NAD_IP=127.0.0.1 nadctl source Stream
```

#### Debug Mode
Enable debug logging:
```bash
nadctl --debug power
LOG_LEVEL=debug nadctl simulator
```

This will show whether devices were loaded from cache or discovered via network scan.

### Cache Details

- **Cache File**: `~/.nadctl_cache.json`
- **Default TTL**: 5 minutes
- **Automatic**: Discovery results are automatically cached
- **Fallback**: If cache is invalid or missing, performs fresh network scan
- **Performance**: Cached results load in ~50ms vs 10-30s for network scan

### Requirements

- Network connectivity to NAD devices
- Terminal with color support (for best TUI experience)

### Spotify Integration Setup

The TUI now includes optional Spotify integration with **device selection and casting** to show currently playing music, control playback, and **cast to any Spotify Connect device** alongside your NAD device controls.

#### Prerequisites
- **Spotify Premium Account**: Required for playback control and device casting
- **Spotify Developer App**: You need API credentials

#### What You Get
- **Now Playing Info**: Track, artist, album, progress
- **Playback Controls**: Play/pause, next/previous, shuffle
- **🎯 Device Casting**: Discover and cast to Chromecast, computers, speakers, phones, and other Spotify Connect devices
- **📱 Visual Device Selection**: Interactive device picker with type icons and status
- **🎵 Multi-Device Support**: Control playback across your entire Spotify ecosystem
- **Visual Progress**: Track progress bar
- **Dual Control**: Independent NAD device and Spotify control
- **Auto-refresh**: Real-time updates every 5 seconds

#### Device Selection Features
- **Device Discovery**: Automatically finds all your Spotify Connect devices
- **Device Types**: Supports computers (💻), speakers (🔊), TVs/Chromecast (📺), phones (📱), and more
- **Visual Selection**: Navigate with ↑↓ keys and Enter to cast
- **Active Status**: Shows which device is currently playing
- **Volume Display**: Shows current volume level for each device
- **Type Icons**: Visual indicators for different device types

#### Quick Setup

1. **Create Spotify App**:
   - Visit [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
   - Create a new app (select "Desktop" app type for PKCE support)
   - Note your **Client ID** (no client secret needed!)
   - Add `http://localhost:8080/callback` as a redirect URI

2. **Configure NAD Controller**:
   ```bash
   # Copy example config
   cp config.example.yaml ~/.nadctl.yaml
   
   # Edit config with your Spotify Client ID
   nano ~/.nadctl.yaml
   ```

3. **Add your Client ID to `~/.nadctl.yaml`**:
   ```yaml
   spotify:
     client_id: "your_actual_client_id_here"
     redirect_url: "http://localhost:8080/callback"  # optional
   ```

4. **Launch TUI and authenticate**:
   ```bash
   ./nadctl tui
   # Press 'a' to start Spotify authentication
   # Browser opens automatically - authorize the app
   # Copy the authorization code from the redirect URL back to the app
   ```

5. **Start casting to devices**:
   ```bash
   # In the TUI: Press 'y' to show Spotify devices
   # Use ↑↓ to select a device, Enter to cast
   
   # Or use CLI:
   ./nadctl spotify devices
   ./nadctl spotify transfer "Living Room Chromecast"
   ```

#### Security Note
This implementation uses **PKCE (Proof Key for Code Exchange)** flow, which is the recommended OAuth2 flow for native/desktop applications. Unlike server-side apps, **no client secret is needed**, making it more secure for client-side applications.

For detailed setup instructions, see [SPOTIFY_SETUP.md](SPOTIFY_SETUP.md).

### Version Information

Check the version of your installation:
```bash
nadctl version
```

For release notes and version history, see [CHANGELOG.md](CHANGELOG.md).

### Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Logrus](https://github.com/sirupsen/logrus) - Structured logging
- [MCP Go](https://github.com/mark3labs/mcp-go) - Model Context Protocol
- [Spotify Web API SDK](https://github.com/zmb3/spotify) - Spotify integration

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgements

- NAD Electronics for their excellent audio equipment
- The Go community for amazing libraries
- [Charm](https://charm.sh/) for the beautiful TUI framework
- [Mark3Labs](https://github.com/mark3labs) for the MCP Go implementation