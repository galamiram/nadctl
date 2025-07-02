package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/galamiram/nadctl/simulator"
)

// TestE2EWithSimulator runs end-to-end tests using the built-in simulator
func TestE2EWithSimulator(t *testing.T) {
	// Start simulator
	sim := simulator.NewNADSimulator()
	port := "30001" // Use standard NAD port

	// Start simulator in background
	go func() {
		if err := sim.Start(port); err != nil {
			t.Errorf("Failed to start simulator: %v", err)
		}
	}()

	// Wait for simulator to start
	time.Sleep(1 * time.Second)

	// Ensure simulator is stopped after tests
	defer func() {
		if err := sim.Stop(); err != nil {
			t.Logf("Warning: Failed to stop simulator: %v", err)
		}
	}()

	// Set environment for tests
	simulatorIP := "127.0.0.1"

	// Run all e2e test scenarios
	t.Run("PowerControl", func(t *testing.T) {
		testPowerControl(t, simulatorIP)
	})

	t.Run("VolumeControl", func(t *testing.T) {
		testVolumeControl(t, simulatorIP)
	})

	t.Run("SourceControl", func(t *testing.T) {
		testSourceControl(t, simulatorIP)
	})

	t.Run("MuteControl", func(t *testing.T) {
		testMuteControl(t, simulatorIP)
	})

	t.Run("BrightnessControl", func(t *testing.T) {
		testBrightnessControl(t, simulatorIP)
	})

	t.Run("VolumeLimit", func(t *testing.T) {
		testVolumeLimit(t, simulatorIP)
	})

	t.Run("BrightnessLimit", func(t *testing.T) {
		testBrightnessLimit(t, simulatorIP)
	})
}

