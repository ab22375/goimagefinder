# AGENTS.md - GoImageFinder

This file provides essential information for AI coding agents working on the GoImageFinder project.

## Project Overview

GoImageFinder is a Go-based image similarity detection tool that indexes images using perceptual hashing and provides CLI and web interfaces for finding duplicate or similar images.

### Key Features
- Multi-format support: JPG, PNG, TIFF, RAW (NEF, CR2, CR3, RAF, ARW, DNG, RWL), and HEIC
- Perceptual hashing: Average hash (aHash) and perceptual hash (pHash)
- Smart similarity scoring with filename matching boost
- Incremental scanning with modification time checking
- Concurrent processing with multi-threaded workers
- JSON output mode for programmatic integration
- Graceful shutdown support via context cancellation

## Technology Stack

- **Language**: Go 1.24.1
- **Architecture**: Pure Go (no CGO) - single static binary
- **Database**: SQLite via `modernc.org/sqlite` (pure Go, no CGO)
- **Image Processing**: `github.com/disintegration/imaging` (pure Go)
- **Metadata Extraction**: `github.com/barasher/go-exiftool` for RAW files

### Key Dependencies
```
github.com/barasher/go-exiftool v1.10.0  // EXIF metadata extraction
github.com/disintegration/imaging v1.6.2  // Pure Go image processing
modernc.org/sqlite v1.34.5                // Pure Go SQLite driver
```

## Project Structure

```
goimagefinder/
├── main.go                      # CLI entry point
├── go.mod, go.sum              # Go module files
├── Makefile                    # Build automation
├── README.md                   # User documentation
├── CLAUDE.md                   # Quick reference for Claude Code
├── TEST_INSTRUCTIONS.md        # Manual testing guide
├── CHANGELOG.md                # Version history
│
├── database/
│   └── database.go             # SQLite operations (init, store, query)
│
├── imageprocessor/
│   ├── package.go              # Package exports
│   ├── image_processing.go     # Search and similarity algorithms
│   ├── hash_utils.go           # Average hash and perceptual hash (DCT)
│   ├── image_types.go          # FloatMatrix and image utilities
│   ├── formats.go              # Format detection (JPEG, PNG, RAW, etc.)
│   ├── loaders.go              # Image loader interface
│   ├── image_loader_registry.go # Registry for format loaders
│   ├── standard_loaders.go     # Standard format loaders (JPG, PNG, etc.)
│   ├── raw_image_loader.go     # RAW format loader
│   ├── tiff_loader.go          # TIFF format loader
│   ├── cr3_*.go                # Canon CR3 specific loaders
│   ├── thumbnail.go            # Thumbnail generation
│   └── utils.go                # Image utility functions
│
├── scanner/
│   ├── scanner.go              # Main scanning orchestration
│   ├── types.go                # ScanOptions, ProcessImageResult
│   ├── progress.go             # Progress tracking
│   ├── dbhelper.go             # Database helper for skip logic
│   ├── fileutils.go            # File format helpers
│   └── processor/
│       └── processor.go        # Scanner-to-imageprocessor adapter
│
├── types/
│   └── types.go                # Shared types (ImageInfo, ImageMatch)
│
├── utils/
│   └── utils.go                # CLI argument parsing
│
├── logging/
│   └── logging.go              # Debug logging with file output
│
├── output/
│   └── json.go                 # JSON output structures and helpers
│
├── signalhandler/
│   └── signalhandler.go        # SIGINT/SIGTERM handling with context
│
├── tests/
│   ├── database/               # Database package tests
│   ├── imageprocessor/         # Image processor tests
│   ├── scanner/                # Scanner tests
│   ├── integration/            # End-to-end integration tests
│   └── utils/                  # Test helpers
│
├── resources/                  # App resources (icons, etc.)
├── scripts/                    # Build scripts
├── build/                      # Build output directory
└── dist/                       # Distribution packages
```

## Build Commands

```bash
# Build CLI tool
make build

# Build for macOS ARM64 (Apple Silicon)
make build-macos-arm64

# Build web server (if webinterface/ exists)
make build-webserver

# Package as macOS .app
make package-macos

# Create distributable DMG
make create-dmg

# Install external RAW tools (exiftool, dcraw, libraw)
make install-tools

# Clean build artifacts
make clean
```

## Test Commands

```bash
# Run all tests
make test

# Run with coverage
make test-coverage

# Run benchmarks
make test-bench

# Run specific package tests
go test -v ./database
go test -v ./imageprocessor
go test -v ./scanner
go test -v ./tests/integration

# Run specific test file
go test -v ./tests/database/database_test.go

# Debug mode testing
make run-debug-scan      # Scan with debug logging
make run-debug-search    # Search with debug logging
```

## Code Style Guidelines

### Imports
Standard library first, internal packages second, external last:
```go
import (
    "context"
    "database/sql"
    "fmt"
    
    "imagefinder/database"
    "imagefinder/types"
    
    "github.com/disintegration/imaging"
)
```

### Error Handling
Explicit error checking with context wrapping:
```go
if err != nil {
    return fmt.Errorf("context about what failed: %v", err)
}
```

### Naming Conventions
- **Exported**: CamelCase (e.g., `ComputeAverageHash`, `ImageInfo`)
- **Unexported**: camelCase (e.g., `processImage`, `avgHash`)
- **Functions**: Use `VerbNoun` format (e.g., `ScanAndStoreFolder`, `FindSimilarImages`)
- **Types**: Exported types start with uppercase
- **Constants**: UPPER_SNAKE_CASE for constants

