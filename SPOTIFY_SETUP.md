# Spotify Integration Setup Guide

This guide will help you set up Spotify integration with your NAD Audio Controller TUI, including **device selection and casting** to Chromecast, computers, speakers, and other Spotify Connect devices using the secure PKCE (Proof Key for Code Exchange) flow.

## Prerequisites

1. **Spotify Premium Account** - Required for controlling playback and casting to devices
2. **Spotify Application** - You need to create a Spotify app to get a Client ID

## Why PKCE Flow?

This integration uses PKCE (Proof Key for Code Exchange), which is the **recommended OAuth2 flow for native/desktop applications**. Key benefits:
- **No client secret needed** - More secure for client-side apps
- **Industry standard** - Recommended by OAuth2 specification for public clients
- **Better security** - Uses cryptographic proof instead of shared secrets

## Step 1: Create a Spotify Application

1. Go to [Spotify Developer Dashboard](https://developer.spotify.com/dashboard)
2. Log in with your Spotify account
3. Click **"Create App"**
4. Fill in the details:
   - **App Name**: `NAD Audio Controller` (or any name you prefer)
   - **App Description**: `Terminal interface for NAD device with Spotify integration`
   - **Website**: Leave blank or use `https://github.com/galamiram/nadctl`
   - **Redirect URI**: `http://localhost:8080/callback`
   - **App Type**: Select **"Desktop App"** (this enables PKCE support)
5. Check the box to agree to terms and click **"Create"**
6. In your new app, click **"Settings"**
7. Note down your **Client ID** (no Client Secret needed!)

## Step 2: Configure NAD Controller

1. Copy the example configuration:
   ```bash
   cp config.example.yaml ~/.nadctl.yaml
   ```

2. Edit the configuration file:
   ```bash
   nano ~/.nadctl.yaml
   ```

3. Add your Spotify Client ID:
   ```yaml
   spotify:
     client_id: "your_actual_client_id_here"
     redirect_url: "http://localhost:8080/callback"  # optional, defaults to this
   ```

## Step 3: Authenticate with Spotify

1. Start the NAD Controller TUI:
   ```bash
   ./nadctl tui
   ```

2. Press `a` to start Spotify authentication
3. **Browser opens automatically** - the app will open your default browser
4. Log in to Spotify and authorize the application
5. After authorization, Spotify will redirect you to `http://localhost:8080/callback?code=...`
6. **Copy only the code value** from the URL (the long string after `code=`)
7. Paste the code into the TUI input field and press Enter
8. Authentication complete! 🎉

### Troubleshooting Authentication

**If browser doesn't open automatically:**
- The TUI will show a fallback URL to copy and paste manually
- This can happen on some Linux systems or remote sessions

**Example of what to copy:**
```
From: http://localhost:8080/callback?code=AQC8X7Zv9...&state=nadctl-state
Copy: AQC8X7Zv9...  (only the code part)
```

## Available Controls

### Spotify Key Bindings

| Key | Action |
|-----|--------|
| `space` | Play/Pause current track |
| `n` | Next track |
| `b` | Previous track |
| `s` | Toggle shuffle mode |
| `t` | Toggle Spotify panel visibility |
| **`y`** | **List Spotify devices and enter selection mode** |
| **`↑/↓`** | **Navigate device selection (when in device mode)** |
| **`Enter`** | **Cast to selected device** |
| **`Esc`** | **Cancel device selection mode** |

### Spotify Panel Features

When visible (press `t` to toggle), the Spotify panel shows:
- **Current Track**: Song name, artist, album
- **Playback State**: Playing/paused status with icons
- **Progress**: Track progress bar and time indicators
- **Controls**: Available actions status
- **Device**: Currently active Spotify device
- **Volume**: Spotify app volume level
- **Modes**: Shuffle and repeat status with visual indicators

### NEW: Device Selection Panel

When you press `y`, the interface shows:
- **Available Devices**: All your Spotify Connect devices
- **Device Types**: Visual icons for each device type:
  - 💻 Computer/Desktop
  - 🔊 Speaker/Sound System
  - 📺 TV/Chromecast/Smart TV
  - 📱 Phone/Mobile Device
  - 🎵 Spotify Connect Device
  - 🎧 Headphones/Audio Device
- **Active Status**: Highlighted active device with ▶️ indicator
- **Volume Levels**: Current volume for each device
- **Restricted Status**: Shows if device doesn't allow remote control

## Device Selection and Casting

### TUI Device Selection

1. **Enter Device Mode**: Press `y` to show available Spotify devices
2. **Navigate**: Use `↑` and `↓` arrow keys to select a device
3. **Cast**: Press `Enter` to transfer playback to the selected device
4. **Cancel**: Press `Esc` to exit device selection without casting

### CLI Device Commands

```bash
# List all available Spotify Connect devices
nadctl spotify devices

# Cast to device by name (supports partial matching)
nadctl spotify transfer "Living Room"
nadctl spotify transfer "Chromecast"
nadctl spotify transfer "Kitchen Speaker"

# Cast to device by index number (from device list)
nadctl spotify transfer 1
nadctl spotify transfer 2

# Cast to device and automatically start playing
nadctl spotify transfer --play "Bedroom Speaker"
nadctl spotify transfer --play 1
```

### Device Types Supported

The integration works with any Spotify Connect device:
- **Chromecast**: Google Cast devices and smart TVs
- **Smart Speakers**: Sonos, Amazon Echo, Google Home
- **Computers**: Desktop and laptop Spotify applications
- **Mobile Devices**: Phones and tablets with Spotify
- **Audio Systems**: Spotify Connect-enabled receivers and speakers
- **Gaming Consoles**: PlayStation, Xbox with Spotify app

### Device Selection Examples

**Visual Device List in TUI:**
```
Spotify Devices:
▶️ 💻 MacBook Pro (Active) - Volume: 75%
   📺 Living Room Chromecast - Volume: 80%
   🔊 Kitchen Sonos - Volume: 60%
   📱 iPhone - Volume: 50% (Restricted)
   🎵 Spotify Connect Speaker - Volume: 90%
```

**CLI Device List:**
```
$ nadctl spotify devices
Available Spotify Devices:
1. 💻 MacBook Pro (Active) - Volume: 75%
2. 📺 Living Room Chromecast - Volume: 80%
3. 🔊 Kitchen Sonos - Volume: 60%
4. 📱 iPhone - Volume: 50% (Restricted)
5. 🎵 Spotify Connect Speaker - Volume: 90%
```

## Integration with NAD Controls

The Spotify integration works alongside your NAD device controls:
- **NAD Controls**: Volume, power, source, brightness affect your physical NAD device
- **Spotify Controls**: Playback, track navigation, Spotify volume affect the Spotify app
- **Device Casting**: Transfer Spotify playback between any Spotify Connect devices
- **Independent Operation**: NAD device, Spotify playback, and device casting work independently
- **Visual Feedback**: All panels update in real-time

## Example Workflows

### Home Theater Setup
1. **Power on NAD**: Press `p` to turn on your NAD amplifier
2. **Select Spotify source**: Use `→`/`←` to choose Stream or Wireless input
3. **Authenticate Spotify**: Follow authentication prompts if needed
4. **Cast to Chromecast**: Press `y`, select TV/Chromecast, press Enter
5. **Control playback**: Use `space`, `n`, `b` for music control
6. **Adjust NAD volume**: Use `+`/`-` for speaker volume

### Multi-Room Audio
1. **List devices**: Use `nadctl spotify devices` to see all rooms
2. **Cast to kitchen**: `nadctl spotify transfer "Kitchen Speaker"`
3. **Move to living room**: `nadctl spotify transfer "Living Room Sonos"`
4. **Continue on phone**: `nadctl spotify transfer "iPhone" --play`

### Quick Device Switching
1. **TUI mode**: Press `y` to show devices
2. **Quick select**: Use ↑↓ to highlight, Enter to cast
3. **Seamless handoff**: Music continues on new device
4. **Visual confirmation**: Active device indicator updates

## Troubleshooting

### Device Selection Issues
- **No devices shown**: Ensure you have active Spotify sessions on other devices
- **Can't cast to device**: Some devices (like phones) may restrict remote control
- **Device not appearing**: Make sure the device has Spotify open and is connected to the same network
- **Casting fails**: Check that the target device supports Spotify Connect

### Authentication Issues
- Ensure your redirect URI in Spotify app settings exactly matches `http://localhost:8080/callback`
- Make sure you're using the correct Client ID
- Check that your Spotify account has Premium status
- Verify you selected "Desktop App" type when creating the Spotify app

### Connection Issues
- Verify you have an active Spotify session (playing music in any Spotify app)
- The Spotify Web API requires an active device to control
- Try refreshing by restarting the TUI and re-authenticating
- For device casting, ensure target devices are on the same network

### Panel Not Showing
- Press `t` to toggle Spotify panel visibility
- Press `y` to show device selection panel
- Ensure you're authenticated (look for connection status)
- Check that Spotify is configured in your config file

### Device Casting Errors
- **"Device is restricted"**: Some devices don't allow remote control
- **"Transfer failed"**: Target device may be offline or busy
- **"No active session"**: Start playing music on any device first
- **"Device not found"**: Device name may have changed or device is offline

## Security & Privacy

- Your Spotify Client ID is stored locally in the config file
- Authentication tokens are managed securely using PKCE flow
- No client secret means no shared secrets that could be compromised
- Device information is retrieved directly from Spotify's official API
- No data is transmitted to external services except Spotify's official API
- Keep your config file permissions secure: `chmod 600 ~/.nadctl.yaml`

## API Rate Limits

The integration automatically manages API rate limits by:
- Queuing commands to prevent overwhelming the API
- Refreshing device state periodically rather than constantly
- Using efficient batch operations for device discovery
- Caching device information to reduce API calls

Enjoy your enhanced audio control experience with secure Spotify integration and seamless device casting! 🎵📱