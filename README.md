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

### macOS App (Recommended)
Build and install the web interface as a native macOS application:

```bash
make create-webserver-dmg
```

This creates `dist/GoImageFinder.dmg`.

**Installation:**
1. Open `dist/GoImageFinder.dmg`
2. Drag **GoImageFinder** to the Applications folder
3. **First launch only:** Right-click the app → Select "Open" → Click "Open" in the dialog
   (Required because the app is not signed with an Apple Developer certificate)

**What happens when you double-click:**
1. Starts the web server on port 8012 (configurable)
2. Waits for server to be ready
3. Automatically opens `http://localhost:8012` in your default browser
4. App icon appears in Dock while running

**To stop:** Quit the app from the Dock or close the terminal window

**Note:** There are two different DMGs:
- `make create-webserver-dmg` → **GoImageFinder.dmg** (web interface with auto-browser launch)
- `make create-dmg` → **goimagefinder.dmg** (CLI tool only, no GUI)

### Docker (Cross-Platform)
The easiest way to run GoImageFinder on any platform:

```bash
# Using docker-compose (recommended)
docker-compose up -d --build

# Or build and run manually
docker build -t goimagefinder .
docker run -d -p 8012:8012 \
  -v goimagefinder_data:/data \
  -v ~/Pictures:/photos:ro \
  goimagefinder
```

Access at `http://localhost:8012`

**Docker Compose with custom settings:**
```bash
# Custom port
PORT=9000 docker-compose up -d

# Stop
docker-compose down
```

Mount your photo directories by editing `docker-compose.yml`:
```yaml
volumes:
  - /path/to/your/photos:/photos:ro
```

### macOS CLI (Legacy)
For the CLI tool only:
```bash
make create-dmg
# Install from dist/goimagefinder.dmg
sudo ln -sf /Applications/goimagefinder.app/Contents/MacOS/goimagefinder /usr/local/bin/goimagefinder
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

### Configuration File

Settings are stored in `~/.goimagefinder/webserver.json`:

```json
{
  "port": 8012,
  "databasePath": "/Users/you/goimagefinder.db",
  "folderPath": "",
  "threshold": 0.75,
  "prefix": "",
  "forceRewrite": false,
  "openBrowser": true
}
```

| Setting | Description | Default |
|---------|-------------|---------|
| `port` | Web server port | 8012 |
| `databasePath` | Default database file | `~/goimagefinder.db` |
| `threshold` | Similarity threshold (0.0-1.0) | 0.75 |
| `openBrowser` | Auto-open browser on start | true |

### Web Interface Features

**Configuration**
- Settings persist in `~/.goimagefinder/webserver.json`
- Remembers database path, folder, and scan options between sessions
- Port and auto-browser configurable

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
- **Batch search**: Select up to 20 images at once
- Adjustable similarity threshold slider
- Thumbnail previews for all formats including RAW
- Clickable paths and copy-to-clipboard buttons
- Similarity scores for each match
- Grouped results view for batch searches with collapsible sections

### Workflow
1. Open `http://localhost:8012`
2. Select or create a database using "..." button
3. Select images folder using "..." button
4. (Optional) Set source prefix
5. Click "Scan" to index images
6. Select one or more query images and click "Search"

### Batch Search
The web interface supports searching for multiple images at once:

1. Click "Select Images" and choose multiple files (Cmd/Ctrl+click)
2. Preview grid shows all selected images with remove buttons
3. Click "Search N Images" to search all at once
4. Results are grouped by query image in collapsible sections
5. Summary shows total matches, successes, and any errors

**Limits**: Maximum 20 images per batch, 50MB total upload size

### API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/scan` | POST | Scan folder for images (SSE progress) |
| `/api/search` | POST | Search with image path (JSON) |
| `/api/upload-search` | POST | Search with uploaded image (multipart) |
| `/api/batch-search` | POST | Search multiple images at once (multipart) |
| `/api/file` | GET | Serve image file or thumbnail |
| `/api/config` | GET/POST | Get/save configuration |
| `/api/database-info` | GET | Get database info and record count |
| `/api/browse` | GET | Browse filesystem for file picker |

## Project Structure

```
goimagefinder/
├── cmd/webserver/          # Web interface
│   ├── main.go             # Server entry point
│   ├── config.go           # Configuration management
│   ├── batch_search_test.go # Batch search tests
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
├── resources/              # App resources
│   └── launcher.sh         # macOS app launcher script
├── main.go                 # CLI entry point
├── Makefile                # Build scripts
├── Dockerfile              # Docker build configuration
└── docker-compose.yml      # Docker Compose configuration
```

## Build Commands

```bash
# Building
make build                  # Build for current platform
make build-macos-arm64      # Build for Apple Silicon
make build-webserver        # Build web server only
make webserver              # Build and run web interface

# macOS Distribution
make create-webserver-dmg   # Create GoImageFinder.dmg (web UI, recommended)
make create-dmg             # Create goimagefinder.dmg (CLI only)
make package-webserver-macos # Create .app bundle for web interface
make package-macos          # Create .app bundle for CLI

# Docker
make docker-build           # Build Docker image
make docker-run             # Run Docker container
make docker-compose-up      # Start with docker-compose
make docker-compose-down    # Stop docker-compose services

# Other
make clean                  # Remove build artifacts
make test                   # Run tests
make install-tools          # Install external RAW tools
make help                   # Show all targets
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
