# Read version from VERSION file
VERSION := $(shell cat VERSION | tr -d '\n')

# Build the application with version injected
build:
	go build -ldflags "-X github.com/galamiram/nadctl/internal/version.Version=$(VERSION)" -o nadctl .

# Install the application
install: build
	mv nadctl /usr/local/bin/

# Clean build artifacts
clean:
	rm -f nadctl

# Test the application
test:
	go test ./...

# Run the TUI in demo mode
demo:
	./nadctl tui --demo

# Show current version (after building)
version: build
	./nadctl version

.PHONY: build install clean test demo version 