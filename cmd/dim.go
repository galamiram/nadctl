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

	"github.com/galamiram/nadctl/internal/nadapi"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// dimCmd represents the dim command
var dimCmd = &cobra.Command{
	Use:   "dim [LEVEL|up|down|list]",
	Short: "Set or get display brightness",
	Long: `Set the display brightness to a specific level or adjust it relatively.

Brightness levels are discrete values, typically 0-3:
  0 = Display off/darkest
  1 = Low brightness  
  2 = Medium brightness
  3 = High brightness

Examples:
  nadctl dim              # Show current brightness
  nadctl dim 0            # Set brightness to 0 (display off)
  nadctl dim 2            # Set brightness to 2 (medium)
  nadctl dim up           # Increase brightness
  nadctl dim down         # Decrease brightness
  nadctl dim list         # List all available levels`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := connectToDevice()
		if err != nil {
			log.WithError(err).Fatal("could not connect to device")
		}
		defer client.Disconnect()

		// No arguments - show current brightness
		if len(args) == 0 {
			currentBrightness, err := client.GetBrightnessInt()
			if err != nil {
				log.WithError(err).Fatal("failed to get current brightness")
			}
			fmt.Printf("Current brightness: %d\n", currentBrightness)
			return
		}

		arg := strings.ToLower(args[0])

		// Handle special commands
		switch arg {
		case "list":
			levels := nadapi.GetAvailableBrightnessLevels()
			fmt.Println("Available brightness levels:")
			for _, level := range levels {
				description := getBrightnessDescription(level)
				fmt.Printf("  %d - %s\n", level, description)
			}
			return

		case "up":
			err = client.ToggleBrightness(nadapi.DirectionUp)
			if err != nil {
				log.WithError(err).Fatal("failed to increase brightness")
			}
			newBrightness, err := client.GetBrightnessInt()
			if err == nil {
				fmt.Printf("Brightness increased to: %d\n", newBrightness)
			} else {
				fmt.Println("Brightness increased")
			}
			return

		case "down":
			err = client.ToggleBrightness(nadapi.DirectionDown)
			if err != nil {
				log.WithError(err).Fatal("failed to decrease brightness")
			}
			newBrightness, err := client.GetBrightnessInt()
			if err == nil {
				fmt.Printf("Brightness decreased to: %d\n", newBrightness)
			} else {
				fmt.Println("Brightness decreased")
			}
			return

		default:
			// Try to parse as a brightness level
			level, err := strconv.Atoi(args[0])
			if err != nil {
				fmt.Printf("Error: '%s' is not a valid brightness level.\n\n", args[0])
				fmt.Println("Usage:")
				fmt.Println("  nadctl dim              # Show current brightness")
				fmt.Println("  nadctl dim 0            # Set brightness to 0 (display off)")
				fmt.Println("  nadctl dim 2            # Set brightness to 2 (medium)")
				fmt.Println("  nadctl dim up           # Increase brightness")
				fmt.Println("  nadctl dim down         # Decrease brightness")
				fmt.Println("  nadctl dim list         # List all available levels")
				fmt.Printf("\nAvailable levels: %v\n", nadapi.GetAvailableBrightnessLevels())
				return
			}

			if !nadapi.IsValidBrightnessLevel(level) {
				fmt.Printf("Error: Brightness level %d is not valid.\n\n", level)
				fmt.Println("Available brightness levels:")
				levels := nadapi.GetAvailableBrightnessLevels()
				for _, l := range levels {
					description := getBrightnessDescription(l)
					fmt.Printf("  %d - %s\n", l, description)
				}
				return
			}

			err = client.SetBrightness(level)
			if err != nil {
				log.WithError(err).Fatal("failed to set brightness")
			}

			description := getBrightnessDescription(level)
			fmt.Printf("Brightness set to: %d (%s)\n", level, description)
		}
	},
}

// getBrightnessDescription returns a human-readable description for brightness levels
func getBrightnessDescription(level int) string {
	switch level {
	case 0:
		return "Display off/darkest"
	case 1:
		return "Low brightness"
	case 2:
		return "Medium brightness"
	case 3:
		return "High brightness"
	default:
		return "Unknown"
	}
}

func init() {
	rootCmd.AddCommand(dimCmd)
}
