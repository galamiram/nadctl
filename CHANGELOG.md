# Changelog

## [1.2.0] - 2025-07-03

### Added
- **Logs Panel**: Added dedicated scrollable logs tab in TUI
  - Real-time log capture and display within TUI interface
  - Scrollable log view with ↑↓ keys when in Logs tab
  - Prevents console log interference with TUI display
  - Stores up to 1000 log entries with automatic cleanup
  - Color-coded log levels (Error, Warning, Info, Debug)
- **Make Release Target**: Added `make release` command for automated releases
  - Automatically reads version from VERSION file
  - Creates git tag and pushes to origin
  - Force flag support for re-releasing same version

### Removed
- **Help Tab**: Removed redundant Help tab (help still shown at bottom of all tabs)

### Changed
- **TUI Layout**: Reorganized tabs from 5 to 4 (Device, Spotify, Settings, Logs)
- **Log Management**: Logs now go to file and TUI panel only (no console output in TUI mode)

### Technical Details
- Added `TUILogHook` for capturing logrus entries in real-time
- Implemented log scrolling and display management
- Updated tab navigation and key bindings
- Enhanced Makefile with automated release workflow

## [1.1.0] - 2025-07-03

### Added
- **Version Command**: Added `nadctl version` command to display version information
- **Build-time Version Injection**: Version is now injected at build time from VERSION file
- **Makefile Build System**: Added comprehensive Makefile with build targets
  - `make build` - Build with version injection
  - `make version` - Build and show version
  - `make test` - Run tests
  - `make demo` - Run TUI in demo mode
  - `make install` - Install to /usr/local/bin
  - `make clean` - Clean build artifacts

### Changed
- **TUI Settings Tab**: Now shows build-time version instead of hardcoded version
- **GoReleaser**: Updated to inject version information during release builds
- **Documentation**: Updated README.md with version command and build instructions

### Technical Details
- Version is stored in `VERSION` file and injected via Go's `-ldflags`
- Created `internal/version` package to manage version information
- Both CLI version command and TUI settings tab use the same version source
- Release builds automatically inject correct version via GoReleaser

## Previous Versions
See git history for changes prior to structured changelog. 