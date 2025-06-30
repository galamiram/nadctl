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

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/galamiram/nadctl/internal/nadapi"
)

var cfgFile string
var debug bool
var noCache bool
var clearCache bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nadctl",
	Short: "CLI for controlling NAD receivers",
	Run: func(cmd *cobra.Command, args []string) {
		if debug {
			log.SetLevel(log.DebugLevel)
		}

		device, err := connectToDevice()
		if err != nil {
			log.WithError(err).Fatal("Failed to connect to device")
		}
		log.WithField("ip", device.IP).Info("Successfully connected to NAD device")
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.nadctl.yaml)")
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "enable debug")
	rootCmd.PersistentFlags().BoolVar(&noCache, "no-cache", false, "disable device discovery cache")
	rootCmd.PersistentFlags().BoolVar(&clearCache, "clear-cache", false, "clear device discovery cache and exit")

	// Handle clear cache flag
	cobra.OnInitialize(func() {
		if clearCache {
			if err := nadapi.ClearCache(); err != nil {
				fmt.Printf("Error clearing cache: %v\n", err)
				os.Exit(1)
			}
			fmt.Println("Cache cleared successfully")
			os.Exit(0)
		}
	})
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		viper.SetConfigType("yaml")
		viper.AddConfigPath(home)
		viper.SetConfigName(".nadctl")
	}

	// Bind environment variables
	viper.SetEnvPrefix("NAD")
	viper.AutomaticEnv()

	// Read config file (ignore if it doesn't exist)
	viper.ReadInConfig()
}

// connectToDevice establishes a connection to a NAD device, with automatic discovery if no IP is configured
func connectToDevice() (*nadapi.Device, error) {
	ip := viper.GetString("ip")
	if ip == "" {
		useCache := !noCache
		cacheTTL := nadapi.DefaultCacheTTL

		if debug {
			if useCache {
				log.Info("No IP address configured, checking cache and discovering NAD devices...")
			} else {
				log.Info("No IP address configured, discovering NAD devices (cache disabled)...")
			}
		}

		devices, fromCache, err := nadapi.DiscoverDevicesWithCache(30*time.Second, useCache, cacheTTL)
		if err != nil {
			return nil, fmt.Errorf("failed to discover devices: %v", err)
		}

		if len(devices) == 0 {
			return nil, fmt.Errorf("no NAD devices found on the network. Please specify an IP address manually")
		}

		ip = devices[0].IP
		if debug {
			cacheStatus := "from network scan"
			if fromCache {
				cacheStatus = "from cache"
			}

			if len(devices) == 1 {
				log.WithFields(log.Fields{
					"ip":     ip,
					"model":  devices[0].Model,
					"source": cacheStatus,
				}).Info("Automatically discovered and using NAD device")
			} else {
				log.WithFields(log.Fields{
					"count":  len(devices),
					"source": cacheStatus,
				}).Info("Multiple NAD devices found, using first one")
			}
		}
	}

	return nadapi.New(ip, "")
}
