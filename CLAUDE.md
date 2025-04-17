# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands
- `make build` - Build application for current platform
- `make build-macos-arm64` - Build for macOS ARM64 (Apple Silicon)
- `make clean` - Remove build artifacts
- `make package-macos` - Create macOS .app package
- `make create-dmg` - Create distributable DMG file

## Test Commands
- `make test` - Run all tests
- `go test ./...` - Run all tests
- `go test ./packagename` - Run tests for specific package
- `go test -v ./packagename/filename_test.go` - Run specific test file

## Debug Commands
- `make run-debug-scan` - Run scan command with debug logging
- `make run-debug-search` - Run search command with debug logging

## Code Style Guidelines
- **Imports:** Standard library first, internal packages second, external last
- **Formatting:** Use `gofmt` with standard 4-space indentation
- **Error Handling:** Explicit error checking with context wrapping
- **Naming:** CamelCase for exported, camelCase for unexported identifiers
- **Types:** Exported types start with uppercase
- **Functions:** Use `VerbNoun` format (e.g., `ScanAndStoreFolder`)
- **Documentation:** Add comments for exported functions and types