package spotify

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"sync"
	"time"

	"runtime"

	"github.com/galamiram/nadctl/nadapi"
	log "github.com/sirupsen/logrus"
	"github.com/zmb3/spotify/v2"
	spotifyapi "github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

// Client handles Spotify API interactions
type Client struct {
	client        *spotify.Client
	auth          *spotifyauth.Authenticator
	token         *oauth2.Token
	connected     bool
	clientID      string
	redirectURL   string
	codeVerifier  string
	codeChallenge string
	server        *http.Server
	authResult    chan authResult
	authMutex     sync.Mutex
}

// authResult holds the result of the OAuth callback
type authResult struct {
	code  string
	state string
	err   error
}

// Track represents current track information
type Track struct {
	Name      string
	Artist    string
	Album     string
	Duration  time.Duration
	Progress  time.Duration
	IsPlaying bool
	ImageURL  string
}

// Device represents a Spotify Connect device
type Device struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Type          string `json:"type"` // Computer, Smartphone, Speaker, etc.
	IsActive      bool   `json:"is_active"`
	IsRestricted  bool   `json:"is_restricted"`
	VolumePercent int    `json:"volume_percent"`
}

// PlaybackState represents the current playback state
type PlaybackState struct {
	Track            Track
	Device           string
	DeviceID         string   // Current device ID
	AvailableDevices []Device // List of available devices
	Volume           int
	IsPlaying        bool
	Shuffle          bool
	Repeat           string // off, track, context
	Progress         int    // Current position in ms
	Duration         int    // Duration in ms
}

// NewClient creates a new Spotify client using PKCE flow (no client secret needed)
func NewClient(clientID, redirectURL string) *Client {
	if redirectURL == "" {
		redirectURL = "http://localhost:8888/callback"
	}

	// Generate PKCE code verifier and challenge
	codeVerifier := generateCodeVerifier()
	codeChallenge := generateCodeChallenge(codeVerifier)

	auth := spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURL),
		spotifyauth.WithScopes(
			spotifyauth.ScopeUserReadCurrentlyPlaying,
			spotifyauth.ScopeUserReadPlaybackState,
			spotifyauth.ScopeUserModifyPlaybackState,
		),
		spotifyauth.WithClientID(clientID),
		// No client secret for PKCE flow
	)

	client := &Client{
		auth:          auth,
		connected:     false,
		clientID:      clientID,
		redirectURL:   redirectURL,
		codeVerifier:  codeVerifier,
		codeChallenge: codeChallenge,
	}

	// Try to load cached token
	if err := client.loadToken(); err != nil {
		log.WithError(err).Debug("Failed to load cached token")
	} else if client.connected {
		log.Info("Successfully loaded cached Spotify token")
	}

	return client
}

// GetAuthURL returns the URL for user authentication using PKCE
func (c *Client) GetAuthURL() string {
	// Build auth URL with PKCE parameters in specific order
	baseURL := "https://accounts.spotify.com/authorize"
	params := url.Values{}

	// Required parameters in order
	params.Set("response_type", "code")
	params.Set("client_id", c.clientID)
	params.Set("redirect_uri", c.redirectURL)
	params.Set("state", "nadctl-state")

	// PKCE parameters
	params.Set("code_challenge", c.codeChallenge)
	params.Set("code_challenge_method", "S256")

	// Scopes
	params.Set("scope", "user-read-currently-playing user-read-playback-state user-modify-playback-state")

	// Optional parameters
	params.Set("show_dialog", "true") // Force auth dialog to show

	authURL := baseURL + "?" + params.Encode()

	log.WithFields(log.Fields{
		"baseURL":        baseURL,
		"client_id":      c.clientID,
		"redirect_uri":   c.redirectURL,
		"state":          "nadctl-state",
		"code_challenge": c.codeChallenge[:20] + "...", // Show first 20 chars only
		"scope":          "user-read-currently-playing user-read-playback-state user-modify-playback-state",
		"full_url":       authURL,
	}).Debug("Generated Spotify authorization URL")

	return authURL
}

