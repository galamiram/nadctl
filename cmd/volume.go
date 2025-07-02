/*
Copyright © 2020 Gal Amiram <galamiram1@gmail.com>

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cmd

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/galamiram/nadctl/nadapi"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// volumeCmd represents the volume command
var volumeCmd = &cobra.Command{
	Use:   "volume [LEVEL|up|down]",
	Short: "Set or get volume level",
	Long: `Set the volume to a specific level or adjust it relatively.

Volume levels are typically in the range of -80 to +10 dB.

Examples:
  nadctl volume              # Show current volume
  nadctl volume set -20      # Set volume to -20 dB (recommended for negative)
  nadctl volume -- -20       # Alternative syntax for negative volumes
  nadctl volume 0            # Set volume to 0 dB (reference level)
  nadctl volume up           # Increase volume by 1 dB
  nadctl volume down         # Decrease volume by 1 dB

Note: For negative volume levels, you can use:
  nadctl volume set -10      # Easiest way (recommended)
  nadctl volume -- -10       # Alternative using -- separator`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := connectToDevice()
		if err != nil {
			log.WithError(err).Fatal("could not connect to device")
		}
		defer client.Disconnect()

		log.WithField("device", client.IP.String()).Debug("Connected to device for volume command")

		// No arguments - show current volume
		if len(args) == 0 {
			log.Debug("No arguments provided, getting current volume")
			currentVolume, err := client.GetVolumeFloat()
			if err != nil {
				log.WithError(err).Fatal("failed to get current volume")
			}
			log.WithField("currentVolume", currentVolume).Debug("Retrieved current volume")
			fmt.Printf("Current volume: %.1f dB\n", currentVolume)
			return
		}

		arg := strings.ToLower(args[0])
		log.WithFields(log.Fields{
			"argument": arg,
			"original": args[0],
		}).Debug("Processing volume command argument")

		// Handle relative adjustments
		switch arg {
		case "up":
			log.Debug("Increasing volume")
			err = client.TuneVolume(nadapi.DirectionUp)
			if err != nil {
				log.WithError(err).Fatal("failed to increase volume")
			}
			newVolume, err := client.GetVolumeFloat()
			if err == nil {
				log.WithField("newVolume", newVolume).Debug("Successfully increased volume")
				fmt.Printf("Volume increased to: %.1f dB\n", newVolume)
			} else {
				log.WithError(err).Debug("Failed to get new volume after increase")
				fmt.Println("Volume increased")
			}
			return

		case "down":
			log.Debug("Decreasing volume")
			err = client.TuneVolume(nadapi.DirectionDown)
			if err != nil {
				log.WithError(err).Fatal("failed to decrease volume")
			}
			newVolume, err := client.GetVolumeFloat()
			if err == nil {
				log.WithField("newVolume", newVolume).Debug("Successfully decreased volume")
				fmt.Printf("Volume decreased to: %.1f dB\n", newVolume)
			} else {
				log.WithError(err).Debug("Failed to get new volume after decrease")
				fmt.Println("Volume decreased")
			}
			return

		default:
			// Try to parse as a volume level
			log.WithField("volumeString", args[0]).Debug("Attempting to parse volume level")
			volume, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				log.WithError(err).WithField("volumeString", args[0]).Debug("Failed to parse volume level")
				fmt.Printf("Error: '%s' is not a valid volume level.\n\n", args[0])
				fmt.Println("Usage:")
				fmt.Println("  nadctl volume              # Show current volume")
				fmt.Println("  nadctl volume set -20      # Set volume to -20 dB (recommended for negative)")
				fmt.Println("  nadctl volume -- -20       # Alternative syntax for negative volumes")
				fmt.Println("  nadctl volume 0            # Set volume to 0 dB")
				fmt.Println("  nadctl volume up           # Increase volume")
				fmt.Println("  nadctl volume down         # Decrease volume")
				fmt.Println("\nVolume range is typically -80 to +10 dB")
				fmt.Println("Note: Use -- before negative numbers to prevent flag parsing")
				return
			}

			log.WithField("parsedVolume", volume).Debug("Successfully parsed volume level")

			// Warn about potentially dangerous volume levels
			if volume > 5 {
				log.WithField("volume", volume).Debug("High volume level detected, requesting confirmation")
				fmt.Printf("Warning: Volume level %.1f dB is quite high. Continue? (y/N): ", volume)
				var response string
				fmt.Scanln(&response)
				log.WithField("userResponse", response).Debug("User response to volume warning")
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					log.Debug("User cancelled high volume operation")
					fmt.Println("Volume change cancelled")
					return
				}
				log.Debug("User confirmed high volume operation")
			}

			log.WithField("volume", volume).Debug("Setting volume to specific level")
			err = client.SetVolume(volume)
			if err != nil {
				log.WithError(err).Fatal("failed to set volume")
			}

			log.WithField("volume", volume).Debug("Successfully set volume")
			fmt.Printf("Volume set to: %.1f dB\n", volume)
		}
	},
}

func init() {
	rootCmd.AddCommand(volumeCmd)

	// Add convenient aliases for common volume levels
	volumeCmd.AddCommand(&cobra.Command{
		Use:   "set LEVEL",
		Short: "Set volume to a specific level",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			volume, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				fmt.Printf("Error: '%s' is not a valid volume level.\n", args[0])
				return
			}

			client, err := connectToDevice()
			if err != nil {
				log.WithError(err).Fatal("could not connect to device")
			}
			defer client.Disconnect()

			// Warn about potentially dangerous volume levels
			if volume > 5 {
				fmt.Printf("Warning: Volume level %.1f dB is quite high. Continue? (y/N): ", volume)
				var response string
				fmt.Scanln(&response)
				if strings.ToLower(response) != "y" && strings.ToLower(response) != "yes" {
					fmt.Println("Volume change cancelled")
					return
				}
			}

			err = client.SetVolume(volume)
			if err != nil {
				log.WithError(err).Fatal("failed to set volume")
			}

			fmt.Printf("Volume set to: %.1f dB\n", volume)
		},
	})
}
