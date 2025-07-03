package cmd

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/galamiram/nadctl/spotify"
)

var spotifyClient *spotify.Client

// spotifyCmd represents the spotify command
var spotifyCmd = &cobra.Command{
	Use:   "spotify",
	Short: "Control Spotify playback",
	Long: `Control Spotify playback including device selection, playback controls, 
and authentication. Supports casting to Chromecast and other Spotify Connect devices.`,
}

var spotifyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current Spotify playback status",
	Long:  `Display current Spotify playback status including track info, device, and available devices.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()
		if !client.IsConnected() {
			logrus.Fatal("Not connected to Spotify. Run 'nadctl spotify connect' first.")
		}

		state, err := client.GetPlaybackState()
		if err != nil {
			logrus.WithError(err).Fatal("Failed to get playback state")
		}

		// Current track and device info
		logrus.WithFields(logrus.Fields{
			"track":    state.Track.Name,
			"artist":   state.Track.Artist,
			"album":    state.Track.Album,
			"device":   state.Device,
			"deviceID": state.DeviceID,
			"playing":  state.IsPlaying,
			"volume":   state.Volume,
			"shuffle":  state.Shuffle,
			"repeat":   state.Repeat,
		}).Info("Current Spotify Status")

		// Show available devices
		if len(state.AvailableDevices) > 0 {
			logrus.Info("Available devices:")
			for i, device := range state.AvailableDevices {
				status := ""
				if device.IsActive {
					status = " (active)"
				}
				logrus.Infof("  %d. %s (%s)%s", i+1, device.Name, device.Type, status)
			}
		}
	},
}

var spotifyConnectCmd = &cobra.Command{
	Use:   "connect",
	Short: "Connect to Spotify",
	Long:  `Authenticate and connect to Spotify using OAuth 2.0.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()

		if client.IsConnected() {
			logrus.Info("Already connected to Spotify")
			return
		}

		// Start authentication
		logrus.Info("Starting Spotify authentication...")

		// Use callback server for authentication
		if err := client.AuthenticateWithCallback(60 * time.Second); err != nil {
			logrus.WithError(err).Fatal("Authentication failed")
		}

		logrus.Info("Successfully connected to Spotify!")
	},
}

var spotifyDisconnectCmd = &cobra.Command{
	Use:   "disconnect",
	Short: "Disconnect from Spotify",
	Long:  `Disconnect from Spotify and clear stored credentials.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()

		if !client.IsConnected() {
			logrus.Info("Not connected to Spotify")
			return
		}

		if err := client.Disconnect(); err != nil {
			logrus.WithError(err).Fatal("Failed to disconnect")
		}

		logrus.Info("Disconnected from Spotify")
	},
}

var spotifyDevicesCmd = &cobra.Command{
	Use:   "devices",
	Short: "List available Spotify Connect devices",
	Long: `List all available Spotify Connect devices including Chromecast, speakers, 
computers, and other Cast-enabled devices.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()
		if !client.IsConnected() {
			logrus.Fatal("Not connected to Spotify. Run 'nadctl spotify connect' first.")
		}

		devices, err := client.GetAvailableDevices()
		if err != nil {
			logrus.WithError(err).Fatal("Failed to get available devices")
		}

		if len(devices) == 0 {
			logrus.Info("No devices found. Make sure you have Spotify open on at least one device.")
			return
		}

		logrus.Info("Available Spotify Connect devices:")
		for i, device := range devices {
			status := ""
			if device.IsActive {
				status = " (active)"
			}
			if device.IsRestricted {
				status += " (restricted)"
			}

			volumeInfo := ""
			// All Spotify devices support volume control
			volumeInfo = fmt.Sprintf(" - Volume: %d%%", device.VolumePercent)

			logrus.WithFields(logrus.Fields{
				"index": i + 1,
				"id":    device.ID,
				"name":  device.Name,
				"type":  device.Type,
			}).Infof("%d. %s (%s)%s%s", i+1, device.Name, device.Type, status, volumeInfo)
		}
	},
}

