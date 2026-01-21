# Changelog

All notable changes to GoImageFinder will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **JSON output mode**: All CLI commands now support `--json` flag for programmatic integration.
  - Scan: `goimagefinder scan --folder=/path --json`
  - Search: `goimagefinder search --image=query.jpg --json`
  - Info: `goimagefinder info --json`
  - Structured JSON output for easy parsing by scripts and other tools

- **Progress streaming**: The scan command supports `--progress` flag for real-time progress updates.
  - Use with `--json` for machine-readable progress: `--json --progress`
  - Outputs JSON lines with processed count, total, and current file

- **Info command**: New `goimagefinder info` command to display database statistics.
  - Shows total images, unique hashes, database size
  - Supports `--json` flag for programmatic access
  - Useful for monitoring and integration

- **Result limit**: Search command now supports `--limit=N` to control the number of results returned.
  - Default: 50 results
  - Example: `goimagefinder search --image=query.jpg --limit=20`

- **Project separation preparation**: Added `webinterface/` directory for independent web interface development.
  - Web interface can now be built without CGO dependencies
  - `cli_executor.go` provides CLI wrapper for web integration
  - Separate `go.mod` and `Makefile` for web interface
  - See `SEPARATION_PLAN.md` for full migration plan

- **Output package**: New `output/` package for JSON formatting utilities.
  - Consistent JSON structures for all CLI output
  - Thread-safe JSON writer for streaming output

- **Multiple prefix filtering**: The `--prefix` option now accepts comma-separated values to filter search results by multiple source prefixes at once.
  - Example: `--prefix=MacBook,iPhone,Camera`
  - Uses efficient SQL `IN` clause for database-level filtering
  - Whitespace around prefixes is automatically trimmed

- **Environment variable configuration for Docker**: All settings can now be configured via environment variables, making Docker deployment much more flexible.
  - `GOIMAGEFINDER_PORT` - HTTP server port
  - `GOIMAGEFINDER_DATABASE_PATH` - SQLite database path
  - `GOIMAGEFINDER_BROWSE_ROOTS` - Comma-separated list of browsable paths
  - `GOIMAGEFINDER_THRESHOLD` - Similarity threshold (0.0-1.0)
  - `GOIMAGEFINDER_OPEN_BROWSER` - Auto-open browser on start

- **Multiple browse roots**: Users can now configure multiple photo folder locations that appear in the web UI file browser.
  - New `browseRoots` config field (array of paths)
  - New `/api/roots` endpoint to list available browse locations
  - Enables scanning photos from multiple drives/locations without restarting the container

- **Improved Docker documentation**: README now includes step-by-step instructions for:
  - Quick start with single photo folder
  - Multiple photo folders setup
  - Docker Compose configuration
  - Environment variables reference
  - Common Docker commands

- **Quit button**: Added a "Quit" button in the web UI header to gracefully shut down the application.
  - Displays confirmation dialog before quitting
  - Shows shutdown message after the server stops
  - New `/api/quit` endpoint (POST) for programmatic shutdown

- **Export results**: Added an "Export" button to download search results as a text file.
  - Appears after a successful search with matches
  - Exports to `goimagefinder-results-TIMESTAMP.txt`
  - Includes timestamp, threshold, scores, and file paths
  - Supports both single and batch search results

### Changed

- **Project structure**: Introduced `webinterface/` directory for future independent web interface development.
  - Web interface files copied to `webinterface/` with separate `go.mod`
  - Original `cmd/webserver/` preserved for backward compatibility
  - Web interface will eventually use CLI via `cli_executor.go` instead of direct Go imports
  - This enables the web interface to be built without CGO (no OpenCV, no SQLite C bindings)

- **CLI help text**: Updated to show new commands and flags (`info`, `--json`, `--progress`, `--limit`).

- `browseRoot` (singular) is now deprecated in favor of `browseRoots` (array). The old field still works for backward compatibility.
- Docker Compose now uses the published image `ab22375/goimagefinder` by default instead of building locally.
- Startup logs now show configured database path and browse roots for easier debugging.

### Fixed

- File browser now correctly uses the first configured browse root when multiple are available.
- **Invalid port handling**: Server now defaults to port 8012 if config has port 0 or negative value.
- **macOS launch instructions**: Updated README with correct steps for opening unsigned apps on modern macOS (System Settings → Privacy & Security → Open Anyway).

---

## How to Upgrade

### Docker Users

1. Pull the latest image:
   ```bash
   docker pull ab22375/goimagefinder
   ```

2. Stop and remove old container:
   ```bash
   docker stop goimagefinder
   docker rm goimagefinder
   ```

3. Run with new environment variables (optional):
   ```bash
   docker run -d -p 8012:8012 \
     -v /path/to/photos:/photos \
     -v /path/to/external:/external \
     -v goimagefinder_data:/data \
     -e GOIMAGEFINDER_BROWSE_ROOTS="/photos,/external" \
     ab22375/goimagefinder
   ```

### From Source

1. Pull latest code:
   ```bash
   git pull
   ```

2. Rebuild:
   ```bash
   make build-webserver
   ```

3. (Optional) Update config file `~/.goimagefinder/webserver.json` to use `browseRoots` array instead of `browseRoot` string.