// CompleteAuth completes the OAuth flow with the authorization code using PKCE
func (c *Client) CompleteAuth(code string) error {
	log.WithField("codeLength", len(code)).Info("Starting CompleteAuth with authorization code")

	// Exchange code for token using PKCE
	tokenURL := "https://accounts.spotify.com/api/token"
	log.WithField("tokenURL", tokenURL).Debug("Preparing token exchange request")

	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("redirect_uri", c.redirectURL)
	data.Set("client_id", c.clientID)
	data.Set("code_verifier", c.codeVerifier)

	log.WithFields(log.Fields{
		"grant_type":   "authorization_code",
		"redirect_uri": c.redirectURL,
		"client_id":    c.clientID,
		"has_code":     len(code) > 0,
		"has_verifier": len(c.codeVerifier) > 0,
	}).Debug("Token exchange request parameters")

	log.Debug("Sending token exchange request to Spotify")
	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		log.WithError(err).Error("HTTP request failed during token exchange")
		return fmt.Errorf("failed to exchange token: %w", err)
	}
	defer resp.Body.Close()

	log.WithField("statusCode", resp.StatusCode).Debug("Received token exchange response")

	if resp.StatusCode != http.StatusOK {
		log.WithField("statusCode", resp.StatusCode).Error("Token exchange failed with non-200 status")
		return fmt.Errorf("token exchange failed with status: %d", resp.StatusCode)
	}

	// Parse token response manually
	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	log.Debug("Parsing token response JSON")
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		log.WithError(err).Error("Failed to parse token response JSON")
		return fmt.Errorf("failed to parse token response: %w", err)
	}

	log.WithFields(log.Fields{
		"hasAccessToken":  len(tokenResp.AccessToken) > 0,
		"hasRefreshToken": len(tokenResp.RefreshToken) > 0,
		"tokenType":       tokenResp.TokenType,
		"expiresIn":       tokenResp.ExpiresIn,
		"scope":           tokenResp.Scope,
	}).Info("Successfully parsed token response")

	// Create OAuth2 token
	token := &oauth2.Token{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: tokenResp.RefreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	log.WithField("expiry", token.Expiry).Debug("Created OAuth2 token")

	c.token = token
	httpClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(token))
	c.client = spotify.New(httpClient)
	c.connected = true

	log.Info("Spotify client initialized and marked as connected")

	// Save token to cache
	log.Debug("Saving token to cache")
	if err := c.saveToken(); err != nil {
		log.WithError(err).Warn("Failed to save token to cache")
	} else {
		log.Debug("Token saved to cache successfully")
	}

	log.Info("CompleteAuth finished successfully")
	return nil
}

// generateCodeVerifier creates a cryptographically random code verifier for PKCE
func generateCodeVerifier() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(bytes)
}

// generateCodeChallenge creates a code challenge from the verifier using SHA256
func generateCodeChallenge(verifier string) string {
	hash := sha256.Sum256([]byte(verifier))
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(hash[:])
}

// IsConnected returns whether the client is authenticated
func (c *Client) IsConnected() bool {
	return c.connected && c.client != nil
}

// Disconnect clears the current session and token cache
func (c *Client) Disconnect() error {
	c.connected = false
	c.client = nil
	c.token = nil

	// Clear cached token
	if err := nadapi.ClearSpotifyToken(); err != nil {
		log.WithError(err).Debug("Failed to clear token cache")
		return err
	}

	log.Info("Disconnected from Spotify and cleared token cache")
	return nil
}

// IsTokenValid checks if the current token is valid and not expired
func (c *Client) IsTokenValid() bool {
	if c.token == nil {
		return false
	}

	// Check if token is expired (with 1 minute buffer)
	return time.Now().Add(1 * time.Minute).Before(c.token.Expiry)
}

// RefreshTokenIfNeeded refreshes the token if it's about to expire
func (c *Client) RefreshTokenIfNeeded() error {
	if !c.IsConnected() || c.IsTokenValid() {
		return nil // Not connected or token is still valid
	}

	if c.token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	log.Debug("Token is about to expire, refreshing")

	cache := nadapi.SpotifyTokenCache{
		AccessToken:  c.token.AccessToken,
		TokenType:    c.token.TokenType,
		RefreshToken: c.token.RefreshToken,
		Expiry:       c.token.Expiry,
		ClientID:     c.clientID,
	}

	return c.refreshTokenFromCache(cache)
}

