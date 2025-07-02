# nadctl

A lightweight application for controlling NAD amplifiers. It contains both a **Terminal User Interface (TUI)**, **Command Line Interface (CLI)**, **Model Context Protocol (MCP) Server**, and a **device simulator** with automatic device discovery and caching.

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
go build -o nadctl
```

## Features
- **Terminal User Interface (TUI)**: Interactive real-time control with visual feedback
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

### Example Usage with AI

Once configured, you can control your NAD device naturally:

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

#### TUI Features:
- 🔴🟢 Real-time connection status indicators
- 📊 Visual progress bars for volume and brightness
- 🎨 Color-coded status messages
- ⚡ Auto-refresh every 10 seconds
- 🖥️ Multi-panel layout with device information

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

### Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Logrus](https://github.com/sirupsen/logrus) - Structured logging
- [MCP Go](https://github.com/mark3labs/mcp-go) - Model Context Protocol

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
