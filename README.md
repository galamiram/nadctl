# nadctl

A lightweight application for controlling NAD amplifiers. It contains both GUI and CLI with automatic device discovery and caching.

## Features
- **Automatic Device Discovery**: Automatically finds NAD devices on your network
- **Smart Caching**: Caches discovery results for faster subsequent operations (5-minute TTL)
- **Command Line Interface**: Control your NAD device via CLI commands
- **Multiple Device Support**: Handles multiple devices on the network
- **Configuration Support**: Use config files or environment variables

## Supported Devices
- NAD C338

## Installation

```bash
go build -o nadctl
```

## Usage

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

### Cache Details

- **Cache File**: `~/.nadctl_cache.json`
- **Default TTL**: 5 minutes
- **Automatic**: Discovery results are automatically cached
- **Fallback**: If cache is invalid or missing, performs fresh network scan
- **Performance**: Cached results load in ~50ms vs 10-30s for network scan

### Debug Mode

Enable debug logging:
```bash
./nadctl --debug power
```

This will show whether devices were loaded from cache or discovered via network scan.