// GetCurrentTrack gets the currently playing track
func (c *Client) GetCurrentTrack() (*Track, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to Spotify")
	}

	currently, err := c.client.PlayerCurrentlyPlaying(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get current track: %w", err)
	}

	if currently == nil || currently.Item == nil {
		return nil, fmt.Errorf("no track currently playing")
	}

	track := &Track{
		Name:      currently.Item.Name,
		IsPlaying: currently.Playing,
		Progress:  time.Duration(currently.Progress) * time.Millisecond,
		Duration:  time.Duration(currently.Item.Duration) * time.Millisecond,
	}

	// Get artist names
	if len(currently.Item.Artists) > 0 {
		track.Artist = currently.Item.Artists[0].Name
		if len(currently.Item.Artists) > 1 {
			for _, artist := range currently.Item.Artists[1:] {
				track.Artist += ", " + artist.Name
			}
		}
	}

	// Get album name
	track.Album = currently.Item.Album.Name

	// Get album art URL
	if len(currently.Item.Album.Images) > 0 {
		track.ImageURL = currently.Item.Album.Images[0].URL
	}

	return track, nil
}

// GetAvailableDevices retrieves all available Spotify Connect devices
func (c *Client) GetAvailableDevices() ([]Device, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected to Spotify")
	}

	devices, err := c.client.PlayerDevices(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get available devices: %v", err)
	}

	var result []Device
	for _, device := range devices {
		result = append(result, Device{
			ID:            device.ID.String(),
			Name:          device.Name,
			Type:          device.Type,
			IsActive:      device.Active,
			IsRestricted:  device.Restricted,
			VolumePercent: int(device.Volume), // Volume is of type Numeric, convert to int
		})
	}

	return result, nil
}

// TransferPlaybackToDevice transfers Spotify playback to a specific device
func (c *Client) TransferPlaybackToDevice(deviceID string, play bool) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to Spotify")
	}

	spotifyDeviceID := spotifyapi.ID(deviceID)
	err := c.client.TransferPlayback(context.Background(), spotifyDeviceID, play)
	if err != nil {
		return fmt.Errorf("failed to transfer playback to device %s: %v", deviceID, err)
	}

	log.WithFields(log.Fields{
		"device_id": deviceID,
		"play":      play,
	}).Info("Successfully transferred playback to device")

	return nil
}

// GetPlaybackState returns current playback state including device information
func (c *Client) GetPlaybackState() (PlaybackState, error) {
	if !c.IsConnected() {
		log.Debug("Not connected to Spotify")
		return PlaybackState{}, fmt.Errorf("not connected to Spotify")
	}

	log.Debug("Getting current playback state from Spotify")

	state, err := c.client.PlayerState(context.Background())
	if err != nil {
		log.WithError(err).Debug("Failed to get player state")
		return PlaybackState{}, fmt.Errorf("failed to get player state: %v", err)
	}

	if state == nil {
		log.Debug("No active playback session")
		return PlaybackState{}, fmt.Errorf("no active playback")
	}

	log.WithFields(log.Fields{
		"track":     state.Item.Name,
		"artist":    state.Item.Artists[0].Name,
		"isPlaying": state.Playing,
		"device":    state.Device.Name,
	}).Debug("Retrieved playback state")

	// Get volume if available
	volume := int(state.Device.Volume) // Volume is of type Numeric, Device is not a pointer

	return PlaybackState{
		Track: Track{
			Name:   state.Item.Name,
			Artist: state.Item.Artists[0].Name,
			Album:  state.Item.Album.Name,
		},
		Device:    state.Device.Name,
		DeviceID:  string(state.Device.ID),
		Volume:    volume,
		IsPlaying: state.Playing,
		Shuffle:   state.ShuffleState,
		Repeat:    string(state.RepeatState),
		Progress:  int(state.Progress),
		Duration:  int(state.Item.Duration),
	}, nil
}

// Play starts or resumes playback
func (c *Client) Play() error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to Spotify")
	}

	return c.client.Play(context.Background())
}

// Pause pauses playback
func (c *Client) Pause() error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to Spotify")
	}

	return c.client.Pause(context.Background())
}

// Next skips to next track
func (c *Client) Next() error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to Spotify")
	}

	return c.client.Next(context.Background())
}

// Previous skips to previous track
func (c *Client) Previous() error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to Spotify")
	}

	return c.client.Previous(context.Background())
}

// SetVolume sets the Spotify app volume (0-100)
func (c *Client) SetVolume(volume int) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to Spotify")
	}

	if volume < 0 {
		volume = 0
	}
	if volume > 100 {
		volume = 100
	}

	return c.client.Volume(context.Background(), volume)
}

