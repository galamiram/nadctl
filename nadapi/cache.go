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

// SpotifyTokenCache represents cached Spotify token information
type SpotifyTokenCache struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry"`
	ClientID     string    `json:"client_id"`
}

// AppCache represents the complete application cache
type AppCache struct {
	Discovery *CachedDiscovery   `json:"discovery,omitempty"`
	Spotify   *SpotifyTokenCache `json:"spotify,omitempty"`
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

	cache, err := LoadAppCache()
	if err != nil {
		log.WithError(err).Debug("Failed to load app cache")
		return nil, err
	}

	if cache.Discovery == nil {
		log.Debug("No discovery data in cache")
		return nil, nil // No discovery cache
	}

	discovery := cache.Discovery
	log.WithFields(log.Fields{
		"deviceCount": len(discovery.Devices),
		"timestamp":   discovery.Timestamp,
		"ttl":         discovery.TTL,
	}).Debug("Successfully loaded discovery cache")

	// Check if cache is expired
	age := time.Since(discovery.Timestamp)
	if age > discovery.TTL {
		log.WithFields(log.Fields{
			"age":       age,
			"ttl":       discovery.TTL,
			"expired":   true,
			"timestamp": discovery.Timestamp,
		}).Debug("Discovery cache is expired")
		return nil, nil // Cache expired
	}

	log.WithFields(log.Fields{
		"age":         age,
		"ttl":         discovery.TTL,
		"expired":     false,
		"deviceCount": len(discovery.Devices),
	}).Debug("Discovery cache is valid, returning cached devices")

	return discovery.Devices, nil
}

// SaveCachedDevices saves device discovery results to cache
func SaveCachedDevices(devices []DiscoveredDevice, ttl time.Duration) error {
	log.WithFields(log.Fields{
		"deviceCount": len(devices),
		"ttl":         ttl,
	}).Debug("Saving devices to cache")

	// Load existing cache to preserve Spotify tokens
	cache, err := LoadAppCache()
	if err != nil {
		cache = &AppCache{} // Start with empty cache if load fails
	}

	// Update discovery data
	cache.Discovery = &CachedDiscovery{
		Devices:   devices,
		Timestamp: time.Now(),
		TTL:       ttl,
	}

	return SaveAppCache(cache)
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

// LoadAppCache loads the complete application cache
func LoadAppCache() (*AppCache, error) {
	log.Debug("Loading application cache")

	cachePath, err := GetCacheFilePath()
	if err != nil {
		log.WithError(err).Debug("Failed to get cache file path")
		return nil, err
	}

	// Check if cache file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		log.WithField("cachePath", cachePath).Debug("Cache file does not exist")
		return &AppCache{}, nil // Return empty cache
	}

	// Read cache file
	data, err := os.ReadFile(cachePath)
	if err != nil {
		log.WithError(err).WithField("cachePath", cachePath).Debug("Failed to read cache file")
		return nil, fmt.Errorf("failed to read cache file: %v", err)
	}

	// Try to parse as new AppCache format first
	var appCache AppCache
	if err := json.Unmarshal(data, &appCache); err == nil && (appCache.Discovery != nil || appCache.Spotify != nil) {
		log.Debug("Successfully loaded new format app cache")
		return &appCache, nil
	}

	// Fall back to legacy CachedDiscovery format for backward compatibility
	var legacyCache CachedDiscovery
	if err := json.Unmarshal(data, &legacyCache); err != nil {
		log.WithError(err).WithField("cachePath", cachePath).Debug("Failed to parse cache file")
		return nil, fmt.Errorf("failed to parse cache file: %v", err)
	}

	log.Debug("Loaded legacy cache format, converting to new format")
	return &AppCache{Discovery: &legacyCache}, nil
}

// SaveAppCache saves the complete application cache
func SaveAppCache(cache *AppCache) error {
	log.Debug("Saving application cache")

	cachePath, err := GetCacheFilePath()
	if err != nil {
		log.WithError(err).Debug("Failed to get cache file path")
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		log.WithError(err).Debug("Failed to marshal cache data to JSON")
		return fmt.Errorf("failed to marshal cache data: %v", err)
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		log.WithError(err).WithField("cachePath", cachePath).Debug("Failed to write cache file")
		return fmt.Errorf("failed to write cache file: %v", err)
	}

	log.Debug("Successfully saved application cache")
	return nil
}

// LoadSpotifyToken loads cached Spotify token
func LoadSpotifyToken() (*SpotifyTokenCache, error) {
	log.Debug("Loading Spotify token from cache")

	cache, err := LoadAppCache()
	if err != nil {
		return nil, err
	}

	if cache.Spotify == nil {
		log.Debug("No Spotify token in cache")
		return nil, nil
	}

	log.Debug("Successfully loaded Spotify token from cache")
	return cache.Spotify, nil
}

// SaveSpotifyToken saves Spotify token to cache
func SaveSpotifyToken(tokenCache *SpotifyTokenCache) error {
	log.Debug("Saving Spotify token to cache")

	cache, err := LoadAppCache()
	if err != nil {
		cache = &AppCache{} // Start with empty cache if load fails
	}

	cache.Spotify = tokenCache

	return SaveAppCache(cache)
}

// ClearSpotifyToken removes Spotify token from cache
func ClearSpotifyToken() error {
	log.Debug("Clearing Spotify token from cache")

	cache, err := LoadAppCache()
	if err != nil {
		return err
	}

	cache.Spotify = nil

	return SaveAppCache(cache)
}
