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
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/galamiram/nadctl/tui"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// tuiCmd represents the tui command
var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch terminal-based GUI for NAD receiver control",
	Long: `Launch an interactive terminal-based graphical user interface for controlling NAD receivers.

The TUI provides a modern, keyboard-driven interface with real-time status updates
and easy access to all device functions including power, volume, source selection,
mute control, and display brightness.

Keyboard shortcuts:
  q/Ctrl+C  - Quit
  p         - Toggle power
  m         - Toggle mute  
  +/-       - Volume up/down
  ←/→       - Previous/next source
  ↑/↓       - Brightness up/down
  r         - Refresh status
  d         - Discover devices

Examples:
  nadctl tui               # Launch the TUI interface`,
	Run: func(cmd *cobra.Command, args []string) {
		if debug {
			log.SetLevel(log.DebugLevel)
		}

		log.Debug("Launching TUI interface")

		// Create and run the TUI application
		app := tui.NewApp()
		p := tea.NewProgram(app, tea.WithAltScreen())

		if _, err := p.Run(); err != nil {
			log.WithError(err).Error("Failed to run TUI")
			fmt.Printf("Error running TUI: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
