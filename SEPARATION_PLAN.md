# GoImageFinder Separation Plan

## Overview

**Goal:** Split the codebase into two independent projects:
1. **goimagefinder** - Standalone CLI executable for `/usr/local/bin`
2. **goimagefinder-web** - Web interface that calls the CLI behind the scenes

---

## Project 1: goimagefinder (CLI)

### Final Structure
```
goimagefinder/
├── main.go                    # CLI entry point
├── go.mod / go.sum
├── Makefile
├── CLAUDE.md
├── database/                  # SQLite operations
├── imageprocessor/            # Image loading, hashing, search
├── scanner/                   # Directory scanning
│   └── processor/
├── logging/                   # Debug logging
├── types/                     # Shared types
├── utils/                     # CLI argument parsing
├── signalhandler/             # Signal handling
├── tests/                     # Unit tests (excluding webserver)
│   ├── database/
│   ├── imageprocessor/
│   ├── integration/
│   ├── scanner/
│   └── utils/
└── resources/                 # Icons (optional)
```

### CLI Commands

**Existing:**
```bash
goimagefinder scan --folder=/path/to/images [--database=...] [--debug]
goimagefinder search --image=/path/to/image.jpg [--database=...] [--threshold=85]
```

**New (for web integration):**
```bash
# JSON output mode
goimagefinder scan --folder=/path --json --progress
goimagefinder search --image=/path --json --limit=50

# Database info
goimagefinder info [--database=...] [--json]
```

### JSON Output Formats

**Scan with progress:**
```json
{"type":"progress","processed":10,"total":100,"current":"/photos/img.jpg","status":"processing"}
{"type":"progress","processed":11,"total":100,"current":"/photos/img2.jpg","status":"success"}
{"type":"complete","processed":100,"total":100,"errors":0,"new_images":50,"skipped":50}
```

**Search results:**
```json
{
  "query": "/tmp/query.jpg",
  "matches": [
    {"path":"/photos/similar.jpg","score":95.2,"width":1920,"height":1080,"format":"jpeg"},
    {"path":"/photos/another.jpg","score":88.1,"width":1920,"height":1080,"format":"jpeg"}
  ],
  "total": 2
}
```

**Database info:**
```json
{
  "database_path": "/home/user/.goimagefinder/images.db",
  "total_images": 5000,
  "database_size_bytes": 12582912,
  "last_modified": "2025-01-20T15:30:00Z"
}
```

---

## Project 2: goimagefinder-web (Web Interface)

### Location
`webinterface/` folder (to be removed from CLI project later)

### Structure
```
webinterface/
├── main.go                    # Web server entry point
├── config.go                  # Configuration management
├── handlers.go                # HTTP request handlers
├── cli_executor.go            # Wrapper to execute goimagefinder CLI
├── go.mod / go.sum            # Separate module (minimal deps)
├── Makefile
├── Dockerfile
├── docker-compose.yml
├── static/
│   ├── script.js
│   ├── style.css
│   └── batch_search_test.js
├── templates/
│   └── index.html
├── tests/
│   └── webserver_test.go
└── resources/
    ├── AppIcon.icns
    └── launcher.sh
```

### Key Change: CLI Execution Instead of Direct Imports

**Before (direct Go imports):**
```go
import (
    "imagefinder/database"
    "imagefinder/imageprocessor"
    "imagefinder/scanner"
)

func handleScan(folder string) {
    scanner.ScanAndStoreFolder(folder, db, options)
}
```

**After (CLI execution):**
```go
func handleScan(folder string) {
    cmd := exec.Command("goimagefinder", "scan",
        "--folder="+folder,
        "--database="+dbPath,
        "--json",
        "--progress")
    // Stream stdout lines to SSE
}
```

### Dependencies

**Web go.mod (minimal):**
```go
module goimagefinder-web

go 1.24.1

// Pure stdlib - no CGO, no OpenCV, no SQLite bindings
```

---

## Implementation Phases

### Phase 1: Enhance CLI for JSON Output

1. Add `--json` flag to output machine-readable JSON
2. Add `--progress` flag for streaming progress (JSON lines)
3. Add `--limit` flag to search command
4. Add `info` subcommand for database statistics
5. Standardize exit codes:
   - 0 = success
   - 1 = error
   - 2 = not found / no results

**Files to modify:**
- `main.go` - Add new flags and info command
- `utils/args.go` - Parse new arguments
- `scanner/scanner.go` - Support progress callbacks
- NEW: `output/json.go` - JSON formatting utilities

