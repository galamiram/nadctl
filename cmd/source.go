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
	"strings"

	"github.com/galamiram/nadctl/nadapi"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// sourceCmd represents the source command
var sourceCmd = &cobra.Command{
	Use:   "source [SOURCE_NAME|next|prev|list]",
	Short: "Set or get input source",
	Long: `Set the input source to a specific source or cycle through sources.

Available sources: Stream, Wireless, TV, Phono, Coax1, Coax2, Opt1, Opt2

Examples:
  nadctl source              # Show current source
  nadctl source Stream       # Set source to Stream
  nadctl source tv           # Set source to TV (case-insensitive)  
  nadctl source next         # Switch to next source
  nadctl source prev         # Switch to previous source
  nadctl source list         # List all available sources`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		client, err := connectToDevice()
		if err != nil {
			log.WithError(err).Fatal("could not connect to device")
		}
		defer client.Disconnect()

		log.WithField("device", client.IP.String()).Debug("Connected to device for source command")

		// No arguments - show current source
		if len(args) == 0 {
			log.Debug("No arguments provided, getting current source")
			currentSource, err := client.GetSource()
			if err != nil {
				log.WithError(err).Fatal("failed to get current source")
			}
			fmt.Printf("Current source: %s\n", currentSource)
			return
		}

		arg := strings.ToLower(args[0])
		log.WithFields(log.Fields{
			"argument": arg,
			"original": args[0],
		}).Debug("Processing source command argument")

		switch arg {
		case "list":
			log.Debug("Listing available sources")
			sources := nadapi.GetAvailableSources()
			fmt.Println("Available sources:")
			for i, source := range sources {
				fmt.Printf("  %d. %s\n", i+1, source)
			}
			return

		case "next":
			log.Debug("Changing to next source")
			newSource, err := client.ToggleSource(nadapi.DirectionUp)
			if err != nil {
				log.WithError(err).Fatal("failed to change source")
			}
			// Extract the source name from response
			if val, extractErr := extractValue(newSource); extractErr == nil {
				log.WithField("newSource", val).Debug("Successfully changed to next source")
				fmt.Printf("Source changed to: %s\n", val)
			} else {
				log.WithError(extractErr).Debug("Failed to extract source name from response")
				fmt.Println("Source changed to next")
			}
			return

		case "prev", "previous":
			log.Debug("Changing to previous source")
			newSource, err := client.ToggleSource(nadapi.DirectionDown)
			if err != nil {
				log.WithError(err).Fatal("failed to change source")
			}
			// Extract the source name from response
			if val, extractErr := extractValue(newSource); extractErr == nil {
				log.WithField("newSource", val).Debug("Successfully changed to previous source")
				fmt.Printf("Source changed to: %s\n", val)
			} else {
				log.WithError(extractErr).Debug("Failed to extract source name from response")
				fmt.Println("Source changed to previous")
			}
			return

		default:
			// Try to set to specific source
			log.WithField("sourceName", arg).Debug("Attempting to set specific source")

			if !nadapi.IsValidSource(arg) {
				log.WithFields(log.Fields{
					"invalidSource":    arg,
					"availableSources": nadapi.GetAvailableSources(),
				}).Debug("Invalid source name provided")
				fmt.Printf("Error: '%s' is not a valid source name.\n\n", arg)
				fmt.Println("Available sources:")
				sources := nadapi.GetAvailableSources()
				for i, source := range sources {
					fmt.Printf("  %d. %s\n", i+1, source)
				}
				fmt.Println("\nYou can also use: next, prev, list")
				return
			}

			log.WithField("sourceName", arg).Debug("Source name validated, setting source")
			err = client.SetSource(arg)
			if err != nil {
				log.WithError(err).Fatal("failed to set source")
			}

			// Find the proper case for the source name
			sources := nadapi.GetAvailableSources()
			var properName string
			for _, s := range sources {
				if strings.EqualFold(s, arg) {
					properName = s
					break
				}
			}

			log.WithFields(log.Fields{
				"requestedSource": arg,
				"actualSource":    properName,
			}).Debug("Successfully set source")
			fmt.Printf("Source set to: %s\n", properName)
		}
	},
}

// Helper function to extract value from response (copied from nadapi package)
func extractValue(raw string) (string, error) {
	s := strings.Split(raw, "=")
	if len(s) > 1 {
		// Simple trim of newline and carriage return
		result := strings.TrimRight(s[1], "\r\n")
		return result, nil
	}
	return "", fmt.Errorf("failed to extract value")
}

func init() {
	rootCmd.AddCommand(sourceCmd)
}
