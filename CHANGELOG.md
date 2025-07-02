# Changelog

## [1.1.0] - 2024-12-18

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