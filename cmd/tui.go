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
	"os/signal"
	"syscall"

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
  nadctl tui               # Launch the TUI interface
  nadctl tui --demo        # Launch TUI in demo mode (no NAD device required)`,
	Run: func(cmd *cobra.Command, args []string) {
		log.Debug("Launching TUI interface")

		// Check if demo mode is enabled
		demoMode, _ := cmd.Flags().GetBool("demo")
		if demoMode {
			log.Info("Starting TUI in demo mode - NAD device connection not required")
		}

		// Create and run the TUI application
		app := tui.NewApp()

		// Set demo mode in the app if flag is set
		if demoMode {
			app.SetDemoMode(true)
		}

		// Set up TUI logging based on configuration
		logToFile, _ := cmd.Root().PersistentFlags().GetBool("log-to-file")
		if debug {
			log.SetLevel(log.DebugLevel)
		}

		if logToFile || debug {
			// Set up file logging and TUI logging
			if err := setupFileLoggingOnlyToFile(); err != nil {
				log.WithError(err).Warn("Failed to set up file logging, logs will only appear in TUI")
				// Still set up TUI logging without file
				tui.SetupTUILogging(app)
			} else {
				// We need to get the file handle to pass to TUI logging
				// For now, use the simpler approach and let file logging be separate
				tui.SetupTUILogging(app)
			}
		} else {
			// No file logging requested, just TUI logging (no console output)
			tui.SetupTUILogging(app)
		}

		// Set up signal handling for graceful cleanup
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Start signal handler in background
		go func() {
			sig := <-sigChan
			log.WithField("signal", sig.String()).Debug("Received signal, initiating cleanup")

			// Perform cleanup
			if err := app.Cleanup(); err != nil {
				log.WithError(err).Debug("Errors occurred during signal cleanup")
			}

			// Exit gracefully
			fmt.Println("\nGraceful shutdown complete")
			os.Exit(0)
		}()

		p := tea.NewProgram(app, tea.WithAltScreen(), tea.WithMouseCellMotion())

		if _, err := p.Run(); err != nil {
			log.WithError(err).Error("Failed to run TUI")
			fmt.Printf("Error running TUI: %v\n", err)

			// Still perform cleanup even if TUI failed
			if cleanupErr := app.Cleanup(); cleanupErr != nil {
				log.WithError(cleanupErr).Debug("Errors occurred during error cleanup")
			}

			os.Exit(1)
		}

		// Normal exit - cleanup is handled by the quit key handler in the TUI
		log.Debug("TUI exited normally")
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
	// Add demo flag to the TUI command
	tuiCmd.Flags().BoolP("demo", "d", false, "run in demo mode without NAD device connection")
}