// ToggleShuffle toggles shuffle mode
func (c *Client) ToggleShuffle() error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to Spotify")
	}

	state, err := c.GetPlaybackState()
	if err != nil {
		return err
	}

	return c.client.Shuffle(context.Background(), !state.Shuffle)
}

// CycleRepeat cycles through repeat modes (off -> context -> track -> off)
func (c *Client) CycleRepeat() error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected to Spotify")
	}

	state, err := c.GetPlaybackState()
	if err != nil {
		return err
	}

	var newRepeat string
	switch state.Repeat {
	case "off":
		newRepeat = "context"
	case "context":
		newRepeat = "track"
	case "track":
		newRepeat = "off"
	default:
		newRepeat = "off"
	}

	// For now, return nil - we'll implement proper repeat functionality later
	// when we can properly test with the Spotify API
	_ = newRepeat
	return fmt.Errorf("repeat functionality not yet implemented - needs proper Spotify API testing")
}

// StartCallbackServer starts an HTTP server to handle OAuth callbacks
func (c *Client) StartCallbackServer() error {
	log.Debug("StartCallbackServer called")
	c.authMutex.Lock()
	defer c.authMutex.Unlock()

	// Clean up any existing state with more thorough cleanup
	if c.server != nil {
		log.Debug("Cleaning up existing server before starting new one")
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		err := c.server.Shutdown(ctx)
		cancel()
		if err != nil {
			log.WithError(err).Debug("Error during server shutdown")
		} else {
			log.Debug("Successfully shut down existing server")
		}
		c.server = nil
	}

	// Close existing channel and wait for any pending operations to complete
	if c.authResult != nil {
		log.Debug("Cleaning up existing auth result channel")
		// Check if channel is not already closed before closing
		select {
		case old := <-c.authResult:
			// Drain any existing data and log what we found
			log.WithFields(log.Fields{
				"code":  len(old.code),
				"state": old.state,
				"err":   old.err,
			}).Debug("Drained stale result from auth channel")
		default:
			// Channel is empty, safe to close
			log.Debug("Auth channel was empty")
		}
		close(c.authResult)
		c.authResult = nil

		// Give a moment for any goroutines to finish
		time.Sleep(100 * time.Millisecond)
		log.Debug("Finished cleaning up auth result channel")
	}

	// Parse redirect URL to get port with fallback ports
	u, err := url.Parse(c.redirectURL)
	if err != nil {
		log.WithError(err).Error("Invalid redirect URL")
		return fmt.Errorf("invalid redirect URL: %w", err)
	}

	log.WithField("redirectURL", c.redirectURL).Debug("Parsed redirect URL")

	// Only use port 8888 - no fallbacks
	var ports []string

	if u.Port() != "" {
		// If a specific port was set in redirect URL, use only that port
		ports = []string{u.Port()}
		log.WithField("port", u.Port()).Debug("Using port from redirect URL")
	} else {
		// Use only our default port 8888
		ports = []string{"8888"}
		log.Debug("Using default port 8888")
	}

	var server *http.Server
	var actualAddr string

	for _, port := range ports {
		addr := u.Hostname() + ":" + port
		if u.Hostname() == "" {
			addr = "localhost:" + port
		}

		log.WithField("address", addr).Info("Attempting to start callback server")

		// Create HTTP handler
		mux := http.NewServeMux()
		mux.HandleFunc("/callback", c.handleCallback)
		log.Debug("Registered callback handler")

		// Create server
		server = &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
		}
		log.WithField("server", fmt.Sprintf("%+v", server)).Debug("Created HTTP server")

		// Try to start server
		log.WithField("address", addr).Debug("Attempting to listen on address")
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.WithError(err).WithField("address", addr).Warn("Port in use, trying next")
			continue
		}

		log.WithField("address", addr).Info("Successfully bound to address")

		// Port is available, start server
		actualAddr = addr
		go func() {
			log.WithField("address", actualAddr).Info("Starting HTTP server goroutine")
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				log.WithError(err).Error("Callback server error")
				// Only send error if we have a valid channel
				c.authMutex.Lock()
				if c.authResult != nil {
					select {
					case c.authResult <- authResult{err: fmt.Errorf("callback server error: %w", err)}:
						log.Debug("Sent callback server error to auth result channel")
					default:
						log.Debug("Could not send callback server error - channel full")
					}
				}
				c.authMutex.Unlock()
			} else {
				log.Debug("HTTP server stopped normally")
			}
		}()
		break
	}

	if server == nil {
		log.Error("Failed to start callback server on any available port")
		return fmt.Errorf("failed to start callback server on any available port")
	}

	c.server = server

	// Update redirect URL if we used a different port
	if actualAddr != u.Host {
		c.redirectURL = "http://" + actualAddr + "/callback"
		log.WithField("new_url", c.redirectURL).Info("Updated redirect URL due to port change")
	}

	// Give server a moment to start
	log.Debug("Waiting for server to start")
	time.Sleep(300 * time.Millisecond)

	// NOW create the auth result channel after server is stable
	c.authResult = make(chan authResult, 1)
	log.Debug("Created new auth result channel after server startup")

	log.WithField("address", actualAddr).Info("Callback server started successfully")
	return nil
}

