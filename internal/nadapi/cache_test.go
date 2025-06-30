package nadapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetCacheFilePath(t *testing.T) {
	path, err := GetCacheFilePath()
	if err != nil {
		t.Fatalf("GetCacheFilePath() error = %v", err)
	}

	if path == "" {
		t.Error("GetCacheFilePath() returned empty path")
	}

	if !filepath.IsAbs(path) {
		t.Errorf("GetCacheFilePath() = %s, want absolute path", path)
	}

	if filepath.Base(path) != ".nadctl_cache.json" {
		t.Errorf("GetCacheFilePath() = %s, want filename '.nadctl_cache.json'", filepath.Base(path))
	}
}

func TestSaveAndLoadCachedDevices(t *testing.T) {
	// Create test devices
	testDevices := []DiscoveredDevice{
		{IP: "192.168.1.100", Model: "NAD C338", Port: "30001"},
		{IP: "192.168.1.101", Model: "NAD C368", Port: "30001"},
	}

	// Create a temporary cache file
	tempDir := t.TempDir()
	originalGetCacheFilePathFunc := getCacheFilePathFunc
	getCacheFilePathFunc = func() (string, error) {
		return filepath.Join(tempDir, ".nadctl_cache.json"), nil
	}
	defer func() { getCacheFilePathFunc = originalGetCacheFilePathFunc }()

	// Test saving
	ttl := 5 * time.Minute
	err := SaveCachedDevices(testDevices, ttl)
	if err != nil {
		t.Fatalf("SaveCachedDevices() error = %v", err)
	}

	// Test loading
	loadedDevices, err := LoadCachedDevices()
	if err != nil {
		t.Fatalf("LoadCachedDevices() error = %v", err)
	}

	if len(loadedDevices) != len(testDevices) {
		t.Errorf("LoadCachedDevices() returned %d devices, want %d", len(loadedDevices), len(testDevices))
	}

	for i, device := range loadedDevices {
		if device.IP != testDevices[i].IP {
			t.Errorf("LoadCachedDevices()[%d].IP = %s, want %s", i, device.IP, testDevices[i].IP)
		}
		if device.Model != testDevices[i].Model {
			t.Errorf("LoadCachedDevices()[%d].Model = %s, want %s", i, device.Model, testDevices[i].Model)
		}
		if device.Port != testDevices[i].Port {
			t.Errorf("LoadCachedDevices()[%d].Port = %s, want %s", i, device.Port, testDevices[i].Port)
		}
	}
}

func TestLoadCachedDevicesNonExistent(t *testing.T) {
	// Create a temporary directory with no cache file
	tempDir := t.TempDir()
	originalGetCacheFilePathFunc := getCacheFilePathFunc
	getCacheFilePathFunc = func() (string, error) {
		return filepath.Join(tempDir, ".nadctl_cache.json"), nil
	}
	defer func() { getCacheFilePathFunc = originalGetCacheFilePathFunc }()

	// Should return nil for non-existent cache
	devices, err := LoadCachedDevices()
	if err != nil {
		t.Errorf("LoadCachedDevices() error = %v, want nil", err)
	}
	if devices != nil {
		t.Errorf("LoadCachedDevices() = %v, want nil", devices)
	}
}

func TestLoadCachedDevicesExpired(t *testing.T) {
	// Create test cache with expired TTL
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, ".nadctl_cache.json")

	expiredCache := CachedDiscovery{
		Devices: []DiscoveredDevice{
			{IP: "192.168.1.100", Model: "NAD C338", Port: "30001"},
		},
		Timestamp: time.Now().Add(-10 * time.Minute), // 10 minutes ago
		TTL:       5 * time.Minute,                   // 5 minute TTL (expired)
	}

	data, err := json.MarshalIndent(expiredCache, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test cache: %v", err)
	}

	err = os.WriteFile(cachePath, data, 0644)
	if err != nil {
		t.Fatalf("Failed to write test cache file: %v", err)
	}

	// Mock getCacheFilePathFunc
	originalGetCacheFilePathFunc := getCacheFilePathFunc
	getCacheFilePathFunc = func() (string, error) {
		return cachePath, nil
	}
	defer func() { getCacheFilePathFunc = originalGetCacheFilePathFunc }()

	// Should return nil for expired cache
	devices, err := LoadCachedDevices()
	if err != nil {
		t.Errorf("LoadCachedDevices() error = %v, want nil", err)
	}
	if devices != nil {
		t.Errorf("LoadCachedDevices() = %v, want nil (expired cache)", devices)
	}
}

