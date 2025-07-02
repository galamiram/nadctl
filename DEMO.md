# 🎵 NAD Controller TUI & Simulator Demo

## Overview

Your NAD controller now includes two powerful new features:

1. **Beautiful Terminal GUI (TUI)** - A modern, interactive interface
2. **Device Simulator** - For testing without a real NAD device

## ✨ New Features Added

### 🖥️ Terminal GUI (TUI)
- **Real-time device status** with beautiful panels and frames
- **Progress bars** for volume and brightness
- **Color-coded status indicators** (power, mute, connection)
- **Animated spinner** during connection
- **Keyboard shortcuts** for all functions
- **Auto-refresh** every 10 seconds
- **Multi-panel layout** with connection, device info, and controls

### 🎛️ Device Simulator
- **Complete NAD protocol implementation**
- **Maintains realistic device state**
- **Supports all commands** (power, volume, source, mute, brightness)
- **Multiple client connections**
- **Detailed logging** of all operations
- **Configurable port**

## 🚀 Quick Start Demo

### Step 1: Start the Simulator
```bash
# Start NAD device simulator
./nadctl simulator

# Or on a custom port
./nadctl simulator --port 30002
```

You'll see:
```
🎵 NAD Device Simulator is running!

🔗 To connect your TUI:
   NAD_IP=127.0.0.1 ./nadctl tui

🔧 To test CLI commands:
   NAD_IP=127.0.0.1 ./nadctl power
   NAD_IP=127.0.0.1 ./nadctl volume up
   NAD_IP=127.0.0.1 ./nadctl source next

⏹️  Press Ctrl+C to stop the simulator
```

### Step 2: Launch the TUI (in another terminal)
```bash
# Connect TUI to simulator
NAD_IP=127.0.0.1 ./nadctl tui

# Or use debug mode to see connection details
NAD_IP=127.0.0.1 ./nadctl tui --debug
```

## 🎮 TUI Controls

| Key | Action |
|-----|--------|
| `p` | Toggle power |
| `m` | Toggle mute |
| `+/-` | Volume up/down |
| `←/→` | Previous/next source |
| `↑/↓` | Brightness up/down |
| `r` | Refresh status |
| `d` | Discover devices |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

## 🎨 Visual Features

### Multi-Panel Layout
- **Connection Status Panel** - Shows connection state with color indicators
- **Device Information Panel** - Displays model and IP
- **Power Status Panel** - Large, prominent power state indicator
- **Audio Controls Panel** - Volume, source, and mute with progress bars
- **Display Controls Panel** - Brightness with progress bar

### Color Coding
- 🟢 **Green** - Connected, power on, unmuted
- 🔴 **Red** - Disconnected, errors, muted
- 🟡 **Yellow** - Connecting, warnings
- 🔵 **Blue** - Information, labels
- ⚫ **Gray** - Disabled states, help text

### Progress Bars
- **Volume bar** - Visual representation of volume level (-80 to +10 dB)
- **Brightness bar** - Visual representation of brightness (0-3)

## 📡 Testing All Features

### Test CLI Commands with Simulator
```bash
# Set environment variable for easy testing
export NAD_IP=127.0.0.1

# Test all commands
./nadctl power               # Toggle power
./nadctl volume up           # Increase volume
./nadctl volume set -20      # Set specific volume
./nadctl source next         # Change source
./nadctl source Stream       # Set specific source
./nadctl mute                # Toggle mute
./nadctl dim up              # Increase brightness
./nadctl discover            # Discovery (will find simulator)
```

### Watch Real-time Updates in TUI
1. Start the simulator in one terminal
2. Start the TUI in another terminal  
3. Use CLI commands in a third terminal
4. Watch the TUI update in real-time!

## 🔧 Advanced Usage

### Custom Configuration
Create `~/.nadctl.yaml`:
```yaml
ip: "192.168.1.100"  # Your real NAD device IP
debug: true          # Enable debug logging
```

### Environment Variables
```bash
export NAD_IP=192.168.1.100    # Device IP
export NAD_DEBUG=true          # Enable debug mode
```

### Discovery and Cache
```bash
./nadctl discover              # Find devices on network
./nadctl tui --clear-cache     # Clear discovery cache
./nadctl tui --no-cache        # Disable cache for this session
```

## 🎯 Production Usage

### With Real NAD Device
```bash
# Auto-discover your NAD device
./nadctl tui

# Or specify IP directly
NAD_IP=192.168.1.100 ./nadctl tui

# Or use config file
./nadctl tui --config ~/.nadctl.yaml
```

### Performance
- **Efficient updates** - Only refreshes when needed
- **Connection pooling** - Reuses connections
- **Caching** - Remembers discovered devices
- **Error handling** - Graceful failure and retry

## 🎨 TUI Screenshots (Text-based)

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          🎵 NAD Audio Controller                           │
│                   Terminal Interface for Premium Audio Control              │
└─────────────────────────────────────────────────────────────────────────────┘

┌─────────────────┐  ┌─────────────────┐
│ Connection Status│  │ Power Status    │
│                 │  │                 │
│ 🟢 Connected to │  │  POWER ON       │
│ 192.168.1.100   │  │                 │
│                 │  │ Press 'p' to    │
└─────────────────┘  │ toggle          │
                     └─────────────────┘
┌─────────────────┐  ┌─────────────────┐
│ Device Info     │  │ Audio Controls  │
│                 │  │                 │
│ Model: NAD T758 │  │ Volume: -20.0dB │
│ IP: 192.168.1.100│  │ ████████░░░░    │
│                 │  │                 │
└─────────────────┘  │ Source: Stream  │
                     │ Mute: 🔊 UNMUTED│
                     └─────────────────┘
                     ┌─────────────────┐
                     │ Display Controls│
                     │                 │
                     │ Brightness: 2   │
                     │ ██████░░░░░░    │
                     │                 │
                     │ Use ↑↓ keys     │
                     └─────────────────┘

✓ Power toggled

Last update: 14:30:25
p toggle power • m toggle mute • + volume up • - volume down • ? more • q quit
```

Enjoy your beautiful new NAD controller interface! 🎵✨ 