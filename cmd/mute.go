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

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// muteCmd represents the mute command
var muteCmd = &cobra.Command{
	Use:   "mute",
	Short: "Toggle mute on and off",
	Long: `Toggle the mute state of the NAD device.

This command will automatically detect the current mute state
and switch it to the opposite state (muted->unmuted or unmuted->muted).

Examples:
  nadctl mute               # Toggle mute state`,
	Run: func(cmd *cobra.Command, args []string) {
		client, err := connectToDevice()
		if err != nil {
			log.WithError(err).Fatal("could not connect to device")
		}
		defer client.Disconnect()

		// Get current state to show what we're doing
		currentState, err := client.GetMuteStatus()
		if err != nil {
			log.WithError(err).Fatal("failed to get current mute state")
		}

		err = client.ToggleMute()
		if err != nil {
			log.WithError(err).Fatal("failed to toggle mute")
		}

		newState := "On"
		if currentState == "On" {
			newState = "Off"
		}
		fmt.Printf("Mute toggled: %s -> %s\n", currentState, newState)
	},
}

func init() {
	rootCmd.AddCommand(muteCmd)
}