// StopCallbackServer stops the OAuth callback server
func (c *Client) StopCallbackServer() error {
	c.authMutex.Lock()
	defer c.authMutex.Unlock()

	if c.server == nil {
		log.Debug("No callback server to stop")
		return nil // Already stopped, no error
	}

	log.Debug("Stopping callback server")

	// Close the result channel first to stop any waiting operations
	if c.authResult != nil {
		log.Debug("Closing auth result channel")
		// Check if channel is not already closed before closing
		select {
		case <-c.authResult:
			// Channel already closed or drained
		default:
			// Don't send cancellation message - let the timeout handle it gracefully
			// This prevents "authentication cancelled" errors when the app is shutting down
			close(c.authResult)
		}
		c.authResult = nil
	}

	// Shutdown server with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Debug("Shutting down HTTP server")
	err := c.server.Shutdown(ctx)
	c.server = nil

	// Give a moment for the server to fully shut down
	time.Sleep(100 * time.Millisecond)

	if err != nil {
		log.WithError(err).Debug("Error during server shutdown")
	} else {
		log.Debug("Callback server stopped successfully")
	}

	return err
}

// handleCallback handles the OAuth callback from Spotify
func (c *Client) handleCallback(w http.ResponseWriter, r *http.Request) {
	log.WithField("url", r.URL.String()).Debug("Received callback")

	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	errorParam := r.URL.Query().Get("error")

	// Send HTML response to user
	var htmlResponse string
	var authResultToSend authResult

	if errorParam != "" {
		log.WithField("error", errorParam).Debug("Authentication error received")
		htmlResponse = `
<!DOCTYPE html>
<html>
<head>
    <title>Spotify Authentication</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background-color: #f5f5f5; }
        .container { background: white; padding: 30px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); max-width: 500px; margin: 0 auto; }
        .error { color: #e22134; }
        .icon { font-size: 48px; margin-bottom: 20px; color: #e22134; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">❌</div>
        <h1 class="error">Authentication Failed</h1>
        <p>There was an error during Spotify authentication: ` + errorParam + `</p>
        <p>Please try again in the NAD Controller application.</p>
    </div>
</body>
</html>`
		authResultToSend = authResult{err: fmt.Errorf("authentication error: %s", errorParam)}
	} else if code != "" && state != "" {
		log.Debug("Authentication successful, received code")
		// Success page with auto-close
		htmlResponse = `
<!DOCTYPE html>
<html>
<head>
    <title>Spotify Authentication</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background-color: #f5f5f5; }
        .container { background: white; padding: 30px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); max-width: 500px; margin: 0 auto; }
        .success { color: #1db954; }
        .icon { font-size: 48px; margin-bottom: 20px; color: #1db954; }
        .countdown { color: #666; font-size: 14px; margin-top: 10px; }
    </style>
    <script>
        let countdown = 3;
        function updateCountdown() {
            const countdownEl = document.getElementById('countdown');
            if (countdownEl) {
                countdownEl.textContent = 'This tab will close automatically in ' + countdown + ' seconds...';
            }
            countdown--;
            if (countdown < 0) {
                // Try multiple methods to close the tab
                try {
                    window.close();
                } catch (e) {
                    // If window.close() fails, try alternative
                    window.location.href = 'about:blank';
                    setTimeout(() => {
                        window.close();
                    }, 100);
                }
            } else {
                setTimeout(updateCountdown, 1000);
            }
        }
        window.onload = function() {
            setTimeout(updateCountdown, 1000);
        };
    </script>
</head>
<body>
    <div class="container">
        <div class="icon">&#10003;</div>
        <h1 class="success">Authentication Successful!</h1>
        <p>Your NAD Controller is now connected to Spotify.</p>
        <div class="countdown" id="countdown">This tab will close automatically in 3 seconds...</div>
    </div>
</body>
</html>`
		authResultToSend = authResult{code: code, state: state}
	} else {
		log.Debug("Invalid callback - missing code or state")
		htmlResponse = `
<!DOCTYPE html>
<html>
<head>
    <title>Spotify Authentication</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; padding: 50px; background-color: #f5f5f5; }
        .container { background: white; padding: 30px; border-radius: 10px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); max-width: 500px; margin: 0 auto; }
        .error { color: #e22134; }
        .icon { font-size: 48px; margin-bottom: 20px; color: #e22134; }
    </style>
</head>
<body>
    <div class="container">
        <div class="icon">&#63;</div>
        <h1 class="error">Invalid Request</h1>
        <p>No authorization code received from Spotify.</p>
        <p>Please try again in the NAD Controller application.</p>
    </div>
</body>
</html>`
		authResultToSend = authResult{err: fmt.Errorf("no authorization code received")}
	}

	// Send response to browser
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(htmlResponse))

	// Send result to waiting authentication process (with safety check)
	go func() {
		c.authMutex.Lock()
		defer c.authMutex.Unlock()

		if c.authResult != nil {
			select {
			case c.authResult <- authResultToSend:
				log.Debug("Auth result sent successfully")
			case <-time.After(1 * time.Second):
				log.Debug("Timeout sending auth result")
			}
		} else {
			log.Debug("Auth result channel is nil, skipping")
		}
	}()
}

