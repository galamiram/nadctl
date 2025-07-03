# Changelog

## [1.3.0] - 2025-01-03

### Added
- **Spotify Device Selection & Casting**: Enhanced TUI with comprehensive device management
  - Interactive device selection interface in Spotify tab
  - Support for Chromecast, computers, smartphones, speakers, and other Cast-enabled devices
  - Real-time device listing with type icons (💻 🔊 📺 📱 🎵 🎧)
  - Visual device selection with ↑↓ navigation and Enter to cast
  - Active device highlighting and volume display
  - Automatic device refresh when entering selection mode
- **Enhanced Spotify CLI Commands**: Extended command-line interface
  - `nadctl spotify devices` - List all available Spotify Connect devices
  - `nadctl spotify transfer [device-name-or-index]` - Cast to specific device
  - Support for device selection by name or numeric index
  - Automatic playback continuation after device transfer
- **Streamlined TUI Workflow**: Improved user experience
  - Single 'y' key for device refresh and selection (no separate 'u' key needed)
  - Contextual help messages during device selection
  - Error handling for restricted devices and connection issues

### Enhanced
- **Spotify Integration**: Upgraded from basic playback to full device ecosystem management
  - Added device discovery and enumeration capabilities
  - Enhanced Spotify client with `GetAvailableDevices()` and `TransferPlaybackToDevice()` methods
  - Improved error handling and connection management
- **TUI Interface**: Enhanced Spotify tab with device management panel
  - Device list panel with selection highlighting
  - Real-time status updates (active/restricted devices)
  - Visual device type indicators and volume levels
  - Escape key handling for device selection mode cancellation

### Technical Details
- Added `CmdSpotifyListDevices` and `CmdSpotifyTransferDevice` command types
- Implemented `spotifyDevicesUpdateMsg` for device list updates
- Enhanced key bindings with `SpotifyDevices`, `SpotifyTransfer`, and navigation keys
- Added device management state tracking (`spotifyDevices`, `spotifyDeviceSelection`, `spotifyDeviceMode`)
- Integrated with Spotify Web API PlayerDevices and TransferPlayback endpoints

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