### Documentation
Add comments for exported functions and types:
```go
// ComputeAverageHash calculates a simple average hash for the image.
// Always returns a hexadecimal string representation.
func ComputeAverageHash(img image.Image) (string, error) {
```

### Formatting
- Use `gofmt` with standard Go formatting
- Run `go fmt ./...` before committing

## Architecture Details

### Image Processing Pipeline
1. **Load**: Use `ImageLoaderRegistry` to load image based on format
2. **Preprocess**: Convert to grayscale, normalize, apply Gaussian blur
3. **Hash**: Compute aHash (8x8) and pHash (32x32 DCT)
4. **Store**: Save hashes and metadata to SQLite

### Similarity Algorithm
1. **Average Hash (aHash)**: 8x8 grayscale, compare pixels to mean brightness
2. **Perceptual Hash (pHash)**: 32x32 DCT-based with median filtering
3. **Filename Boost**: Similar filenames get +0.0 to +0.15 score boost
4. **Final Score**: `0.7 * pHashSimilarity + 0.3 * avgHashSimilarity + filenameBoost`

### Concurrency Model
- **Scanner**: Walks directory and collects files
- **Worker Pool**: Semaphore-limited goroutines (default: 75% of CPU cores)
- **Chunk Processing**: Files processed in chunks of 100
- **Results Buffer**: Channel-based result forwarding with timeout handling
- **Graceful Shutdown**: Context cancellation stops new file processing

### Database Schema
```sql
CREATE TABLE images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL,
    source_prefix TEXT,
    format TEXT,
    width INTEGER,
    height INTEGER,
    created_at TEXT,
    modified_at TEXT,
    size INTEGER,
    average_hash TEXT,
    perceptual_hash TEXT,
    features BLOB,
    UNIQUE(path, source_prefix)
);

-- Indexes
idx_path, idx_source_prefix, idx_average_hash, idx_perceptual_hash
```

### SQLite Configuration
- **WAL Mode**: Enabled for concurrent reads during writes
- **Busy Timeout**: 5 seconds to retry locked operations
- **Synchronous**: NORMAL mode for performance with safety

## CLI Commands

### Scan
```bash
goimagefinder scan --folder=PATH [options]
  --database=PATH    Database file (default: ./images.db)
  --prefix=NAME      Source prefix for organizing
  --force            Force rewrite existing entries
  --json             Output JSON
  --progress         Stream progress as JSON lines
  --debug            Enable debug logging
  --logfile=PATH     Custom log file
```

### Search
```bash
goimagefinder search --image=PATH [options]
  --database=PATH    Database file
  --threshold=VALUE  Similarity threshold 0.0-1.0 (default: 0.8)
  --prefix=NAME      Filter by source prefix (comma-separated for multiple)
  --limit=N          Max results (default: 50)
  --json             Output JSON
  --debug            Enable debug logging
```

### Info
```bash
goimagefinder info [options]
  --database=PATH    Database file
  --json             Output JSON
```

## Testing Strategy

### Unit Tests
- Each package has tests in `tests/<package>/`
- Use `t.TempDir()` for temporary files
- Test both success and error cases
- Verify database state after operations

### Integration Tests
- Located in `tests/integration/`
- Test complete workflows: scan → search
- Test duplicate detection, cross-format similarity
- Test incremental scanning, prefix filtering

### Test Helpers
```go
// Create test images with specific patterns
createPatternImage(t, path, "checkerboard")  // checkerboard, gradient, solid, random
```

## Security Considerations

### File System
- Validate file paths before processing
- Skip files that can't be accessed (don't fail entire scan)
- Check file extensions against whitelist

### Database
- Use prepared statements to prevent SQL injection
- Unique constraint on (path, source_prefix) prevents duplicates

### Image Processing
- Recover from panics during image loading to prevent crashes
- Timeout on semaphore acquisition to prevent deadlocks
- Graceful shutdown on SIGINT/SIGTERM

### External Tools
- RAW processing uses external tools (exiftool, dcraw) - validate paths
- External tool output should be validated before use

## Common Tasks

### Adding a New RAW Format
1. Add format constant to `imageprocessor/formats.go`
2. Add extension to `formatExtensions` map
3. Add to `IsRawFormat()` check if needed
4. Update loader in `raw_image_loader.go` or create format-specific loader

### Adding a New CLI Command
1. Add command detection in `utils/utils.go` (ParseArguments)
2. Add handler function in `main.go`
3. Add JSON output types to `output/json.go` if needed
4. Add tests in `tests/` directory

### Modifying Database Schema
1. Update `InitDatabase()` in `database/database.go`
2. Add migration check for existing databases (see format column migration example)
3. Update `types/types.go` if struct changes
4. Update tests accordingly

## Debugging

### Enable Debug Mode
```bash
goimagefinder scan --folder=/path --debug --logfile=debug.log
goimagefinder search --image=query.jpg --debug --threshold=0.5
```

### Debug Logging Functions
- `logging.DebugLog()` - General debug info
- `logging.LogError()` - Error conditions
- `logging.LogImageProcessed()` - Image processing results

### Common Issues
1. **SQLITE_BUSY**: Check WAL mode and busy timeout settings
2. **Deadlocks**: Check semaphore acquisition/release balance
3. **Memory leaks**: Ensure image resources are released
4. **Panics**: Image loading has panic recovery, check logs for stack traces