// WaitForCallback waits for the OAuth callback with a timeout
func (c *Client) WaitForCallback(timeout time.Duration) (string, error) {
	log.WithField("timeout", timeout).Info("Starting to wait for OAuth callback")

	// Check if auth result channel exists
	if c.authResult == nil {
		log.Error("Auth result channel is nil")
		return "", fmt.Errorf("auth result channel not initialized")
	}

	log.Debug("Auth result channel is ready, waiting for callback")

	select {
	case result, ok := <-c.authResult:
		if !ok {
			// Channel was closed - this happens during graceful shutdown
			log.Info("Auth result channel was closed during shutdown")
			return "", fmt.Errorf("authentication was interrupted by application shutdown")
		}
		log.Debug("Received result from auth channel")
		if result.err != nil {
			log.WithError(result.err).Error("Auth callback returned error")
			return "", result.err
		}
		if result.state != "nadctl-state" {
			log.WithFields(log.Fields{
				"expected": "nadctl-state",
				"actual":   result.state,
			}).Error("Invalid state parameter in callback")
			return "", fmt.Errorf("invalid state parameter")
		}
		log.WithField("codeLength", len(result.code)).Info("Successfully received valid auth code")
		return result.code, nil
	case <-time.After(timeout):
		log.WithField("timeout", timeout).Warn("Authentication callback timed out")
		return "", fmt.Errorf("authentication timeout after %v", timeout)
	}
}

// AuthenticateWithCallback performs complete OAuth authentication with automatic callback handling
func (c *Client) AuthenticateWithCallback(timeout time.Duration) error {
	log.WithField("timeout", timeout).Info("Starting AuthenticateWithCallback")

	// Start callback server
	log.Debug("Starting callback server for authentication")
	if err := c.StartCallbackServer(); err != nil {
		log.WithError(err).Error("Failed to start callback server")
		return fmt.Errorf("failed to start callback server: %w", err)
	}
	log.Info("Callback server started successfully")

	// Ensure server is stopped when done
	defer func() {
		log.Debug("Stopping callback server after authentication")
		c.StopCallbackServer()
	}()

	// Get the auth URL and open browser
	authURL := c.GetAuthURL()
	log.WithField("authURL", authURL).Info("Opening browser for Spotify authentication")

	// Open browser automatically
	if err := c.openBrowser(authURL); err != nil {
		log.WithError(err).Warn("Failed to open browser automatically")
		log.WithField("authURL", authURL).Info("Please manually open the following URL in your browser")
		// Don't return error, continue waiting for callback as user might open manually
	} else {
		log.Info("Browser opened successfully")
	}

	// Wait for callback
	log.WithField("timeout", timeout).Info("Waiting for authentication callback")
	code, err := c.WaitForCallback(timeout)
	if err != nil {
		log.WithError(err).Error("Failed to receive authentication callback")
		return fmt.Errorf("failed to receive callback: %w", err)
	}
	log.WithField("codeLength", len(code)).Info("Received authentication code")

	// Complete authentication
	log.Debug("Completing authentication with received code")
	if err := c.CompleteAuth(code); err != nil {
		log.WithError(err).Error("Failed to complete authentication")
		return fmt.Errorf("failed to complete authentication: %w", err)
	}

	log.Info("Authentication completed successfully")
	return nil
}

