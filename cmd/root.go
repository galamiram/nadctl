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
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/galamiram/nadctl/nadapi"
)

var cfgFile string
var debug bool
var noCache bool
var clearCache bool
var debugMode bool
var demoMode bool
var logToFile bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nadctl",
	Short: "CLI for controlling NAD receivers",
	Run: func(cmd *cobra.Command, args []string) {
		// Set up file logging first if requested
		if logToFile {
			if err := setupFileLogging(); err != nil {
				log.WithError(err).Warn("Failed to set up file logging, continuing with console only")
			}
		}

		// Then set debug level if debug flag is set (this will override file logging level if needed)
		if debug {
			log.SetLevel(log.DebugLevel)
			// Also enable file logging if not already enabled
			if !logToFile {
				if err := setupFileLogging(); err != nil {
					log.WithError(err).Warn("Failed to set up file logging in debug mode, continuing with console only")
				}
			}
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
	rootCmd.PersistentFlags().BoolVar(&debugMode, "debug-mode", false, "enable debug mode")
	rootCmd.PersistentFlags().BoolVar(&demoMode, "demo", false, "enable demo mode (TUI without NAD device)")
	rootCmd.PersistentFlags().BoolVar(&logToFile, "log-to-file", false, "enable logging to file")

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
	log.Debug("Initializing configuration")

	if cfgFile != "" {
		// Use config file from the flag.
		log.WithField("configFile", cfgFile).Debug("Using config file from flag")
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			log.WithError(err).Debug("Failed to get home directory")
			fmt.Println(err)
			os.Exit(1)
		}

		log.WithField("homeDir", home).Debug("Found home directory")

		viper.SetConfigType("yaml")
		viper.AddConfigPath(home)
		viper.SetConfigName(".nadctl")

		log.WithFields(log.Fields{
			"configType": "yaml",
			"configPath": home,
			"configName": ".nadctl",
		}).Debug("Set default config file parameters")
	}

	// Bind environment variables
	viper.SetEnvPrefix("NAD")
	viper.AutomaticEnv()
	log.Debug("Environment variables bound with NAD prefix")

	// Read config file (ignore if it doesn't exist)
	if err := viper.ReadInConfig(); err == nil {
		log.WithField("configFile", viper.ConfigFileUsed()).Debug("Successfully loaded config file")
	} else {
		log.WithError(err).Debug("No config file found or failed to read (using defaults)")
	}

	// Log some key configuration values in debug mode
	if debug {
		ip := viper.GetString("ip")
		if ip != "" {
			log.WithField("ip", ip).Debug("IP address configured")
		} else {
			log.Debug("No IP address configured")
		}

		// Check environment variables
		if nadIP := os.Getenv("NAD_IP"); nadIP != "" {
			log.WithField("NAD_IP", nadIP).Debug("NAD_IP environment variable set")
		}
		if nadDebug := os.Getenv("NAD_DEBUG"); nadDebug != "" {
			log.WithField("NAD_DEBUG", nadDebug).Debug("NAD_DEBUG environment variable set")
		}
	}

	log.Debug("Configuration initialization completed")
}

// connectToDevice establishes a connection to a NAD device, with automatic discovery if no IP is configured
func connectToDevice() (*nadapi.Device, error) {
	ip := viper.GetString("ip")
	log.WithField("configuredIP", ip).Debug("Checking for configured IP address")

	if ip == "" {
		log.Debug("No IP address configured, proceeding with device discovery")
		useCache := !noCache
		cacheTTL := nadapi.DefaultCacheTTL

		log.WithFields(log.Fields{
			"useCache": useCache,
			"cacheTTL": cacheTTL,
		}).Debug("Device discovery configuration")

		if debug {
			if useCache {
				log.Info("No IP address configured, checking cache and discovering NAD devices...")
			} else {
				log.Info("No IP address configured, discovering NAD devices (cache disabled)...")
			}
		}

		log.Debug("Starting device discovery with cache")
		devices, fromCache, err := nadapi.DiscoverDevicesWithCache(30*time.Second, useCache, cacheTTL)
		if err != nil {
			log.WithError(err).Debug("Device discovery failed")
			return nil, fmt.Errorf("failed to discover devices: %v", err)
		}

		log.WithFields(log.Fields{
			"deviceCount": len(devices),
			"fromCache":   fromCache,
		}).Debug("Device discovery completed")

		if len(devices) == 0 {
			log.Debug("No NAD devices found during discovery")
			return nil, fmt.Errorf("no NAD devices found on the network. Please specify an IP address manually")
		}

		ip = devices[0].IP
		log.WithField("selectedIP", ip).Debug("Selected first discovered device")

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

				log.WithField("devices", devices).Debug("All discovered devices")
			}
		}
	} else {
		log.WithField("ip", ip).Debug("Using configured IP address")
	}

	log.WithField("ip", ip).Debug("Establishing connection to NAD device")
	device, err := nadapi.New(ip, "")
	if err != nil {
		log.WithError(err).WithField("ip", ip).Debug("Failed to connect to NAD device")
		return nil, err
	}

	log.WithField("ip", ip).Debug("Successfully connected to NAD device")
	return device, nil
}

// setupFileLogging configures file logging in addition to console logging
func setupFileLogging() error {
	return setupFileLoggingWithConsole(true)
}

// setupFileLoggingOnlyToFile configures file logging without console output
func setupFileLoggingOnlyToFile() error {
	return setupFileLoggingWithConsole(false)
}

// setupFileLoggingWithConsole configures file logging with optional console output
func setupFileLoggingWithConsole(includeConsole bool) error {
	// Get home directory
	home, err := homedir.Dir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %v", err)
	}

	// Create logs directory if it doesn't exist
	logDir := filepath.Join(home, ".nadctl_logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	// Create log file with timestamp
	logFile := filepath.Join(logDir, "nadctl.log")

	// Open log file
	file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	// Configure logrus to write to file only or file + console
	if includeConsole {
		log.SetOutput(io.MultiWriter(os.Stderr, file))
	} else {
		log.SetOutput(file)
	}

	// Set formatting for better file logs
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05.000",
		ForceColors:     false, // No colors in file logs
	})

	// Set log level to Debug when file logging is enabled to capture everything
	log.SetLevel(log.DebugLevel)

	log.WithField("logFile", logFile).Info("File logging enabled")
	if includeConsole {
		fmt.Printf("📝 Debug logs will be written to: %s\n", logFile)
	}

	return nil
}
