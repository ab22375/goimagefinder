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

### Pure Go - No CGO Required

GoImageFinder is built entirely in pure Go with no CGO dependencies:
- **Single static binary** - just download and run
- **No compiler needed** - works on any macOS without Xcode
- **Cross-platform compatible** - same binary works across OS versions

Go dependencies are automatically managed via `go mod`.

### External Tools (for RAW support)
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
3. **First launch only** (required because the app is not signed):

   **Option A - System Preferences:**
   - Double-click the app (it will be blocked silently)
   - Open **System Settings → Privacy & Security**
   - Scroll down to find "GoImageFinder was blocked"
   - Click **Open Anyway** → Enter password → Click **Open**

   **Option B - Terminal (faster):**
   ```bash
   xattr -cr /Applications/GoImageFinder.app
   ```
   Then double-click the app normally.

**What happens when you launch:**
1. Starts the web server on port 8012 (configurable)
2. Waits for server to be ready
3. Automatically opens `http://localhost:8012` in your default browser
4. App icon appears in Dock while running

**To stop:** Click "Quit" in the web UI, or quit the app from the Dock

**Troubleshooting:**
- If the app doesn't open or browser doesn't launch, check `~/.goimagefinder/webserver.json`
- Ensure `port` is set to `8012` (not `0`) and `openBrowser` is `true`
- View logs at `~/.goimagefinder/logs/webserver.log`

**Note:** There are two different DMGs:
- `make create-webserver-dmg` → **GoImageFinder.dmg** (web interface with auto-browser launch)
- `make create-dmg` → **goimagefinder.dmg** (CLI tool only, no GUI)

### Docker (Cross-Platform)
The easiest way to run GoImageFinder on any platform.

#### Quick Start (Single Photo Folder)

**Step 1: Pull the image**
```bash
docker pull ab22375/goimagefinder
```

**Step 2: Run with your photo folder**
```bash
docker run -d -p 8012:8012 \
  -v /path/to/your/photos:/photos \
  -v goimagefinder_data:/data \
  ab22375/goimagefinder
```

**Step 3: Open browser**
```
http://localhost:8012
```

#### Multiple Photo Folders

To browse and scan photos from multiple locations (internal drive, external SSD, backup drive, etc.):

**Step 1: Pull the image**
```bash
docker pull ab22375/goimagefinder
```

**Step 2: Run with multiple volume mounts and set browse roots**
```bash
docker run -d -p 8012:8012 \
  -v /Users/me/Pictures:/photos \
  -v /Volumes/ExternalSSD:/external \
  -v /Volumes/Backup:/backup \
  -v goimagefinder_data:/data \
  -e GOIMAGEFINDER_BROWSE_ROOTS="/photos,/external,/backup" \
  ab22375/goimagefinder
```

**Step 3: Open browser** - All mounted locations will be available in the web UI
```
http://localhost:8012
```

#### Using Docker Compose (Recommended)

**Step 1: Create or edit `docker-compose.yml`**
```yaml
version: '3.8'
services:
  goimagefinder:
    image: ab22375/goimagefinder
    container_name: goimagefinder
    ports:
      - "8012:8012"
    volumes:
      - goimagefinder_data:/data
      # Add your photo folders here:
      - /Users/me/Pictures:/photos:ro
      - /Volumes/ExternalSSD:/external:ro
      - /Volumes/Backup:/backup:ro
    environment:
      # List all container paths you mounted above:
      - GOIMAGEFINDER_BROWSE_ROOTS=/photos,/external,/backup
    restart: unless-stopped

volumes:
  goimagefinder_data:
```

**Step 2: Start**
```bash
docker-compose up -d
```

**Step 3: View logs (optional)**
```bash
docker-compose logs -f
```

**Step 4: Stop**
```bash
docker-compose down
```

#### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `GOIMAGEFINDER_PORT` | HTTP server port | 8012 |
| `GOIMAGEFINDER_DATABASE_PATH` | SQLite database path | /data/goimagefinder.db |
| `GOIMAGEFINDER_BROWSE_ROOTS` | Comma-separated paths to browse | /photos |
| `GOIMAGEFINDER_THRESHOLD` | Similarity threshold (0.0-1.0) | 0.75 |
| `GOIMAGEFINDER_OPEN_BROWSER` | Auto-open browser on start | false |

#### Docker Commands Reference

