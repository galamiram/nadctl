package nadapi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mitchellh/go-homedir"
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
	cachePath, err := GetCacheFilePath()
	if err != nil {
		return nil, err
	}

	// Check if cache file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return nil, nil // No cache file, return empty
	}

	// Read cache file
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cache file: %v", err)
	}

	var cache CachedDiscovery
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse cache file: %v", err)
	}

	// Check if cache is expired
	if time.Since(cache.Timestamp) > cache.TTL {
		return nil, nil // Cache expired
	}

	return cache.Devices, nil
}

// SaveCachedDevices saves device discovery results to cache
func SaveCachedDevices(devices []DiscoveredDevice, ttl time.Duration) error {
	cachePath, err := GetCacheFilePath()
	if err != nil {
		return err
	}

	cache := CachedDiscovery{
		Devices:   devices,
		Timestamp: time.Now(),
		TTL:       ttl,
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %v", err)
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write cache file: %v", err)
	}

	return nil
}

// ClearCache removes the cached discovery results
func ClearCache() error {
	cachePath, err := GetCacheFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(cachePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to clear cache: %v", err)
	}

	return nil
}

// IsCacheValid checks if cached results exist and are still valid
func IsCacheValid() (bool, error) {
	devices, err := LoadCachedDevices()
	if err != nil {
		return false, err
	}
	return len(devices) > 0, nil
}

// DiscoverDevicesWithCache attempts to load from cache first, then discovers if needed
func DiscoverDevicesWithCache(timeout time.Duration, useCache bool, cacheTTL time.Duration) ([]DiscoveredDevice, bool, error) {
	var fromCache bool

	// Try to load from cache first if useCache is true
	if useCache {
		cachedDevices, err := LoadCachedDevices()
		if err == nil && len(cachedDevices) > 0 {
			return cachedDevices, true, nil
		}
	}

	// Perform fresh discovery
	devices, err := DiscoverDevices(timeout)
	if err != nil {
		return nil, false, err
	}

	// Save to cache if discovery was successful and we have devices
	if len(devices) > 0 {
		if err := SaveCachedDevices(devices, cacheTTL); err != nil {
			// Log error but don't fail the discovery
			fmt.Fprintf(os.Stderr, "Warning: failed to save cache: %v\n", err)
		}
	}

	return devices, fromCache, nil
}