func TestClearCache(t *testing.T) {
	// Create a temporary cache file
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, ".nadctl_cache.json")

	// Create dummy cache file
	err := os.WriteFile(cachePath, []byte(`{"devices":[],"timestamp":"2023-01-01T00:00:00Z","ttl":300000000000}`), 0644)
	if err != nil {
		t.Fatalf("Failed to create test cache file: %v", err)
	}

	// Mock getCacheFilePathFunc
	originalGetCacheFilePathFunc := getCacheFilePathFunc
	getCacheFilePathFunc = func() (string, error) {
		return cachePath, nil
	}
	defer func() { getCacheFilePathFunc = originalGetCacheFilePathFunc }()

	// Clear cache
	err = ClearCache()
	if err != nil {
		t.Errorf("ClearCache() error = %v", err)
	}

	// Verify file is deleted
	if _, err := os.Stat(cachePath); !os.IsNotExist(err) {
		t.Error("ClearCache() did not delete cache file")
	}
}

func TestClearCacheNonExistent(t *testing.T) {
	// Test clearing non-existent cache (should not error)
	tempDir := t.TempDir()
	cachePath := filepath.Join(tempDir, ".nadctl_cache.json")

	// Mock getCacheFilePathFunc
	originalGetCacheFilePathFunc := getCacheFilePathFunc
	getCacheFilePathFunc = func() (string, error) {
		return cachePath, nil
	}
	defer func() { getCacheFilePathFunc = originalGetCacheFilePathFunc }()

	// Should not error when clearing non-existent cache
	err := ClearCache()
	if err != nil {
		t.Errorf("ClearCache() error = %v, want nil", err)
	}
}

func TestIsCacheValid(t *testing.T) {
	tempDir := t.TempDir()
	originalGetCacheFilePathFunc := getCacheFilePathFunc
	getCacheFilePathFunc = func() (string, error) {
		return filepath.Join(tempDir, ".nadctl_cache.json"), nil
	}
	defer func() { getCacheFilePathFunc = originalGetCacheFilePathFunc }()

	// Test with no cache
	valid, err := IsCacheValid()
	if err != nil {
		t.Errorf("IsCacheValid() error = %v", err)
	}
	if valid {
		t.Error("IsCacheValid() = true, want false (no cache)")
	}

	// Create valid cache
	testDevices := []DiscoveredDevice{
		{IP: "192.168.1.100", Model: "NAD C338", Port: "30001"},
	}
	err = SaveCachedDevices(testDevices, 5*time.Minute)
	if err != nil {
		t.Fatalf("SaveCachedDevices() error = %v", err)
	}

	// Test with valid cache
	valid, err = IsCacheValid()
	if err != nil {
		t.Errorf("IsCacheValid() error = %v", err)
	}
	if !valid {
		t.Error("IsCacheValid() = false, want true (valid cache)")
	}
}

func TestDiscoverDevicesWithCacheNoCache(t *testing.T) {
	// Mock getCacheFilePathFunc to use temp directory
	tempDir := t.TempDir()
	originalGetCacheFilePathFunc := getCacheFilePathFunc
	getCacheFilePathFunc = func() (string, error) {
		return filepath.Join(tempDir, ".nadctl_cache.json"), nil
	}
	defer func() { getCacheFilePathFunc = originalGetCacheFilePathFunc }()

	// Test discovery with cache disabled
	// Note: This test will attempt actual network discovery, so we'll skip it in CI
	// devices, fromCache, err := DiscoverDevicesWithCache(1*time.Second, false, 5*time.Minute)
	// We'll test the structure instead
	t.Log("Skipping actual network discovery test - would require mocking network operations")
}

func TestDiscoverDevicesWithCacheFromCache(t *testing.T) {
	// Mock getCacheFilePathFunc to use temp directory
	tempDir := t.TempDir()
	originalGetCacheFilePathFunc := getCacheFilePathFunc
	getCacheFilePathFunc = func() (string, error) {
		return filepath.Join(tempDir, ".nadctl_cache.json"), nil
	}
	defer func() { getCacheFilePathFunc = originalGetCacheFilePathFunc }()

	// Pre-populate cache
	testDevices := []DiscoveredDevice{
		{IP: "192.168.1.100", Model: "NAD C338", Port: "30001"},
	}
	err := SaveCachedDevices(testDevices, 5*time.Minute)
	if err != nil {
		t.Fatalf("SaveCachedDevices() error = %v", err)
	}

	// Test discovery with cache enabled
	devices, fromCache, err := DiscoverDevicesWithCache(10*time.Second, true, 5*time.Minute)
	if err != nil {
		t.Fatalf("DiscoverDevicesWithCache() error = %v", err)
	}

	if !fromCache {
		t.Error("DiscoverDevicesWithCache() fromCache = false, want true (should use cache)")
	}

	if len(devices) != 1 {
		t.Errorf("DiscoverDevicesWithCache() returned %d devices, want 1", len(devices))
	}

	if devices[0].IP != "192.168.1.100" {
		t.Errorf("DiscoverDevicesWithCache()[0].IP = %s, want 192.168.1.100", devices[0].IP)
	}
}

func TestDefaultCacheTTL(t *testing.T) {
	expected := 5 * time.Minute
	if DefaultCacheTTL != expected {
		t.Errorf("DefaultCacheTTL = %v, want %v", DefaultCacheTTL, expected)
	}
}