```bash
# Pull latest image
docker pull ab22375/goimagefinder

# Stop running container
docker stop goimagefinder

# Remove container
docker rm goimagefinder

# View logs
docker logs goimagefinder

# Custom port
docker run -d -p 9000:8012 \
  -v /path/to/photos:/photos \
  -v goimagefinder_data:/data \
  ab22375/goimagefinder
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
| `--json` | Output results in JSON format |
| `--progress` | Stream progress updates as JSON lines (use with --json) |
| `--debug` | Enable debug logging |
| `--logfile=PATH` | Custom log file path |

Example:
```bash
goimagefinder scan --folder=/Users/photos --db=photos.db --prefix=MacBook --debug

# JSON output for scripting
goimagefinder scan --folder=/Users/photos --json --progress
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
| `--prefix=NAME` | Filter results by source prefix (supports comma-separated list) |
| `--limit=N` | Maximum number of results to return (default: 50) |
| `--json` | Output results in JSON format |
| `--debug` | Enable debug logging |
| `--logfile=PATH` | Custom log file path |

Example:
```bash
# Single prefix
goimagefinder search --image=vacation.jpg --db=photos.db --threshold=0.75 --prefix=MacBook

# Multiple prefixes (comma-separated)
goimagefinder search --image=vacation.jpg --db=photos.db --prefix=MacBook,iPhone,Camera

# JSON output for scripting
goimagefinder search --image=vacation.jpg --json --limit=20
```

### Database Info
Display database statistics:

```bash
goimagefinder info [options]
```

| Option | Description |
|--------|-------------|
| `--database=PATH` | Database file path |
| `--json` | Output results in JSON format |

Example:
```bash
goimagefinder info --db=photos.db

# JSON output
goimagefinder info --json
```

### JSON Output Mode

All commands support `--json` flag for programmatic integration:

```bash
# Scan with streaming progress
goimagefinder scan --folder=/photos --json --progress
# Output: {"type":"progress","processed":10,"total":100,...}
# Output: {"type":"complete","success":true,"processed":100,...}

# Search returns JSON result
goimagefinder search --image=query.jpg --json
# Output: {"success":true,"query":"query.jpg","matches":[...],"total":5}

# Info returns database stats
goimagefinder info --json
# Output: {"success":true,"total_images":5000,"database_size_bytes":12582912}
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
  "openBrowser": true,
  "browseRoots": ["/photos", "/external"]
}
```

| Setting | Description | Default |
|---------|-------------|---------|
| `port` | Web server port | 8012 |
| `databasePath` | Default database file | `~/goimagefinder.db` |
| `threshold` | Similarity threshold (0.0-1.0) | 0.75 |
| `openBrowser` | Auto-open browser on start | true |
| `browseRoots` | List of paths available in file browser | `[home directory]` |

**Note:** Environment variables override config file values. See Docker section for available environment variables.

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
- **Export results**: Download search results as a text file

**Application Control**
- **Quit button**: Gracefully shut down the application from the web UI

### Workflow
1. Open `http://localhost:8012`
2. Select or create a database using "..." button
3. Select images folder using "..." button
4. (Optional) Set source prefix
5. Click "Scan" to index images
6. Select one or more query images and click "Search"
7. (Optional) Click "Export" to save results to a text file
8. Click "Quit" button (top-right) to shut down when done

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
| `/api/roots` | GET | List available browse root paths |
| `/api/quit` | POST | Shut down the application |

## Project Structure

The project is organized into two main components that can be developed independently:

### CLI Tool (goimagefinder)
```
goimagefinder/
├── main.go                 # CLI entry point
├── database/               # SQLite operations
├── imageprocessor/         # Image loading and hashing
│   ├── formats.go          # Format detection
│   ├── loaders.go          # Loader interfaces
│   └── format_*.go         # Format-specific loaders
├── scanner/                # Directory scanning
│   └── processor/          # Scanner adapter
├── output/                 # JSON output formatting
├── logging/                # Debug and error logging
├── types/                  # Common type definitions
├── utils/                  # CLI argument parsing
├── signalhandler/          # Signal handling
├── resources/              # App resources
└── Makefile                # Build scripts
```

## Build Commands

```bash
# Building
make build                  # Build for current platform
make build-macos-arm64      # Build for Apple Silicon

# macOS Distribution
make create-dmg             # Create goimagefinder.dmg
make package-macos          # Create .app bundle

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
