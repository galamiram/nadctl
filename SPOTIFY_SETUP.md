# Spotify Integration Setup Guide

This guide will help you set up Spotify integration with your NAD Audio Controller TUI using the secure PKCE (Proof Key for Code Exchange) flow.

## Prerequisites

1. **Spotify Premium Account** - Required for controlling playback
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

### Spotify Panel Features

When visible (press `t` to toggle), the Spotify panel shows:
- **Current Track**: Song name, artist, album
- **Playback State**: Playing/paused status with icons
- **Progress**: Track progress bar and time indicators
- **Controls**: Available actions status
- **Device**: Currently active Spotify device
- **Volume**: Spotify app volume level
- **Modes**: Shuffle and repeat status with visual indicators

## Integration with NAD Controls

The Spotify integration works alongside your NAD device controls:
- **NAD Controls**: Volume, power, source, brightness affect your physical NAD device
- **Spotify Controls**: Playback, track navigation, Spotify volume affect the Spotify app
- **Independent Operation**: Both systems work independently and simultaneously
- **Visual Feedback**: Both panels update in real-time

## Example Workflow

1. **Start your day**: Power on NAD device with `p`
2. **Select source**: Choose Spotify input on NAD with `→`/`←`
3. **Authenticate**: Follow Spotify authentication prompts
4. **Control playback**: Use `space`, `n`, `b` for music control
5. **Adjust audio**: Use NAD volume (`+`/`-`) for speakers
6. **Toggle view**: Press `t` to show/hide Spotify information

## Troubleshooting

### Authentication Issues
- Ensure your redirect URI in Spotify app settings exactly matches `http://localhost:8080/callback`
- Make sure you're using the correct Client ID
- Check that your Spotify account has Premium status
- Verify you selected "Desktop App" type when creating the Spotify app

### Connection Issues
- Verify you have an active Spotify session (playing music in any Spotify app)
- The Spotify Web API requires an active device to control
- Try refreshing by restarting the TUI and re-authenticating

### Panel Not Showing
- Press `t` to toggle Spotify panel visibility
- Ensure you're authenticated (look for connection status)
- Check that Spotify is configured in your config file

### PKCE Flow Issues
- Make sure your Spotify app is configured as "Desktop App" type
- Verify the redirect URI matches exactly (including http:// and port)
- Check that you're copying the full authorization code from the redirect URL

## Security & Privacy

- Your Spotify Client ID is stored locally in the config file
- Authentication tokens are managed securely using PKCE flow
- No client secret means no shared secrets that could be compromised
- No data is transmitted to external services except Spotify's official API
- Keep your config file permissions secure: `chmod 600 ~/.nadctl.yaml`

## API Rate Limits

The integration automatically manages API rate limits by:
- Queuing commands to prevent overwhelming the API
- Refreshing state periodically (every 5 seconds) rather than constantly
- Using efficient batch operations where possible

Enjoy your enhanced audio control experience with secure Spotify integration! 🎵