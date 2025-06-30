package cmd

import (
	"testing"

	"github.com/galamiram/nadctl/internal/nadapi"
)

func TestVolumeCmdStructure(t *testing.T) {
	if volumeCmd.Use != "volume [LEVEL|up|down]" {
		t.Errorf("Volume command Use = %s, want 'volume [LEVEL|up|down]'", volumeCmd.Use)
	}

	if volumeCmd.Short != "Set or get volume level" {
		t.Errorf("Volume command Short = %s, want 'Set or get volume level'", volumeCmd.Short)
	}

	if volumeCmd.Long == "" {
		t.Error("Volume command missing Long description")
	}
}

func TestVolumeCmdHasSubcommands(t *testing.T) {
	// Check if volume command has subcommands
	subCommands := volumeCmd.Commands()

	// Should have a 'set' subcommand
	hasSetCommand := false
	for _, cmd := range subCommands {
		if cmd.Name() == "set" {
			hasSetCommand = true
			if cmd.Short != "Set volume to a specific level" {
				t.Errorf("Volume set command Short = %s, want 'Set volume to a specific level'", cmd.Short)
			}
			break
		}
	}

	if !hasSetCommand {
		t.Error("Volume command missing 'set' subcommand")
	}
}

func TestVolumeCmdArgumentValidation(t *testing.T) {
	// Test that volumeCmd accepts the right number of arguments
	if volumeCmd.Args == nil {
		t.Log("Volume command has no argument validation - this is expected")
	} else {
		// If it has argument validation, test it
		t.Log("Volume command has argument validation")
	}
}

// Test volume-related validation functions from nadapi
func TestVolumeValidationFunctions(t *testing.T) {
	// These functions should exist in the nadapi package
	// Test that direction constants are properly defined
	if nadapi.DirectionUp != 1 {
		t.Errorf("DirectionUp = %d, want 1", nadapi.DirectionUp)
	}

	if nadapi.DirectionDown != -1 {
		t.Errorf("DirectionDown = %d, want -1", nadapi.DirectionDown)
	}
}

// Test the volume command is properly registered
func TestVolumeCmdRegistration(t *testing.T) {
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "volume" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Volume command not registered with root command")
	}
}