var spotifyTransferCmd = &cobra.Command{
	Use:   "transfer [device-name-or-index]",
	Short: "Transfer playback to a specific device",
	Long: `Transfer Spotify playback to a specific device. You can specify either:
- The device name (e.g., "Living Room Chromecast")
- The device index from the devices list (e.g., "1")

Use 'nadctl spotify devices' to see available devices.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()
		if !client.IsConnected() {
			logrus.Fatal("Not connected to Spotify. Run 'nadctl spotify connect' first.")
		}

		deviceSelector := args[0]

		// Get available devices
		devices, err := client.GetAvailableDevices()
		if err != nil {
			logrus.WithError(err).Fatal("Failed to get available devices")
		}

		if len(devices) == 0 {
			logrus.Fatal("No devices found. Make sure you have Spotify open on at least one device.")
		}

		var selectedDevice *spotify.Device

		// Try to parse as index first
		if index, err := strconv.Atoi(deviceSelector); err == nil {
			if index < 1 || index > len(devices) {
				logrus.Fatalf("Invalid device index %d. Use 'nadctl spotify devices' to see available devices.", index)
			}
			selectedDevice = &devices[index-1]
		} else {
			// Search by name (case-insensitive)
			deviceSelector = strings.ToLower(deviceSelector)
			for _, device := range devices {
				if strings.Contains(strings.ToLower(device.Name), deviceSelector) {
					selectedDevice = &device
					break
				}
			}
		}

		if selectedDevice == nil {
			logrus.Fatalf("Device '%s' not found. Use 'nadctl spotify devices' to see available devices.", args[0])
		}

		// Check if already active
		if selectedDevice.IsActive {
			logrus.Infof("Device '%s' is already the active playback device.", selectedDevice.Name)
			return
		}

		// Transfer playback (continue playing if music was already playing)
		play, _ := cmd.Flags().GetBool("play")
		err = client.TransferPlaybackToDevice(selectedDevice.ID, play)
		if err != nil {
			logrus.WithError(err).Fatal("Failed to transfer playback")
		}

		logrus.WithFields(logrus.Fields{
			"device": selectedDevice.Name,
			"type":   selectedDevice.Type,
			"id":     selectedDevice.ID,
		}).Info("Successfully transferred playback to device")
	},
}

var spotifyPlayCmd = &cobra.Command{
	Use:   "play",
	Short: "Start Spotify playback",
	Long:  `Start Spotify playback on the current device.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()
		if !client.IsConnected() {
			logrus.Fatal("Not connected to Spotify. Run 'nadctl spotify connect' first.")
		}

		if err := client.Play(); err != nil {
			logrus.WithError(err).Fatal("Failed to start playback")
		}

		logrus.Info("Started Spotify playback")
	},
}

var spotifyPauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause Spotify playback",
	Long:  `Pause Spotify playback on the current device.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()
		if !client.IsConnected() {
			logrus.Fatal("Not connected to Spotify. Run 'nadctl spotify connect' first.")
		}

		if err := client.Pause(); err != nil {
			logrus.WithError(err).Fatal("Failed to pause playback")
		}

		logrus.Info("Paused Spotify playback")
	},
}

var spotifyNextCmd = &cobra.Command{
	Use:   "next",
	Short: "Skip to next track",
	Long:  `Skip to the next track in the Spotify queue.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()
		if !client.IsConnected() {
			logrus.Fatal("Not connected to Spotify. Run 'nadctl spotify connect' first.")
		}

		if err := client.Next(); err != nil {
			logrus.WithError(err).Fatal("Failed to skip to next track")
		}

		logrus.Info("Skipped to next track")
	},
}

