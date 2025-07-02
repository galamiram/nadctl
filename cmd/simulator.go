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

	"github.com/galamiram/nadctl/simulator"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var simulatorPort string

// simulatorCmd represents the simulator command
var simulatorCmd = &cobra.Command{
	Use:   "simulator",
	Short: "Start NAD device simulator for testing",
	Long: `Start a NAD device simulator that responds to all NAD protocol commands.

This is useful for testing the TUI interface and CLI commands without needing
a real NAD device. The simulator listens on port 30001 (default NAD port)
and responds to all supported commands with realistic behavior.

The simulator maintains state for:
- Power (On/Off)
- Volume (-80 to +10 dB)
- Source (Stream, Wireless, TV, Phono, Coax1, Coax2, Opt1, Opt2)
- Mute (On/Off)  
- Brightness (0-3)
- Device model

Examples:
  nadctl simulator                    # Start simulator on port 30001
  nadctl simulator --port 30002       # Start on custom port
  
Then in another terminal:
  NAD_IP=127.0.0.1 nadctl tui         # Connect TUI to simulator
  NAD_IP=127.0.0.1 nadctl volume up   # Test CLI commands`,
	Run: func(cmd *cobra.Command, args []string) {
		if debug {
			log.SetLevel(log.DebugLevel)
		}

		log.Info("🎵 Starting NAD Device Simulator...")

		// Create and start simulator
		sim := simulator.NewNADSimulator()

		if err := sim.Start(simulatorPort); err != nil {
			log.WithError(err).Fatal("Failed to start simulator")
		}

		// Print usage instructions
		fmt.Println()
		fmt.Println("📱 NAD Device Simulator is running!")
		fmt.Println()
		fmt.Println("🔗 To connect your TUI:")
		fmt.Printf("   NAD_IP=127.0.0.1 %s tui\n", os.Args[0])
		fmt.Println()
		fmt.Println("🔧 To test CLI commands:")
		fmt.Printf("   NAD_IP=127.0.0.1 %s power\n", os.Args[0])
		fmt.Printf("   NAD_IP=127.0.0.1 %s volume up\n", os.Args[0])
		fmt.Printf("   NAD_IP=127.0.0.1 %s source next\n", os.Args[0])
		fmt.Println()
		fmt.Println("⏹️  Press Ctrl+C to stop the simulator")
		fmt.Println()

		// Wait for interrupt signal
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		<-sigChan

		log.Info("Shutting down simulator...")
		if err := sim.Stop(); err != nil {
			log.WithError(err).Error("Error stopping simulator")
		}

		fmt.Println("Simulator stopped. Goodbye! 👋")
	},
}

func init() {
	rootCmd.AddCommand(simulatorCmd)
	simulatorCmd.Flags().StringVar(&simulatorPort, "port", "30001", "Port to listen on")
}
