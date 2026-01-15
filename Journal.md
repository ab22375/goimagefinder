# GoImageFinder Development Journal

This file records improvements and changes made to the project over time.

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