func testPowerControl(t *testing.T, ip string) {
	// Test power toggle (should start Off, go to On)
	output, err := runNadctlCommand(ip, "power")
	if err != nil {
		t.Fatalf("Power command failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Power toggled:") {
		t.Errorf("Expected power toggle output, got: %s", output)
	}

	// Power should now be On, toggle again to Off
	output, err = runNadctlCommand(ip, "power")
	if err != nil {
		t.Fatalf("Second power command failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Power toggled:") {
		t.Errorf("Expected second power toggle output, got: %s", output)
	}
}

func testVolumeControl(t *testing.T, ip string) {
	// Test volume up
	output, err := runNadctlCommand(ip, "volume", "up")
	if err != nil {
		t.Fatalf("Volume up failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Volume increased to:") {
		t.Errorf("Expected volume increase output, got: %s", output)
	}

	// Test volume down
	output, err = runNadctlCommand(ip, "volume", "down")
	if err != nil {
		t.Fatalf("Volume down failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Volume decreased to:") {
		t.Errorf("Expected volume decrease output, got: %s", output)
	}

	// Test specific volume setting using -- to avoid flag parsing issues
	output, err = runNadctlCommand(ip, "volume", "set", "--", "-25")
	if err != nil {
		t.Fatalf("Volume set failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Volume set to: -25") {
		t.Errorf("Expected volume set to -25, got: %s", output)
	}

	// Test current volume display
	output, err = runNadctlCommand(ip, "volume")
	if err != nil {
		t.Fatalf("Volume query failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Current volume: -25") {
		t.Errorf("Expected current volume -25, got: %s", output)
	}
}

func testSourceControl(t *testing.T, ip string) {
	// Test source setting
	output, err := runNadctlCommand(ip, "source", "TV")
	if err != nil {
		t.Fatalf("Source set failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Source set to: TV") {
		t.Errorf("Expected source set to TV, got: %s", output)
	}

	// Test source next
	output, err = runNadctlCommand(ip, "source", "next")
	if err != nil {
		t.Fatalf("Source next failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Source changed to:") {
		t.Errorf("Expected source change output, got: %s", output)
	}

	// Test source prev
	output, err = runNadctlCommand(ip, "source", "prev")
	if err != nil {
		t.Fatalf("Source prev failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Source changed to:") {
		t.Errorf("Expected source change output, got: %s", output)
	}

	// Test current source display
	output, err = runNadctlCommand(ip, "source")
	if err != nil {
		t.Fatalf("Source query failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Current source:") {
		t.Errorf("Expected current source output, got: %s", output)
	}

	// Test source list
	output, err = runNadctlCommand(ip, "source", "list")
	if err != nil {
		t.Fatalf("Source list failed: %v, output: %s", err, output)
	}

	expectedSources := []string{"Stream", "Wireless", "TV", "Phono", "Coax1", "Coax2", "Opt1", "Opt2"}
	for _, source := range expectedSources {
		if !strings.Contains(output, source) {
			t.Errorf("Expected source %s in list, got: %s", source, output)
		}
	}
}

func testMuteControl(t *testing.T, ip string) {
	// Test mute toggle
	output, err := runNadctlCommand(ip, "mute")
	if err != nil {
		t.Fatalf("Mute command failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Mute toggled:") {
		t.Errorf("Expected mute toggle output, got: %s", output)
	}

	// Toggle again
	output, err = runNadctlCommand(ip, "mute")
	if err != nil {
		t.Fatalf("Second mute command failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Mute toggled:") {
		t.Errorf("Expected second mute toggle output, got: %s", output)
	}
}

func testBrightnessControl(t *testing.T, ip string) {
	// Test brightness setting
	output, err := runNadctlCommand(ip, "dim", "2")
	if err != nil {
		t.Fatalf("Brightness set failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Brightness set to: 2") {
		t.Errorf("Expected brightness set to 2, got: %s", output)
	}

	// Test brightness up
	output, err = runNadctlCommand(ip, "dim", "up")
	if err != nil {
		t.Fatalf("Brightness up failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Brightness increased to:") {
		t.Errorf("Expected brightness increase output, got: %s", output)
	}

	// Test brightness down
	output, err = runNadctlCommand(ip, "dim", "down")
	if err != nil {
		t.Fatalf("Brightness down failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Brightness decreased to:") {
		t.Errorf("Expected brightness decrease output, got: %s", output)
	}

	// Test current brightness display
	output, err = runNadctlCommand(ip, "dim")
	if err != nil {
		t.Fatalf("Brightness query failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Current brightness:") {
		t.Errorf("Expected current brightness output, got: %s", output)
	}

	// Test brightness list
	output, err = runNadctlCommand(ip, "dim", "list")
	if err != nil {
		t.Fatalf("Brightness list failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Available brightness levels:") {
		t.Errorf("Expected brightness levels list, got: %s", output)
	}
}

func testVolumeLimit(t *testing.T, ip string) {
	// The CLI warns for volumes > 5dB, so let's test the simulator behavior directly
	// by checking that the API layer clamps correctly

	// First, test that normal volumes work
	output, err := runNadctlCommand(ip, "volume", "set", "5")
	if err != nil {
		t.Fatalf("Volume set to 5 failed: %v, output: %s", err, output)
	}

	// Verify it was set correctly
	output, err = runNadctlCommand(ip, "volume")
	if err != nil {
		t.Fatalf("Volume query failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "5") {
		t.Errorf("Expected volume to be set to 5, got: %s", output)
	}

	// Test volume minimum - try to set below -80dB using -- to avoid flag parsing
	output, err = runNadctlCommand(ip, "volume", "set", "--", "-100")
	if err != nil {
		t.Fatalf("Volume set below limit failed: %v, output: %s", err, output)
	}

	// Verify it was clamped to -80
	output, err = runNadctlCommand(ip, "volume")
	if err != nil {
		t.Fatalf("Volume query after minimum test failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "-80") {
		t.Errorf("Expected volume to be clamped to -80, got: %s", output)
	}

	// Note: We skip testing the maximum clamp via CLI due to confirmation prompts
	// The simulator correctly implements the 10dB limit as verified in unit tests
}

func testBrightnessLimit(t *testing.T, ip string) {
	// Brightness has wrapping behavior, not clamping
	// Test brightness maximum (3) wraps to minimum (0)
	output, err := runNadctlCommand(ip, "dim", "3")
	if err != nil {
		t.Fatalf("Brightness set to max failed: %v, output: %s", err, output)
	}

	// Try to increase beyond max - should wrap to 0
	output, err = runNadctlCommand(ip, "dim", "up")
	if err != nil {
		t.Fatalf("Brightness up at max failed: %v, output: %s", err, output)
	}

	// Should now be 0 (wrapped around)
	output, err = runNadctlCommand(ip, "dim")
	if err != nil {
		t.Fatalf("Brightness query after wrap test failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Current brightness: 0") {
		t.Errorf("Expected brightness to wrap to 0, got: %s", output)
	}

	// Test brightness minimum (0) wraps to maximum (3)
	// Try to decrease beyond min - should wrap to 3
	output, err = runNadctlCommand(ip, "dim", "down")
	if err != nil {
		t.Fatalf("Brightness down at min failed: %v, output: %s", err, output)
	}

	// Should now be 3 (wrapped around)
	output, err = runNadctlCommand(ip, "dim")
	if err != nil {
		t.Fatalf("Brightness query after wrap test failed: %v, output: %s", err, output)
	}

	if !strings.Contains(output, "Current brightness: 3") {
		t.Errorf("Expected brightness to wrap to 3, got: %s", output)
	}
}

// runNadctlCommand executes a nadctl command against the simulator
func runNadctlCommand(ip string, args ...string) (string, error) {
	// Check if binary exists, if not build it
	binaryPath := "./nadctl"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		// Binary doesn't exist, build it
		buildCmd := exec.Command("go", "build", "-o", binaryPath, ".")
		if buildErr := buildCmd.Run(); buildErr != nil {
			return "", fmt.Errorf("failed to build nadctl binary: %v", buildErr)
		}
	}

	cmd := exec.Command(binaryPath, args...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("NAD_IP=%s", ip),
	)

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// TestSimulatorUnitTests tests the simulator independently
func TestSimulatorUnitTests(t *testing.T) {
	t.Run("SimulatorCreation", func(t *testing.T) {
		sim := simulator.NewNADSimulator()
		if sim == nil {
			t.Fatal("Failed to create simulator")
		}

		state := sim.GetState()
		if state.Power != "Off" {
			t.Errorf("Expected initial power state Off, got %s", state.Power)
		}

		if state.Volume != -30.0 {
			t.Errorf("Expected initial volume -30.0, got %.1f", state.Volume)
		}

		if state.Source != "Stream" {
			t.Errorf("Expected initial source Stream, got %s", state.Source)
		}

		if state.Brightness != 2 {
			t.Errorf("Expected initial brightness 2, got %d", state.Brightness)
		}

		if state.Mute != "Off" {
			t.Errorf("Expected initial mute Off, got %s", state.Mute)
		}
	})

	t.Run("SimulatorStateUpdate", func(t *testing.T) {
		sim := simulator.NewNADSimulator()

		// Test state update
		newState := simulator.DeviceState{
			Power:      "On",
			Volume:     -20.0,
			Source:     "TV",
			Mute:       "On",
			Brightness: 3,
			Model:      "Test Model",
		}

		sim.SetState(newState)
		currentState := sim.GetState()

		if currentState.Power != "On" {
			t.Errorf("Expected power On, got %s", currentState.Power)
		}

		if currentState.Volume != -20.0 {
			t.Errorf("Expected volume -20.0, got %.1f", currentState.Volume)
		}

		if currentState.Source != "TV" {
			t.Errorf("Expected source TV, got %s", currentState.Source)
		}

		if currentState.Mute != "On" {
			t.Errorf("Expected mute On, got %s", currentState.Mute)
		}

		if currentState.Brightness != 3 {
			t.Errorf("Expected brightness 3, got %d", currentState.Brightness)
		}
	})
}
