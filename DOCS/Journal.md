# GoImageFinder Development Journal

This file records improvements and changes made to the project over time.

---

## 2026-01 - Docker & macOS App Distribution (v2.3)

### New Features
- **Docker support**: Full containerized deployment with docker-compose
- **macOS App bundle**: Native .app with auto-browser launch
- **Configurable port**: Server port now configurable via config file
- **Auto-browser launch**: Opens browser automatically on startup (configurable)

### Docker Implementation

**Dockerfile**
- Multi-stage build for minimal image size
- Debian bookworm base with OpenCV runtime libraries
- Includes dcraw and exiftool for RAW processing
- Health check endpoint for container orchestration
- Volumes for persistent data and photo mounting

**docker-compose.yml**
- Named volume for database persistence
- Environment variable for custom port (`PORT=9000`)
- Read-only photo mounts for security
- Automatic restart policy

### macOS App Bundle

**Launcher Script (`resources/launcher.sh`)**
- Reads port from config file
- Starts webserver in background
- Waits for server readiness (up to 30s)
- Opens default browser to correct URL
- Maintains process for Dock presence
- Logs to `~/.goimagefinder/logs/webserver.log`

**Info.plist Configuration**
- Bundle identifier: `com.goimagefinder.webserver`
- Executes launcher.sh instead of binary directly
- NSHighResolutionCapable for Retina displays

### Configuration Updates

**New config fields in `~/.goimagefinder/webserver.json`:**
```json
{
  "port": 8012,
  "openBrowser": true
}
```

### New Makefile Targets
- `package-webserver-macos` - Creates GoImageFinder.app (web interface)
- `create-webserver-dmg` - Creates GoImageFinder.dmg (web interface with auto-browser)
- `package-macos` - Creates goimagefinder.app (CLI only)
- `create-dmg` - Creates goimagefinder.dmg (CLI only)
- `docker-build` - Builds Docker image
- `docker-run` - Runs Docker container
- `docker-compose-up` - Starts with docker-compose
- `docker-compose-down` - Stops docker-compose services

### Two DMG Types
| Command | Output | Contains |
|---------|--------|----------|
| `make create-webserver-dmg` | GoImageFinder.dmg | Web server + launcher (double-click to start) |
| `make create-dmg` | goimagefinder.dmg | CLI tool only (terminal use) |

### macOS Gatekeeper Note
Since the app is not signed with an Apple Developer certificate, users must:
1. Right-click the app → Select "Open"
2. Click "Open" in the security dialog
This only needs to be done once per installation.

### Files Added
- `Dockerfile`
- `docker-compose.yml`
- `.dockerignore`
- `resources/launcher.sh`

### Files Modified
- `cmd/webserver/config.go` - Added Port and OpenBrowser fields
- `cmd/webserver/main.go` - Added openBrowser function, config-based port
- `Makefile` - Added Docker and webserver packaging targets
- `README.md` - Updated installation documentation

---

## 2026-01 - Batch Image Search (v2.2)

### New Features
- **Multi-image search**: Search for up to 20 images in a single request
- **Grouped results display**: Results organized by query image with collapsible sections
- **Batch summary statistics**: Shows total matches, successes, and failures at a glance
- **Multi-select file picker**: Select multiple images with preview grid and remove buttons
- **Per-image error handling**: Failed images don't block successful searches

### Technical Implementation

**Backend (`cmd/webserver/main.go`)**
- Added `BatchSearchResult` type for grouped response structure
- New `/api/batch-search` endpoint accepting multipart form with multiple images
- Single database connection reused for all queries in batch
- Enforces 20-image limit and 50MB max upload size

**Frontend (`cmd/webserver/static/`)**
- Multi-file input with `multiple` attribute
- Preview grid with hover-to-remove functionality
- Dynamic search button text showing selected count
- Collapsible result groups with query thumbnails
- Summary bar with success/error/empty counts

### Files Changed
- `cmd/webserver/main.go` - Added batch search handler and types
- `cmd/webserver/templates/index.html` - Multi-file input and preview grid
- `cmd/webserver/static/script.js` - Batch selection and grouped results display
- `cmd/webserver/static/style.css` - Styles for preview grid and result groups
- `cmd/webserver/batch_search_test.go` - 10 unit and integration tests

### Test Coverage
- Request/response structure validation
- Empty request handling
- Maximum image limit enforcement
- Partial failure scenarios
- Database integration
- Handler integration test

---

## 2025-06 - Web Interface Enhancement (v2.1)

### New Features
- Enhanced web interface with file browser for database and folder selection
- Added persistent configuration management (`~/.goimagefinder/webserver.json`)
- Implemented real-time database record count display
- Added copy-to-clipboard functionality for search results
- Improved thumbnail generation for all image formats including RAW
- Added source prefix and force rewrite options to web interface

---

## 2025-06 - Architecture Refactoring

### Problem Addressed
Duplication between root-level `imageprocessor` package and `scanner/imageprocessor` package:
- Two separate packages with overlapping functionality
- Inconsistent interfaces for image loading
- Scattered format detection logic
- Confusing dependency relationships

### Solution Implemented

**1. Centralized Core Functionality**
- Created `formats.go` in root imageprocessor with common format detection
- Standardized `ImageLoader` interface in `loaders.go`
- Consolidated format-specific loaders

**2. Clean Adapter Pattern**
- Renamed `scanner/imageprocessor` to `scanner/processor`
- Implemented `processor.go` as a lightweight adapter
- Removed duplicate format detection code

**3. Improved Dependency Direction**
- All format detection now flows from root imageprocessor
- Scanner code depends on core imageprocessor, not vice versa
- Reduced code duplication significantly

### Files Changed
Added:
- `imageprocessor/formats.go`
- `imageprocessor/loaders.go`
- `imageprocessor/standard_loaders.go`
- `scanner/processor/processor.go`

Modified:
- `scanner/scanner.go`
- `scanner/fileutils.go`
- `imageprocessor/image_loader_registry.go`

### Benefits
- Single source of truth for format detection
- Clearer separation between core and scanner-specific code
- Easier to add new image formats
- Simplified extension points

---

## 2025 - Major Version 2.0

### New Features
- Added web-based graphical user interface
- Full CR3 format support with optimized processing
- Enhanced RAW image processing with multiple conversion strategies
- Comprehensive debug logging system
- Improved similarity scoring with filename boost feature

### Improvements
- Optimized memory usage for large-scale image processing
- Better error handling and fallback mechanisms for RAW formats

---

## Future Improvements

- Further consolidation of specialized format handlers
- Enhanced error handling and logging in core imageprocessor
- Progress streaming for batch search (SSE)
- Drag-and-drop support for batch image selection
- Export search results to CSV/JSON
- Code signing for macOS distribution (notarization)
- Linux AppImage or Flatpak packaging
- Windows installer (MSI/NSIS)
- Kubernetes Helm chart for enterprise deployment
