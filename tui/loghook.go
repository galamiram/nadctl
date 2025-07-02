package tui

import (
	"io"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
)

// TUILogHook is a logrus hook that captures log entries for display in the TUI
type TUILogHook struct {
	app    *App
	levels []logrus.Level
}

// NewTUILogHook creates a new TUI log hook
func NewTUILogHook(app *App) *TUILogHook {
	return &TUILogHook{
		app: app,
		levels: []logrus.Level{
			logrus.PanicLevel,
			logrus.FatalLevel,
			logrus.ErrorLevel,
			logrus.WarnLevel,
			logrus.InfoLevel,
			logrus.DebugLevel,
		},
	}
}

// Levels returns the available logging levels
func (hook *TUILogHook) Levels() []logrus.Level {
	return hook.levels
}

// Fire is called when a logging event is fired
func (hook *TUILogHook) Fire(entry *logrus.Entry) error {
	if hook.app == nil {
		return nil
	}

	// Convert logrus fields to our format
	fields := make(map[string]interface{})
	for k, v := range entry.Data {
		fields[k] = v
	}

	// Get the level name
	levelName := strings.ToUpper(entry.Level.String())

	// Format the message (remove newlines for cleaner display)
	message := strings.ReplaceAll(entry.Message, "\n", " ")

	// Truncate very long messages
	if len(message) > 200 {
		message = message[:200] + "..."
	}

	// Add the log entry to the TUI
	hook.app.addLogEntry(levelName, message, fields)

	return nil
}

// SetupTUILogging sets up logging to capture entries for the TUI only (no console output)
func SetupTUILogging(app *App) {
	// Create and add the TUI hook
	hook := NewTUILogHook(app)
	logrus.AddHook(hook)

	// Redirect logrus output to discard to prevent console interference with TUI
	// This ensures logs only go to our hook (and thus to the TUI panel) and to file if configured
	logrus.SetOutput(io.Discard)
}

// SetupTUILoggingWithFile sets up logging for TUI with optional file output
func SetupTUILoggingWithFile(app *App, logFile *os.File) {
	// Create and add the TUI hook
	hook := NewTUILogHook(app)
	logrus.AddHook(hook)

	// If file is provided, log to file but not console
	if logFile != nil {
		logrus.SetOutput(logFile)
	} else {
		// No file, discard console output entirely
		logrus.SetOutput(io.Discard)
	}
}
