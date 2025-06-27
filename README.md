# GoImageFinder

A Go-based image similarity detection and management system that indexes images using perceptual hashing and provides both command-line and web interfaces for searching duplicate or similar images.

## Technologies

- **Go 1.18+** - Main programming language
- **SQLite3** - Database for storing image metadata and hashes
- **GoCV** - OpenCV bindings for image processing
- **go-exiftool** - EXIF metadata extraction from RAW files
- **HTML/CSS/JavaScript** - Web interface frontend

## Dependencies

### Go Packages
```bash
go get github.com/mattn/go-sqlite3
go get gocv.io/x/gocv
go get github.com/barasher/go-exiftool
go get golang.org/x/image
```

### External Tools
- **dcraw** - RAW image conversion
- **exiftool** - Metadata extraction
- **libraw** - RAW image processing library
- **rawtherapee-cli** (optional) - Alternative RAW processor

## Installation

### From Source
```bash
git clone https://github.com/yourusername/goimagefinder
cd goimagefinder
make build
```

### macOS DMG
Download the DMG from releases and install. Then create a command-line symlink:
```bash
sudo ln -sf /Applications/goimagefinder.app/Contents/MacOS/goimagefinder /usr/local/bin/goimagefinder
```

## Command Line Usage

### Scan Images
Index a directory of images into a SQLite database:

```bash
goimagefinder scan --folder=/path/to/images [options]
```

Options:
- `--database=PATH` or `--db=PATH` - Database file path (default: ./images.db)
- `--prefix=NAME` - Source prefix for organizing images from different sources
- `--force` - Force rewrite of existing entries
- `--debug` - Enable debug logging
- `--logfile=PATH` - Custom log file path (default: imagefinder.log)

Example:
```bash
goimagefinder scan --folder=/Users/photos --db=photos.db --prefix=MacBook --debug
```

### Search Similar Images
Find images similar to a query image:

```bash
goimagefinder search --image=/path/to/query.jpg [options]
```

Options:
- `--database=PATH` or `--db=PATH` - Database file path
- `--threshold=VALUE` - Similarity threshold 0.0-1.0 (default: 0.8)
- `--prefix=NAME` - Filter results by source prefix
- `--debug` - Enable debug logging
- `--logfile=PATH` - Custom log file path

Example:
```bash
goimagefinder search --image=vacation.jpg --db=photos.db --threshold=0.75
```

## Web Interface Usage

### Start Web Server
```bash
goimagefinder webserver [port]
```

Or run directly:
```bash
go run cmd/webserver/main.go cmd/webserver/config.go [port]
```

Default port is 8012. Access at `http://localhost:8012`

### Features

- **Configuration**: Settings persist in `~/.goimagefinder/webserver.json`
- **Database Management**: File browser for database selection, real-time record count
- **Image Scanning**: Visual folder selection, progress tracking, scan options
- **Similarity Search**: Drag-and-drop query images, adjustable threshold, thumbnail previews
- **Results**: Clickable thumbnails and paths, copy-to-clipboard, similarity scores

### Web Interface Example Workflow

1. Open browser to `http://localhost:8012`
2. Click "..." next to Database path to select or create a database
3. Click "..." next to folder path to select images directory
4. Optional: Set source prefix (e.g., "ExternalDrive1")
5. Click "Scan" to index images
6. Select an image file and click "Search" to find similar images

## Project Structure

```
goimagefinder/
├── cmd/webserver/        # Web interface application
│   ├── main.go          # Web server entry point
│   ├── config.go        # Configuration management
│   ├── static/          # JavaScript and CSS
│   └── templates/       # HTML templates
├── database/            # SQLite database operations
├── imageprocessor/      # Image loading and hashing
│   ├── image_processing.go
│   ├── thumbnail.go
│   └── format_*.go      # Format-specific loaders
├── scanner/             # Directory scanning and indexing
├── logging/             # Debug and error logging
├── types/               # Common type definitions
├── utils/               # Utility functions
├── main.go              # CLI entry point
└── Makefile            # Build scripts
```

## Supported Image Formats

- **Standard**: JPG, JPEG, PNG, GIF, BMP, TIFF, WEBP
- **RAW**: CR2, CR3, NEF, ARW, DNG, ORF, RW2, RAF, SRW
- **HEIC**: Apple HEIC format

## Image Similarity Algorithm

The system uses two perceptual hashing algorithms:

1. **Average Hash (aHash)**: 8x8 grayscale representation comparing pixels to mean brightness
2. **Perceptual Hash (pHash)**: 32x32 DCT-based transformation with median filtering

Similarity score calculation:
- Hamming distance between hashes
- Filename similarity boost for related files (e.g., IMG_1234.JPG and IMG_1234.CR2)
- Weighted combination of aHash and pHash scores

## Database Schema

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

CREATE INDEX idx_path ON images(path);
CREATE INDEX idx_average_hash ON images(average_hash);
CREATE INDEX idx_perceptual_hash ON images(perceptual_hash);
```

## Build Commands

```bash
make build              # Build for current platform
make build-macos-arm64  # Build for Apple Silicon
make webserver          # Build and run web interface
make clean              # Remove build artifacts
make package-macos      # Create macOS .app bundle
make create-dmg         # Create distributable DMG
make test               # Run tests
```

## Debug Mode

Enable debug logging with `--debug` flag. Log includes:
- Image processing details
- Hash computation values
- Error diagnostics
- Processing statistics
- Search match details

Example debug output:
```
2025/06/27 10:15:23 Starting scan on folder: /Users/photos
2025/06/27 10:15:24 Found 1283 image files to process
2025/06/27 10:15:25 PROCESSED: /Users/photos/img001.jpg
2025/06/27 10:15:26 Hash values - aHash: 3e7a73fff6fe0604, pHash: 84840b9f3efed512
```

## Performance Notes

- Concurrent processing with configurable worker threads
- Incremental scanning skips unchanged files
- SQLite indexes optimize search queries
- RAW preview extraction for faster processing
- Thumbnail caching for web interface

## License

MIT License

## Contributing

1. Fork the repository
2. Create feature branch (`git checkout -b feature/new-feature`)
3. Commit changes (`git commit -m 'Add new feature'`)
4. Push branch (`git push origin feature/new-feature`)
5. Open Pull Request