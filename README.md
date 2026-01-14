# GoImageFinder

A Go-based image similarity detection tool that indexes images using perceptual hashing and provides both CLI and web interfaces for finding duplicate or similar images.

## Features

- **Multi-format support**: JPG, PNG, TIFF, RAW (NEF, CR2, CR3, RAF, ARW, DNG), and HEIC
- **Perceptual hashing**: Average hash (aHash) and perceptual hash (pHash) for robust comparison
- **Smart similarity scoring**: Weighted hash comparison with filename matching boost
- **Incremental scanning**: Skips unchanged files unless forced
- **Concurrent processing**: Multi-threaded for optimal performance
- **Web interface**: Browser-based GUI with drag-and-drop search

## Requirements

### Go Dependencies
```bash
go get github.com/mattn/go-sqlite3
go get gocv.io/x/gocv
go get github.com/barasher/go-exiftool
go get golang.org/x/image
```

### External Tools
- **exiftool** - Metadata and preview extraction
- **dcraw** - RAW image conversion
- **libraw** - RAW image processing
- **rawtherapee-cli** (optional) - Alternative RAW processor

Install on macOS:
```bash
brew install dcraw exiftool libraw rawtherapee
```

Or use make:
```bash
make install-tools
```

## Installation

### From Source
```bash
git clone https://github.com/yourusername/goimagefinder
cd goimagefinder
make build            # Build CLI tool
make build-webserver  # Build web interface
```

Binaries are created in `./build/`:
- `./build/goimagefinder` - CLI tool
- `./build/webserver` - Web interface

### macOS DMG
Download the DMG from releases, install, then create a symlink:
```bash
sudo rm -f /usr/local/bin/goimagefinder
echo '#!/bin/bash' | sudo tee /usr/local/bin/goimagefinder > /dev/null
echo 'exec /Applications/goimagefinder.app/Contents/MacOS/goimagefinder "$@"' | sudo tee -a /usr/local/bin/goimagefinder > /dev/null
sudo chmod +x /usr/local/bin/goimagefinder
```

## Command Line Usage

### Scan Images
Index a directory of images into a SQLite database:

```bash
goimagefinder scan --folder=/path/to/images [options]
```

| Option | Description |
|--------|-------------|
| `--database=PATH` | Database file path (default: ./images.db) |
| `--prefix=NAME` | Source prefix for organizing images |
| `--force` | Force rewrite of existing entries |
| `--debug` | Enable debug logging |
| `--logfile=PATH` | Custom log file path |

Example:
```bash
goimagefinder scan --folder=/Users/photos --db=photos.db --prefix=MacBook --debug
```

### Search Similar Images
Find images similar to a query image:

```bash
goimagefinder search --image=/path/to/query.jpg [options]
```

| Option | Description |
|--------|-------------|
| `--database=PATH` | Database file path |
| `--threshold=VALUE` | Similarity threshold 0.0-1.0 (default: 0.8) |
| `--prefix=NAME` | Filter results by source prefix |
| `--debug` | Enable debug logging |
| `--logfile=PATH` | Custom log file path |

Example:
```bash
goimagefinder search --image=vacation.jpg --db=photos.db --threshold=0.75
```

## Web Interface

### Start the Server
```bash
./build/webserver [port]
```

Or run without building:
```bash
go run ./cmd/webserver/ [port]
```

Or build and run in one step:
```bash
make webserver
```

Default port is 8012. Access at `http://localhost:8012`

### Web Interface Features

**Configuration**
- Settings persist in `~/.goimagefinder/webserver.json`
- Remembers database path, folder, and scan options between sessions

**Database Management**
- Visual file browser for database selection
- Real-time record count display
- Automatic database creation

**Image Scanning**
- Folder selection with built-in file browser
- Source prefix configuration
- Force rewrite option
- Real-time progress tracking

**Similarity Search**
- Drag-and-drop or click to select query images
- Adjustable similarity threshold slider
- Thumbnail previews for all formats including RAW
- Clickable paths and copy-to-clipboard buttons
- Similarity scores for each match

### Workflow
1. Open `http://localhost:8012`
2. Select or create a database using "..." button
3. Select images folder using "..." button
4. (Optional) Set source prefix
5. Click "Scan" to index images
6. Select a query image and click "Search"

## Project Structure

```
goimagefinder/
├── cmd/webserver/          # Web interface
│   ├── main.go             # Server entry point
│   ├── config.go           # Configuration management
│   ├── static/             # JavaScript and CSS
│   └── templates/          # HTML templates
├── database/               # SQLite operations
├── imageprocessor/         # Image loading and hashing
│   ├── formats.go          # Format detection
│   ├── loaders.go          # Loader interfaces
│   └── format_*.go         # Format-specific loaders
├── scanner/                # Directory scanning
│   └── processor/          # Scanner adapter
├── logging/                # Debug and error logging
├── types/                  # Common type definitions
├── utils/                  # Utility functions
├── main.go                 # CLI entry point
└── Makefile                # Build scripts
```

## Build Commands

```bash
make build              # Build for current platform
make build-macos-arm64  # Build for Apple Silicon
make webserver          # Build and run web interface
make build-webserver    # Build web server only
make clean              # Remove build artifacts
make package-macos      # Create macOS .app bundle
make create-dmg         # Create distributable DMG
make test               # Run tests
make install-tools      # Install external RAW tools
make help               # Show all targets
```

## Debug Mode

Enable with `--debug` flag. Logs include:
- Image processing details
- Hash computation values
- Error diagnostics
- Processing statistics
- Search match details

## Technical Details

### Similarity Algorithm
1. **Average Hash (aHash)**: 8x8 grayscale, compares pixels to mean brightness
2. **Perceptual Hash (pHash)**: 32x32 DCT-based with median filtering
3. **Filename boost**: Similar filenames (e.g., IMG_1234.JPG and IMG_1234.CR2) get a score boost

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
```

## License

MIT License

## Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/new-feature`)
3. Commit changes (`git commit -m 'Add new feature'`)
4. Push branch (`git push origin feature/new-feature`)
5. Open Pull Request