var spotifyPrevCmd = &cobra.Command{
	Use:   "prev",
	Short: "Skip to previous track",
	Long:  `Skip to the previous track in the Spotify queue.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()
		if !client.IsConnected() {
			logrus.Fatal("Not connected to Spotify. Run 'nadctl spotify connect' first.")
		}

		if err := client.Previous(); err != nil {
			logrus.WithError(err).Fatal("Failed to skip to previous track")
		}

		logrus.Info("Skipped to previous track")
	},
}

var spotifyVolumeCmd = &cobra.Command{
	Use:   "volume [level]",
	Short: "Set Spotify volume (0-100)",
	Long:  `Set the Spotify volume level from 0 to 100 percent.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()
		if !client.IsConnected() {
			logrus.Fatal("Not connected to Spotify. Run 'nadctl spotify connect' first.")
		}

		volume, err := strconv.Atoi(args[0])
		if err != nil || volume < 0 || volume > 100 {
			logrus.Fatal("Volume must be a number between 0 and 100")
		}

		if err := client.SetVolume(volume); err != nil {
			logrus.WithError(err).Fatal("Failed to set volume")
		}

		logrus.WithField("volume", volume).Info("Set Spotify volume")
	},
}

var spotifyShuffleCmd = &cobra.Command{
	Use:   "shuffle",
	Short: "Toggle shuffle mode",
	Long:  `Toggle Spotify shuffle mode on or off.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()
		if !client.IsConnected() {
			logrus.Fatal("Not connected to Spotify. Run 'nadctl spotify connect' first.")
		}

		if err := client.ToggleShuffle(); err != nil {
			logrus.WithError(err).Fatal("Failed to toggle shuffle")
		}

		logrus.Info("Toggled Spotify shuffle mode")
	},
}

var spotifyRepeatCmd = &cobra.Command{
	Use:   "repeat",
	Short: "Cycle repeat mode",
	Long:  `Cycle through Spotify repeat modes: off -> track -> context -> off.`,
	Run: func(cmd *cobra.Command, args []string) {
		client := getSpotifyClient()
		if !client.IsConnected() {
			logrus.Fatal("Not connected to Spotify. Run 'nadctl spotify connect' first.")
		}

		if err := client.CycleRepeat(); err != nil {
			logrus.WithError(err).Fatal("Failed to cycle repeat mode")
		}

		logrus.Info("Cycled Spotify repeat mode")
	},
}

// getSpotifyClient returns the Spotify client instance
func getSpotifyClient() *spotify.Client {
	if spotifyClient == nil {
		clientID := viper.GetString("spotify.client_id")
		if clientID == "" {
			logrus.Fatal("Spotify client ID not configured. Set it with: nadctl config set spotify.client_id YOUR_CLIENT_ID")
		}

		redirectURL := viper.GetString("spotify.redirect_url")
		if redirectURL == "" {
			redirectURL = "http://localhost:8888/callback"
		}

		spotifyClient = spotify.NewClient(clientID, redirectURL)
	}
	return spotifyClient
}

func init() {
	// Add all subcommands to spotifyCmd
	spotifyCmd.AddCommand(spotifyStatusCmd)
	spotifyCmd.AddCommand(spotifyConnectCmd)
	spotifyCmd.AddCommand(spotifyDisconnectCmd)
	spotifyCmd.AddCommand(spotifyDevicesCmd)
	spotifyCmd.AddCommand(spotifyTransferCmd)
	spotifyCmd.AddCommand(spotifyPlayCmd)
	spotifyCmd.AddCommand(spotifyPauseCmd)
	spotifyCmd.AddCommand(spotifyNextCmd)
	spotifyCmd.AddCommand(spotifyPrevCmd)
	spotifyCmd.AddCommand(spotifyVolumeCmd)
	spotifyCmd.AddCommand(spotifyShuffleCmd)
	spotifyCmd.AddCommand(spotifyRepeatCmd)

	// Add flags
	spotifyTransferCmd.Flags().BoolP("play", "p", false, "Start playing immediately after transfer")

	// Add spotify command to root
	rootCmd.AddCommand(spotifyCmd)
}