// saveToken saves the current token to cache using nadapi cache system
func (c *Client) saveToken() error {
	if c.token == nil {
		return fmt.Errorf("no token to save")
	}

	cache := &nadapi.SpotifyTokenCache{
		AccessToken:  c.token.AccessToken,
		TokenType:    c.token.TokenType,
		RefreshToken: c.token.RefreshToken,
		Expiry:       c.token.Expiry,
		ClientID:     c.clientID,
	}

	if err := nadapi.SaveSpotifyToken(cache); err != nil {
		return fmt.Errorf("failed to save token to cache: %w", err)
	}

	log.Debug("Token saved to cache")
	return nil
}

// loadToken loads a token from cache if available and valid
func (c *Client) loadToken() error {
	cache, err := nadapi.LoadSpotifyToken()
	if err != nil {
		log.WithError(err).Debug("Failed to load token from cache")
		return err
	}

	if cache == nil {
		log.Debug("No token cache found")
		return nil // Not an error, just no cache
	}

	// Verify the token is for the same client ID
	if cache.ClientID != c.clientID {
		log.Debug("Token cache is for different client ID, ignoring")
		return nil
	}

	// Check if token is expired (with 5 minute buffer)
	if time.Now().Add(5 * time.Minute).After(cache.Expiry) {
		log.Debug("Cached token is expired or about to expire")

		// Try to refresh the token if we have a refresh token
		if cache.RefreshToken != "" {
			if err := c.refreshTokenFromCache(*cache); err != nil {
				log.WithError(err).Debug("Failed to refresh token from cache")
				nadapi.ClearSpotifyToken() // Remove invalid cache
				return nil
			}
			return nil // Successfully refreshed
		}

		log.Debug("No refresh token available, removing expired cache")
		nadapi.ClearSpotifyToken()
		return nil
	}

	// Token is valid, use it
	c.token = &oauth2.Token{
		AccessToken:  cache.AccessToken,
		TokenType:    cache.TokenType,
		RefreshToken: cache.RefreshToken,
		Expiry:       cache.Expiry,
	}

	// Initialize Spotify client with cached token
	httpClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(c.token))
	c.client = spotify.New(httpClient)
	c.connected = true

	log.Debug("Token loaded from cache and client initialized")
	return nil
}

// refreshTokenFromCache attempts to refresh a token using the refresh token
func (c *Client) refreshTokenFromCache(cache nadapi.SpotifyTokenCache) error {
	log.Debug("Attempting to refresh token from cache")

	tokenURL := "https://accounts.spotify.com/api/token"
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", cache.RefreshToken)
	data.Set("client_id", c.clientID)

	resp, err := http.PostForm(tokenURL, data)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token refresh failed with status: %d", resp.StatusCode)
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		TokenType    string `json:"token_type"`
		RefreshToken string `json:"refresh_token,omitempty"`
		ExpiresIn    int    `json:"expires_in"`
		Scope        string `json:"scope"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to parse refresh token response: %w", err)
	}

	// Use existing refresh token if new one wasn't provided
	refreshToken := tokenResp.RefreshToken
	if refreshToken == "" {
		refreshToken = cache.RefreshToken
	}

	// Create new OAuth2 token
	token := &oauth2.Token{
		AccessToken:  tokenResp.AccessToken,
		TokenType:    tokenResp.TokenType,
		RefreshToken: refreshToken,
		Expiry:       time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second),
	}

	c.token = token
	httpClient := oauth2.NewClient(context.Background(), oauth2.StaticTokenSource(token))
	c.client = spotify.New(httpClient)
	c.connected = true

	// Save the refreshed token
	if err := c.saveToken(); err != nil {
		log.WithError(err).Debug("Failed to save refreshed token")
	}

	log.Debug("Token refreshed successfully")
	return nil
}

// openBrowser opens the default browser with the given URL
func (c *Client) openBrowser(url string) error {
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

	return exec.Command(cmd, args...).Start()
}
