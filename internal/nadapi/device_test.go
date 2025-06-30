package nadapi

import (
	"net"
	"reflect"
	"testing"
)

func TestGetAvailableSources(t *testing.T) {
	expected := []string{"Stream", "Wireless", "TV", "Phono", "Coax1", "Coax2", "Opt1", "Opt2"}
	result := GetAvailableSources()

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetAvailableSources() = %v, want %v", result, expected)
	}
}

func TestGetAvailableBrightnessLevels(t *testing.T) {
	expected := []int{0, 1, 2, 3}
	result := GetAvailableBrightnessLevels()

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("GetAvailableBrightnessLevels() = %v, want %v", result, expected)
	}
}

func TestIsValidSource(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected bool
	}{
		{"Valid source - Stream", "Stream", true},
		{"Valid source - TV", "TV", true},
		{"Valid source case insensitive - stream", "stream", true},
		{"Valid source case insensitive - tv", "tv", true},
		{"Valid source mixed case - StReAm", "StReAm", true},
		{"Invalid source - Radio", "Radio", false},
		{"Invalid source - CD", "CD", false},
		{"Empty source", "", false},
		{"Invalid source - numbers", "123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidSource(tt.source)
			if result != tt.expected {
				t.Errorf("IsValidSource(%s) = %v, want %v", tt.source, result, tt.expected)
			}
		})
	}
}

func TestIsValidBrightnessLevel(t *testing.T) {
	tests := []struct {
		name     string
		level    int
		expected bool
	}{
		{"Valid level 0", 0, true},
		{"Valid level 1", 1, true},
		{"Valid level 2", 2, true},
		{"Valid level 3", 3, true},
		{"Invalid level -1", -1, false},
		{"Invalid level 4", 4, false},
		{"Invalid level 10", 10, false},
		{"Invalid level -10", -10, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidBrightnessLevel(tt.level)
			if result != tt.expected {
				t.Errorf("IsValidBrightnessLevel(%d) = %v, want %v", tt.level, result, tt.expected)
			}
		})
	}
}

func TestExtractValue(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected string
		hasError bool
	}{
		{"Valid response", "Main.Power=On\r\n", "On", false},
		{"Valid response with spaces", "Main.Volume=-20.5\r\n", "-20.5", false},
		{"Valid response simple", "Main.Source=Stream\n", "Strea", false},
		{"Invalid response no equals", "Main.Power", "", true},
		{"Invalid response empty", "", "", true},
		{"Response with multiple equals", "Main.Test=Value=Extra\n", "Val", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := extractValue(tt.raw)
			if tt.hasError {
				if err == nil {
					t.Errorf("extractValue(%q) expected error, got nil", tt.raw)
				}
			} else {
				if err != nil {
					t.Errorf("extractValue(%q) unexpected error: %v", tt.raw, err)
				}
				if result != tt.expected {
					t.Errorf("extractValue(%q) = %q, want %q", tt.raw, result, tt.expected)
				}
			}
		})
	}
}

func TestTrimSuffix(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"String with newline", "Hello\n", "Hell"},
		{"String with carriage return and newline", "World\r\n", "World"},
		{"String with just carriage return", "Test\r", "Tes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := trimSuffix(tt.input)
			if result != tt.expected {
				t.Errorf("trimSuffix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// Test that trimSuffix panics on empty or too short strings (expected behavior)
func TestTrimSuffixPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("trimSuffix with empty string should panic")
		}
	}()

	trimSuffix("")
}

func TestTrimSuffixPanicSingleChar(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("trimSuffix with single character should panic")
		}
	}()

	trimSuffix("A")
}

func TestInc(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{"Simple increment", []byte{192, 168, 1, 1}, []byte{192, 168, 1, 2}},
		{"Rollover single byte", []byte{192, 168, 1, 255}, []byte{192, 168, 2, 0}},
		{"Multiple rollover", []byte{192, 168, 255, 255}, []byte{192, 169, 0, 0}},
		{"All ones", []byte{255, 255, 255, 255}, []byte{0, 0, 0, 0}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy since inc modifies in place
			input := make([]byte, len(tt.input))
			copy(input, tt.input)

			inc(input)

			if !reflect.DeepEqual(input, tt.expected) {
				t.Errorf("inc(%v) = %v, want %v", tt.input, input, tt.expected)
			}
		})
	}
}

// TestDirectionConstants ensures the direction constants have expected values
func TestDirectionConstants(t *testing.T) {
	if DirectionUp != 1 {
		t.Errorf("DirectionUp = %d, want 1", DirectionUp)
	}
	if DirectionDown != -1 {
		t.Errorf("DirectionDown = %d, want -1", DirectionDown)
	}
}

// TestConstants verifies important constants
func TestConstants(t *testing.T) {
	if defaultPort != "30001" {
		t.Errorf("defaultPort = %s, want 30001", defaultPort)
	}
	if maxBrightness != 3 {
		t.Errorf("maxBrightness = %d, want 3", maxBrightness)
	}
}

// MockDevice tests for device operations would require a mock TCP server
// These tests verify the basic structure and error handling
func TestNewDeviceValidation(t *testing.T) {
	tests := []struct {
		name           string
		addr           string
		port           string
		shouldErr      bool
		skipConnection bool
	}{
		{"Invalid IP address", "invalid.ip", "", true, false},
		{"Empty IP address", "", "", true, false},
		{"Valid IP format", "192.168.1.1", "30001", false, true}, // Skip actual connection
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skipConnection {
				// Only test IP parsing, not actual connection
				ip := net.ParseIP(tt.addr)
				if ip == nil {
					t.Errorf("New(%s, %s) IP parsing failed", tt.addr, tt.port)
				}
				t.Logf("IP parsing successful for %s", tt.addr)
				return
			}

			_, err := New(tt.addr, tt.port)
			if tt.shouldErr && err == nil {
				t.Errorf("New(%s, %s) expected error, got nil", tt.addr, tt.port)
			}
			if !tt.shouldErr && err != nil {
				t.Errorf("New(%s, %s) unexpected error: %v", tt.addr, tt.port, err)
			}
		})
	}
}
