package nadapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
)

// CachedDiscovery represents cached device discovery results
type CachedDiscovery struct {
	Devices   []DiscoveredDevice `json:"devices"`
	Timestamp time.Time          `json:"timestamp"`
	TTL       time.Duration      `json:"ttl"`
}

// DefaultCacheTTL is the default time-to-live for cached discovery results
const DefaultCacheTTL = 5 * time.Minute

// getCacheFilePathFunc is a variable that can be overridden for testing
var getCacheFilePathFunc = defaultGetCacheFilePath

// defaultGetCacheFilePath returns the default path to the cache file
func defaultGetCacheFilePath() (string, error) {
	home, err := homedir.Dir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %v", err)
	}
	return filepath.Join(home, ".nadctl_cache.json"), nil
}

// GetCacheFilePath returns the path to the cache file
func GetCacheFilePath() (string, error) {
	return getCacheFilePathFunc()
}

// GetCacheFilePathFunc returns the current cache file path function (for testing)
func GetCacheFilePathFunc() func() (string, error) {
	return getCacheFilePathFunc
}

// SetCacheFilePathFunc sets the cache file path function (for testing)
func SetCacheFilePathFunc(fn func() (string, error)) {
	getCacheFilePathFunc = fn
}

// LoadCachedDevices loads cached device discovery results
func LoadCachedDevices() ([]DiscoveredDevice, error) {
	log.Debug("Loading cached devices")

	cachePath, err := GetCacheFilePath()
	if err != nil {
		log.WithError(err).Debug("Failed to get cache file path")
		return nil, err
	}

	log.WithField("cachePath", cachePath).Debug("Cache file path determined")

	// Check if cache file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		log.WithField("cachePath", cachePath).Debug("Cache file does not exist")
		return nil, nil // No cache file, return empty
	}

	log.WithField("cachePath", cachePath).Debug("Cache file exists, reading contents")

	// Read cache file
	data, err := os.ReadFile(cachePath)
	if err != nil {
		log.WithError(err).WithField("cachePath", cachePath).Debug("Failed to read cache file")
		return nil, fmt.Errorf("failed to read cache file: %v", err)
	}

	log.WithFields(log.Fields{
		"cachePath": cachePath,
		"dataSize":  len(data),
	}).Debug("Successfully read cache file")

	var cache CachedDiscovery
	if err := json.Unmarshal(data, &cache); err != nil {
		log.WithError(err).WithField("cachePath", cachePath).Debug("Failed to parse cache file JSON")
		return nil, fmt.Errorf("failed to parse cache file: %v", err)
	}

	log.WithFields(log.Fields{
		"deviceCount": len(cache.Devices),
		"timestamp":   cache.Timestamp,
		"ttl":         cache.TTL,
	}).Debug("Successfully parsed cache file")

	// Check if cache is expired
	age := time.Since(cache.Timestamp)
	if age > cache.TTL {
		log.WithFields(log.Fields{
			"age":       age,
			"ttl":       cache.TTL,
			"expired":   true,
			"timestamp": cache.Timestamp,
		}).Debug("Cache is expired")
		return nil, nil // Cache expired
	}

	log.WithFields(log.Fields{
		"age":         age,
		"ttl":         cache.TTL,
		"expired":     false,
		"deviceCount": len(cache.Devices),
	}).Debug("Cache is valid, returning cached devices")

	return cache.Devices, nil
}

// SaveCachedDevices saves device discovery results to cache
func SaveCachedDevices(devices []DiscoveredDevice, ttl time.Duration) error {
	log.WithFields(log.Fields{
		"deviceCount": len(devices),
		"ttl":         ttl,
	}).Debug("Saving devices to cache")

	cachePath, err := GetCacheFilePath()
	if err != nil {
		log.WithError(err).Debug("Failed to get cache file path")
		return err
	}

	log.WithField("cachePath", cachePath).Debug("Cache file path determined for saving")

	cache := CachedDiscovery{
		Devices:   devices,
		Timestamp: time.Now(),
		TTL:       ttl,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		log.WithError(err).Debug("Failed to marshal cache data to JSON")
		return fmt.Errorf("failed to marshal cache data: %v", err)
	}

	log.WithFields(log.Fields{
		"cachePath": cachePath,
		"dataSize":  len(data),
	}).Debug("Successfully marshaled cache data")

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		log.WithError(err).WithField("cachePath", cachePath).Debug("Failed to write cache file")
		return fmt.Errorf("failed to write cache file: %v", err)
	}

	log.WithFields(log.Fields{
		"cachePath":   cachePath,
		"deviceCount": len(devices),
		"ttl":         ttl,
	}).Debug("Successfully saved devices to cache file")

	return nil
}

