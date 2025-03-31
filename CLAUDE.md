# GoImageFinder - Claude.md

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
- **Formatting:** Use `gofmt` (standard Go formatting with 4-space indentation)
- **Error Handling:** Explicit error checking, error wrapping with context
- **Naming:** CamelCase for exported identifiers, camelCase for unexported
- **Types:** Exported types start with uppercase, custom types for specialization
- **Functions:** Use `VerbNoun` format (e.g., `ScanAndStoreFolder`)
- **Errors:** Always propagate errors up call stack with context