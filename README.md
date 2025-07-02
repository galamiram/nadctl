# nadctl

A lightweight application for controlling NAD amplifiers. It contains both a **Terminal User Interface (TUI)**, **Command Line Interface (CLI)**, and a **device simulator** with automatic device discovery and caching.

## Features
- **Terminal User Interface (TUI)**: Interactive real-time control with visual feedback
- **Device Simulator**: Built-in NAD device simulator for testing without hardware
- **Automatic Device Discovery**: Automatically finds NAD devices on your network
- **Smart Caching**: Caches discovery results for faster subsequent operations (5-minute TTL)
- **Command Line Interface**: Control your NAD device via CLI commands
- **Multiple Device Support**: Handles multiple devices on the network
- **Configuration Support**: Use config files or environment variables

## Supported Devices
- NAD C338
- NAD T 758 V3i (simulated)

## Installation

```bash
go build -o nadctl
```

## Usage

### Terminal User Interface (TUI)

Launch the interactive terminal interface:

```bash
# Auto-discover and connect to first NAD device
./nadctl tui

# Connect to specific device
NAD_IP=192.168.1.100 ./nadctl tui

# Use with simulator
NAD_IP=127.0.0.1 ./nadctl tui
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
./nadctl simulator

# Start on custom port
./nadctl simulator --port 8080

# In another terminal, connect TUI to simulator
NAD_IP=127.0.0.1 ./nadctl tui

# Or test with CLI commands
NAD_IP=127.0.0.1 ./nadctl power
NAD_IP=127.0.0.1 ./nadctl volume up
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
./nadctl power

# Subsequent runs: uses cached results (much faster)
./nadctl volume up

# Discover all devices on network
./nadctl discover

# Force refresh cache by rescanning network
./nadctl discover --refresh

# Show cache status
./nadctl discover --show-cache

# Clear cache
./nadctl --clear-cache
```

### Cache Management

```bash
# Disable cache for a single command
./nadctl --no-cache power

# Clear cache and exit
./nadctl --clear-cache

# Force refresh discovery cache
./nadctl discover --refresh

# View cache status and information
./nadctl discover --show-cache
```

### Manual Configuration
You can specify a device IP address in several ways:

```bash
# Environment variable
export NAD_IP=192.168.1.100
./nadctl power

# Config file (~/.nadctl.yaml)
ip: 192.168.1.100
```

### Available Commands

```bash
# Terminal User Interface
./nadctl tui                         # Launch interactive TUI

# Device simulator
./nadctl simulator                   # Start NAD device simulator
./nadctl simulator --port 8080      # Start simulator on custom port

# Device discovery
./nadctl discover                    # List all NAD devices on network
./nadctl discover --refresh          # Force network rescan
./nadctl discover --show-cache       # Show cached devices
./nadctl discover --timeout 60s     # Set discovery timeout

# Power control
./nadctl power                       # Toggle power on/off

# Volume control
./nadctl volume                      # Show current volume
./nadctl volume set -20              # Set volume to -20 dB (recommended for negative)
./nadctl volume -- -20               # Alternative syntax for negative volumes
./nadctl volume 0                    # Set volume to 0 dB (reference level)
./nadctl volume up                   # Increase volume by 1 dB
./nadctl volume down                 # Decrease volume by 1 dB

# Volume range is typically -80 to +10 dB

# Source control
./nadctl source                      # Show current source
./nadctl source list                 # List all available sources
./nadctl source Stream               # Set source to Stream
./nadctl source tv                   # Set source to TV (case-insensitive)
./nadctl source next                 # Switch to next source
./nadctl source prev                 # Switch to previous source

# Available sources: Stream, Wireless, TV, Phono, Coax1, Coax2, Opt1, Opt2

# Mute control
./nadctl mute                        # Toggle mute

# Display brightness
./nadctl dim                         # Show current brightness
./nadctl dim 0                       # Set brightness to 0 (display off)
./nadctl dim 2                       # Set brightness to 2 (medium)
./nadctl dim up                      # Increase brightness
./nadctl dim down                    # Decrease brightness
./nadctl dim list                    # List all available levels

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
./nadctl simulator

# Terminal 2: Test TUI
NAD_IP=127.0.0.1 ./nadctl tui

# Terminal 3: Test CLI commands
NAD_IP=127.0.0.1 ./nadctl power
NAD_IP=127.0.0.1 ./nadctl volume set -25
NAD_IP=127.0.0.1 ./nadctl source Stream
```

#### Debug Mode
Enable debug logging:
```bash
./nadctl --debug power
LOG_LEVEL=debug ./nadctl simulator
```

This will show whether devices were loaded from cache or discovered via network scan.

### Cache Details

- **Cache File**: `~/.nadctl_cache.json`
- **Default TTL**: 5 minutes
- **Automatic**: Discovery results are automatically cached
- **Fallback**: If cache is invalid or missing, performs fresh network scan
- **Performance**: Cached results load in ~50ms vs 10-30s for network scan

### Requirements

- Go 1.19+ (for building from source)
- Network connectivity to NAD devices
- Terminal with color support (for best TUI experience)

### Dependencies

- [Bubble Tea](https://github.com/charmbracelet/bubbletea) - TUI framework
- [Lipgloss](https://github.com/charmbracelet/lipgloss) - Terminal styling
- [Bubbles](https://github.com/charmbracelet/bubbles) - TUI components
- [Logrus](https://github.com/sirupsen/logrus) - Structured logging