// ClearCache removes the cached discovery results
func ClearCache() error {
	log.Debug("Clearing cache")

	cachePath, err := GetCacheFilePath()
	if err != nil {
		log.WithError(err).Debug("Failed to get cache file path for clearing")
		return err
	}

	log.WithField("cachePath", cachePath).Debug("Attempting to clear cache file")

	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		log.WithError(err).WithField("cachePath", cachePath).Debug("Failed to remove cache file")
		return fmt.Errorf("failed to clear cache: %v", err)
	}

	if os.IsNotExist(err) {
		log.WithField("cachePath", cachePath).Debug("Cache file did not exist (nothing to clear)")
	} else {
		log.WithField("cachePath", cachePath).Debug("Successfully cleared cache file")
	}

	return nil
}

// IsCacheValid checks if cached results exist and are still valid
func IsCacheValid() (bool, error) {
	log.Debug("Checking if cache is valid")

	devices, err := LoadCachedDevices()
	if err != nil {
		log.WithError(err).Debug("Error while loading cached devices for validation")
		return false, err
	}

	valid := len(devices) > 0
	log.WithFields(log.Fields{
		"valid":       valid,
		"deviceCount": len(devices),
	}).Debug("Cache validation completed")

	return valid, nil
}

// DiscoverDevicesWithCache attempts to load from cache first, then discovers if needed
func DiscoverDevicesWithCache(timeout time.Duration, useCache bool, cacheTTL time.Duration) ([]DiscoveredDevice, bool, error) {
	log.WithFields(log.Fields{
		"timeout":  timeout,
		"useCache": useCache,
		"cacheTTL": cacheTTL,
	}).Debug("Starting device discovery with cache")

	var fromCache bool

	// Try to load from cache first if useCache is true
	if useCache {
		log.Debug("Attempting to load devices from cache")
		cachedDevices, err := LoadCachedDevices()
		if err == nil && len(cachedDevices) > 0 {
			log.WithField("deviceCount", len(cachedDevices)).Debug("Successfully loaded devices from cache")
			return cachedDevices, true, nil
		}
		if err != nil {
			log.WithError(err).Debug("Error loading from cache, will perform fresh discovery")
		} else {
			log.Debug("No valid cached devices found, will perform fresh discovery")
		}
	} else {
		log.Debug("Cache disabled, performing fresh discovery")
	}

	// Perform fresh discovery
	log.WithField("timeout", timeout).Debug("Performing fresh device discovery")
	devices, err := DiscoverDevices(timeout)
	if err != nil {
		log.WithError(err).Debug("Fresh device discovery failed")
		return nil, false, err
	}

	log.WithField("deviceCount", len(devices)).Debug("Fresh device discovery completed")

	// Save to cache if discovery was successful and we have devices
	if len(devices) > 0 {
		log.WithFields(log.Fields{
			"deviceCount": len(devices),
			"cacheTTL":    cacheTTL,
		}).Debug("Saving discovered devices to cache")

		if err := SaveCachedDevices(devices, cacheTTL); err != nil {
			// Log error but don't fail the discovery
			log.WithError(err).Debug("Failed to save devices to cache (continuing anyway)")
			fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
		} else {
			log.Debug("Successfully saved discovered devices to cache")
		}
	} else {
		log.Debug("No devices found, not saving to cache")
	}

	log.WithFields(log.Fields{
		"deviceCount": len(devices),
		"fromCache":   fromCache,
	}).Debug("Device discovery with cache completed")

	return devices, fromCache, nil
}
