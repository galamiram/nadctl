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
	"time"

	"github.com/galamiram/nadctl/internal/nadapi"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// discoverCmd represents the discover command
var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Discover NAD devices on the network",
	Long: `Scan the local network for NAD devices and display their information.
This command will search all local network interfaces for devices
listening on the standard NAD port (30001) and verify they are
NAD devices by querying their model information.

The discovery results are cached for faster subsequent operations.
Use --no-cache to bypass the cache or --clear-cache to reset it.`,
	Run: func(cmd *cobra.Command, args []string) {
		timeout, _ := cmd.Flags().GetDuration("timeout")
		forceRefresh, _ := cmd.Flags().GetBool("refresh")
		showCache, _ := cmd.Flags().GetBool("show-cache")

		// Handle cache status display
		if showCache {
			displayCacheStatus()
			return
		}

		useCache := !noCache && !forceRefresh
		cacheTTL := nadapi.DefaultCacheTTL

		log.Info("Scanning network for NAD devices...")
		devices, fromCache, err := nadapi.DiscoverDevicesWithCache(timeout, useCache, cacheTTL)
		if err != nil {
			log.WithError(err).Fatal("Failed to discover devices")
		}

		if len(devices) == 0 {
			fmt.Println("No NAD devices found on the network")
			return
		}

		// Show cache status
		cacheStatus := "from network scan"
		if fromCache {
			cacheStatus = "from cache"
		}

		fmt.Printf("Found %d NAD device(s) (%s):\n\n", len(devices), cacheStatus)
		for i, device := range devices {
			fmt.Printf("%d. %s\n", i+1, device.Model)
			fmt.Printf("   IP: %s:%s\n", device.IP, device.Port)
			fmt.Println()
		}

		fmt.Println("To use a specific device, set the IP in your config file or use:")
		fmt.Printf("export NAD_IP=%s\n", devices[0].IP)

		if fromCache {
			fmt.Printf("\nNote: Results loaded from cache. Use --refresh to scan network again.\n")
		}
	},
}

func displayCacheStatus() {
	valid, err := nadapi.IsCacheValid()
	if err != nil {
		fmt.Printf("Error checking cache: %v\n", err)
		return
	}

	if !valid {
		fmt.Println("Cache: No valid cached devices found")
		return
	}

	devices, err := nadapi.LoadCachedDevices()
	if err != nil {
		fmt.Printf("Error loading cache: %v\n", err)
		return
	}

	fmt.Printf("Cache status: %d device(s) cached\n", len(devices))
	for i, device := range devices {
		fmt.Printf("  %d. %s at %s:%s\n", i+1, device.Model, device.IP, device.Port)
	}

	// Try to read cache file for timestamp info
	cachePath, err := nadapi.GetCacheFilePath()
	if err == nil {
		if info, err := os.Stat(cachePath); err == nil {
			fmt.Printf("Cache file: %s\n", cachePath)
			fmt.Printf("Last updated: %s\n", info.ModTime().Format("2006-01-02 15:04:05"))
		}
	}
}

func init() {
	rootCmd.AddCommand(discoverCmd)
	discoverCmd.Flags().DurationP("timeout", "t", 30*time.Second, "Discovery timeout")
	discoverCmd.Flags().BoolP("refresh", "r", false, "Force refresh by bypassing cache")
	discoverCmd.Flags().Bool("show-cache", false, "Show current cache status")
}
