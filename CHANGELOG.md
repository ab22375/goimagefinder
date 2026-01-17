# Changelog

All notable changes to GoImageFinder will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

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

### Changed

- `browseRoot` (singular) is now deprecated in favor of `browseRoots` (array). The old field still works for backward compatibility.
- Docker Compose now uses the published image `ab22375/goimagefinder` by default instead of building locally.
- Startup logs now show configured database path and browse roots for easier debugging.

### Fixed

- File browser now correctly uses the first configured browse root when multiple are available.

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