### Phase 2: Create webinterface/ Directory

1. Create `webinterface/` directory structure
2. Move files from `cmd/webserver/`:
   - `main.go` → `webinterface/main.go`
   - `config.go` → `webinterface/config.go`
   - `static/` → `webinterface/static/`
   - `templates/` → `webinterface/templates/`
3. Move Docker files:
   - `Dockerfile` → `webinterface/Dockerfile`
   - `docker-compose.yml` → `webinterface/docker-compose.yml`
4. Move resources:
   - `resources/launcher.sh` → `webinterface/resources/launcher.sh`
5. Move tests:
   - `tests/webserver/` → `webinterface/tests/`
6. Create new files:
   - `webinterface/go.mod`
   - `webinterface/Makefile`
   - `webinterface/cli_executor.go`

### Phase 3: Refactor Web Server (Future)

1. Refactor `main.go` to use `cli_executor.go`
2. Remove all imports of `imagefinder/*` packages
3. Update handlers to parse CLI JSON output
4. Update SSE to stream CLI progress output

### Phase 4: Cleanup CLI Project (Future)

1. Remove `cmd/webserver/` directory
2. Remove web-related Makefile targets
3. Update `tests/` to exclude webserver tests
4. Update documentation

---

## Files to Delete from CLI (After Phase 2)

```
cmd/webserver/           # Entire directory (moved to webinterface/)
Dockerfile               # Moved to webinterface/
docker-compose.yml       # Moved to webinterface/
tests/webserver/         # Moved to webinterface/tests/
```

---

## Benefits

| Aspect | Before | After |
|--------|--------|-------|
| Web build | Requires CGO, OpenCV, SQLite | Pure Go, no CGO |
| CLI updates | Requires web rebuild | Independent |
| Deployment | Single monolith | CLI on server, Web in container |
| Testing | Coupled | Independent test suites |
| Web dependencies | Heavy (gocv, sqlite3) | Lightweight (stdlib only) |
| Cross-compilation | Complex (CGO) | Web: trivial, CLI: same as before |

---

## Configuration

### CLI Configuration
- Database path: `--database` flag or `~/.goimagefinder/images.db`
- Debug logging: `--debug` flag

### Web Configuration (`~/.goimagefinder/webserver.json`)
```json
{
  "port": 8012,
  "cli_binary_path": "/usr/local/bin/goimagefinder",
  "database_path": "~/.goimagefinder/images.db",
  "browse_roots": ["/Users", "/Volumes"],
  "default_threshold": 85
}
```

---

## Migration Checklist

### Phase 1: CLI JSON Output ✅ COMPLETED
- [x] Add --json flag parsing
- [x] Add --progress flag parsing
- [x] Add --limit flag to search
- [x] Add info subcommand
- [x] Implement JSON output for scan
- [x] Implement JSON progress streaming (basic)
- [x] Implement JSON output for search
- [x] Implement JSON output for info
- [x] Standardize exit codes
- [x] Update help text

### Phase 2: Create webinterface/ ✅ COMPLETED
- [x] Create webinterface/ directory
- [x] Create webinterface/go.mod
- [x] Create webinterface/Makefile
- [x] Move cmd/webserver/main.go
- [x] Move cmd/webserver/config.go
- [x] Move static/ directory
- [x] Move templates/ directory
- [x] Move Dockerfile
- [x] Move docker-compose.yml
- [x] Move resources/launcher.sh
- [x] Move tests/webserver/
- [x] Create cli_executor.go stub

### Phase 3: Refactor Web (Future - Do This Separately)
- [ ] Implement cli_executor.go fully
- [ ] Refactor handlers to use CLI instead of direct imports
- [ ] Remove imagefinder/* imports from webinterface/main.go
- [ ] Update SSE for CLI streaming
- [ ] Test all endpoints
- [ ] Update webinterface/config.go to add cli_binary_path setting

### Phase 4: Cleanup CLI Project (Future - After Phase 3)
- [ ] Delete cmd/webserver/ directory
- [ ] Delete Dockerfile from CLI root (keep in webinterface/)
- [ ] Delete docker-compose.yml from CLI root
- [ ] Remove webserver-related targets from CLI Makefile
- [ ] Move tests/webserver/ to webinterface/ (already copied)
- [ ] Update CLAUDE.md documentation
