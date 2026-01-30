# Architecture Comparison: GoImageFinder vs Swift ImageMatch

## Executive Summary

This document compares the Go GoImageFinder CLI tool with the Swift ImageMatch GUI application, analyzing what architectural changes would be needed in the Go program to support the more sophisticated database and configuration model used by Swift ImageMatch.

**Key Finding**: The Swift ImageMatch app uses a "Disk-centric" architecture where multiple storage locations (disks/folders) are managed independently with rich metadata, scan jobs, and configuration per disk. The Go CLI uses a simpler "single database, prefix-based" approach.

---

## Current Go Implementation Overview

### Data Model
```
Single SQLite Database
├── images table
│   ├── id, path, source_prefix
│   ├── format, width, height
│   ├── created_at, modified_at, size
│   ├── average_hash, perceptual_hash
│   └── features (BLOB)
└── Indexes on path, source_prefix, hashes
```

### Configuration Approach
- **No persistent config**: All parameters passed via CLI flags
- **Settings per invocation**: `--database`, `--prefix`, `--threshold`, etc.
- **Simple string-based prefix**: Used to group images by source
- **No state management**: No tracking of scan jobs, progress, or disk status

### CLI Interface
```bash
# Scan - one-shot operation
goimagefinder scan --folder=/path --database=db.db --prefix=Disk1

# Search - immediate results
goimagefinder search --image=query.jpg --database=db.db --threshold=0.8

# Info - database statistics
goimagefinder info --database=db.db
```

---

## Swift ImageMatch Architecture

### Data Model (Multi-Table Relational)

```
ImageMatch.sqlite Database
├── disks table
│   ├── id (UUID), name, path
│   ├── diskDescription
│   ├── excludePatterns (JSON array)
│   ├── includeHiddenFiles (bool)
│   ├── status (idle/scanning/error)
│   ├── lastScanAt, lastScanDurationSeconds
│   ├── totalImages, totalSizeBytes
│   └── createdAt, updatedAt
│
├── scanned_images table
│   ├── id (UUID), diskId (FK)
│   ├── path, relativePath, filename
│   ├── format (enum: jpeg/png/raw/etc)
│   ├── width, height, fileSize
│   ├── fileModifiedAt
│   ├── averageHash, perceptualHash
│   ├── thumbnailPath, thumbnailGenerated
│   ├── processingStatus (pending/processed/error)
│   ├── processingError
│   └── createdAt, updatedAt
│
└── scan_jobs table
    ├── id (UUID), diskId (FK)
    ├── status (queued/running/completed/failed/cancelled)
    ├── progress (0-100)
    ├── currentFile
    ├── totalFiles, processedFiles
    ├── skippedFiles, errorFiles, newImages
    ├── startedAt, completedAt
    ├── errorMessage
    └── createdAt
```

### Configuration Approach

**Disk-Level Configuration** (per storage location):
- `excludePatterns`: Array of glob patterns to skip
- `includeHiddenFiles`: Whether to scan hidden files
- `name`, `description`: Human-readable identification

**Application-Level Settings** (UserDefaults):
```swift
showThumbnails: Bool              // Display image thumbnails
textOnlySearchResults: Bool       // Don't load actual images
loggingEnabled: Bool              // Enable scan logging
logsFolderPath: String?           // Where to save logs
scanConcurrency: Int              // Parallel workers (default: CPU count)
hashingImageSize: Int             // Max dimension for hashing (256-1024px)
```

**Runtime Search Configuration**:
```swift
SearchQuery {
    queryImagePath: String
    threshold: Double (0.0-1.0)
    diskIds: Set<UUID>?           // nil = search all disks
    limit: Int (default: 100)
}
```

---

## Required Changes for Go Implementation

### 1. Database Schema Migration

#### New Tables Needed

**disks table** - Manage multiple storage locations:
```sql
CREATE TABLE disks (
    id BLOB PRIMARY KEY,              -- UUID (16 bytes)
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,        -- Absolute path to disk/folder
    description TEXT,
    exclude_patterns TEXT,            -- JSON array of patterns
    include_hidden_files BOOLEAN DEFAULT 0,
    status TEXT DEFAULT 'idle',       -- idle, scanning, error
    last_scan_at DATETIME,
    last_scan_duration_seconds INTEGER,
    total_images INTEGER DEFAULT 0,
    total_size_bytes INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL
);

CREATE INDEX idx_disks_path ON disks(path);
CREATE INDEX idx_disks_status ON disks(status);
```

**scan_jobs table** - Track scan operations:
```sql
CREATE TABLE scan_jobs (
    id BLOB PRIMARY KEY,              -- UUID
    disk_id BLOB NOT NULL,
    status TEXT DEFAULT 'queued',     -- queued, running, completed, failed, cancelled
    progress INTEGER DEFAULT 0,       -- 0-100
    current_file TEXT,
    total_files INTEGER DEFAULT 0,
    processed_files INTEGER DEFAULT 0,
    skipped_files INTEGER DEFAULT 0,
    error_files INTEGER DEFAULT 0,
    new_images INTEGER DEFAULT 0,
    started_at DATETIME,
    completed_at DATETIME,
    error_message TEXT,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (disk_id) REFERENCES disks(id) ON DELETE CASCADE
);

CREATE INDEX idx_scan_jobs_disk_id ON scan_jobs(disk_id);
CREATE INDEX idx_scan_jobs_status ON scan_jobs(status);
CREATE INDEX idx_scan_jobs_created_at ON scan_jobs(created_at DESC);
```

**Modified images table** - Add disk relationship:
```sql
-- Option A: Rename and extend existing table
CREATE TABLE scanned_images (
    id BLOB PRIMARY KEY,              -- UUID instead of INTEGER
    disk_id BLOB NOT NULL,            -- NEW: Foreign key to disks
    path TEXT NOT NULL,
    relative_path TEXT NOT NULL,      -- NEW: Path relative to disk root
    filename TEXT NOT NULL,           -- NEW: Just the filename
    format TEXT NOT NULL,             -- enum values
    width INTEGER,
    height INTEGER,
    file_size INTEGER NOT NULL,
    file_modified_at DATETIME,        -- File system modification time
    average_hash TEXT,
    perceptual_hash TEXT,
    thumbnail_path TEXT,              -- NEW: Path to generated thumbnail
    thumbnail_generated BOOLEAN DEFAULT 0,
    processing_status TEXT DEFAULT 'pending', -- pending, processed, error
    processing_error TEXT,
    created_at DATETIME NOT NULL,     -- When record was created
    updated_at DATETIME NOT NULL,     -- When record was last updated
    FOREIGN KEY (disk_id) REFERENCES disks(id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX idx_scanned_images_disk_path ON scanned_images(disk_id, path);
CREATE INDEX idx_scanned_images_average_hash ON scanned_images(average_hash);
CREATE INDEX idx_scanned_images_perceptual_hash ON scanned_images(perceptual_hash);
CREATE INDEX idx_scanned_images_processing_status ON scanned_images(processing_status);
```

**Compatibility view** (for backward compatibility):
```sql
-- View that maps new schema to old CLI expectations
CREATE VIEW images AS
SELECT
    id,
    path,
    lower(hex(disk_id)) as source_prefix,  -- Convert UUID to hex string
    format,
    width,
    height,
    created_at,
    file_modified_at as modified_at,
    file_size as size,
    average_hash,
    perceptual_hash
FROM scanned_images
WHERE processing_status = 'processed';
```

### 2. New Configuration Management System

**Configuration Sources** (in order of precedence):
1. CLI flags (highest priority)
2. Environment variables
3. Configuration file
4. Default values (lowest priority)

**Configuration File Structure** (YAML/JSON/TOML):
```yaml
# ~/.goimagefinder/config.yaml

# Application settings
app:
  database_path: ~/.goimagefinder/imagematch.db
  log_level: info
  
# Performance settings
performance:
  scan_concurrency: 8              # Number of parallel workers
  hashing_image_size: 512          # Max dimension for hash computation
  thumbnail_size: 256              # Thumbnail dimensions
  
# Display settings
display:
  show_thumbnails: true
  text_only_results: false
  
# Logging settings
logging:
  enabled: false
  logs_folder: ~/.goimagefinder/logs
  retention_days: 30

# Disk definitions (managed via CLI or API)
disks: []  # Populated via 'disk add' command
```

**Environment Variables**:
```bash
GOIMAGEFINDER_DATABASE_PATH
GOIMAGEFINDER_LOG_LEVEL
GOIMAGEFINDER_SCAN_CONCURRENCY
GOIMAGEFINDER_HASHING_IMAGE_SIZE
GOIMAGEFINDER_LOGGING_ENABLED
GOIMAGEFINDER_LOGS_FOLDER
```

### 3. New CLI Commands

**Disk Management**:
```bash
# Add a new disk/folder
goimagefinder disk add --name="External HDD" --path=/Volumes/Photos \
  --exclude="*.tmp,*.cache" --exclude="**/thumbnails/**"

# List all disks
goimagefinder disk list
# Output: ID | Name | Path | Status | Total Images | Last Scan

# Update disk settings
goimagefinder disk update <disk-id> --name="New Name" --exclude="*.bak"

# Remove a disk (and optionally delete all its images)
goimagefinder disk remove <disk-id> [--delete-images]

# Show disk details and scan history
goimagefinder disk show <disk-id>
```

**Scan Job Management**:
```bash
# Start a scan (creates a job, returns job ID)
goimagefinder scan start --disk=<disk-id> [--force]
# Output: {"job_id": "uuid", "status": "queued"}

# List scan jobs
goimagefinder scan list [--disk=<disk-id>] [--status=running]

# Show scan progress
goimagefinder scan status <job-id>
# Output: {
#   "job_id": "uuid",
#   "status": "running",
#   "progress": 45,
#   "current_file": "/path/to/current.jpg",
#   "processed": 450,
#   "total": 1000,
#   "errors": 2
# }

# Cancel a running scan
goimagefinder scan cancel <job-id>

# View scan history/logs
goimagefinder scan logs <job-id>
```

**Enhanced Search**:
```bash
# Search with disk filtering
goimagefinder search --image=query.jpg --disks=disk1-uuid,disk2-uuid

# Search with all configurable parameters
goimagefinder search \
  --image=query.jpg \
  --threshold=0.85 \
  --disks=disk1-uuid \
  --limit=50 \
  --include-metadata
```

**Settings Management**:
```bash
# View current settings
goimagefinder config get

# Set a configuration value
goimagefinder config set performance.scan_concurrency 16
goimagefinder config set display.show_thumbnails false

# Reset to defaults
goimagefinder config reset
```

### 4. Internal Architecture Changes

#### Package Structure Changes

```
database/
├── database.go              # Existing: basic connection
├── migrations.go            # NEW: Database migration manager
├── disk_repository.go       # NEW: Disk CRUD operations
├── image_repository.go      # NEW: Image CRUD (extends existing)
├── scan_job_repository.go   # NEW: Scan job operations
├── search.go                # NEW: Search operations with disk filtering
└── models.go                # NEW: Database models

config/
├── config.go                # NEW: Configuration management
├── loader.go                # NEW: Load from file/env/flags
└── defaults.go              # NEW: Default values

disk/
├── manager.go               # NEW: Disk lifecycle management
├── validator.go             # NEW: Path validation
└── scanner.go               # NEW: Disk-specific scanning

scan/
├── job.go                   # NEW: Scan job definitions
├── orchestrator.go          # NEW: Job queue and execution
├── progress.go              # NEW: Progress tracking
├── worker.go                # NEW: Worker pool management
└── logger.go                # NEW: Scan logging service

models/                      # NEW: Central type definitions
├── disk.go
├── image.go
├── scan_job.go
└── search.go
```

#### Key Types (Go structs)

```go
// models/disk.go
type Disk struct {
    ID                      uuid.UUID
    Name                    string
    Path                    string
    Description             *string
    ExcludePatterns         []string
    IncludeHiddenFiles      bool
    Status                  DiskStatus
    LastScanAt              *time.Time
    LastScanDurationSeconds *int
    TotalImages             int
    TotalSizeBytes          int64
    CreatedAt               time.Time
    UpdatedAt               time.Time
}

type DiskStatus string
const (
    DiskStatusIdle     DiskStatus = "idle"
    DiskStatusScanning DiskStatus = "scanning"
    DiskStatusError    DiskStatus = "error"
)

// models/scan_job.go
type ScanJob struct {
    ID              uuid.UUID
    DiskID          uuid.UUID
    Status          ScanJobStatus
    Progress        int
    CurrentFile     *string
    TotalFiles      int
    ProcessedFiles  int
    SkippedFiles    int
    ErrorFiles      int
    NewImages       int
    StartedAt       *time.Time
    CompletedAt     *time.Time
    ErrorMessage    *string
    CreatedAt       time.Time
}

type ScanJobStatus string
const (
    ScanJobStatusQueued    ScanJobStatus = "queued"
    ScanJobStatusRunning   ScanJobStatus = "running"
    ScanJobStatusCompleted ScanJobStatus = "completed"
    ScanJobStatusFailed    ScanJobStatus = "failed"
    ScanJobStatusCancelled ScanJobStatus = "cancelled"
)

// models/image.go (extends existing)
type ScannedImage struct {
    ID                 uuid.UUID
    DiskID             uuid.UUID
    Path               string
    RelativePath       string
    Filename           string
    Format             ImageFormat
    Width              *int
    Height             *int
    FileSize           int64
    FileModifiedAt     *time.Time
    AverageHash        *string
    PerceptualHash     *string
    ThumbnailPath      *string
    ThumbnailGenerated bool
    ProcessingStatus   ProcessingStatus
    ProcessingError    *string
    CreatedAt          time.Time
    UpdatedAt          time.Time
}

// models/search.go
type SearchQuery struct {
    QueryImagePath string
    Threshold      float64
    DiskIDs        []uuid.UUID  // nil = search all
    Limit          int
}

type SearchResult struct {
    ImageID         uuid.UUID
    DiskID          uuid.UUID
    DiskName        string
    Path            string
    Similarity      float64
    Width           *int
    Height          *int
    Format          string
    FileSize        *int64
    FileModifiedAt  *time.Time
}
```

### 5. Configuration Parameter Handling

#### Parameter Passing Strategy

**Hierarchical Configuration**:
```
1. CLI flags (highest priority)
   └─ Override everything else
   
2. Environment variables
   └─ Override config file and defaults
   
3. Config file (~/.goimagefinder/config.yaml)
   └─ Override defaults
   
4. Default values
   └─ Compiled-in defaults
```

**Configuration Interface**:
```go
// config/config.go
type Config struct {
    App         AppConfig
    Performance PerformanceConfig
    Display     DisplayConfig
    Logging     LoggingConfig
}

type AppConfig struct {
    DatabasePath string `yaml:"database_path" env:"GOIMAGEFINDER_DATABASE_PATH"`
    LogLevel     string `yaml:"log_level" env:"GOIMAGEFINDER_LOG_LEVEL"`
}

type PerformanceConfig struct {
    ScanConcurrency  int `yaml:"scan_concurrency" env:"GOIMAGEFINDER_SCAN_CONCURRENCY"`
    HashingImageSize int `yaml:"hashing_image_size" env:"GOIMAGEFINDER_HASHING_IMAGE_SIZE"`
}

type DisplayConfig struct {
    ShowThumbnails     bool `yaml:"show_thumbnails"`
    TextOnlyResults    bool `yaml:"text_only_results"`
}

type LoggingConfig struct {
    Enabled       bool   `yaml:"enabled" env:"GOIMAGEFINDER_LOGGING_ENABLED"`
    LogsFolder    string `yaml:"logs_folder" env:"GOIMAGEFINDER_LOGS_FOLDER"`
    RetentionDays int    `yaml:"retention_days"`
}

// Load configuration from all sources
func Load(flags map[string]string) (*Config, error) {
    // 1. Start with defaults
    cfg := DefaultConfig()
    
    // 2. Load from config file
    if err := cfg.LoadFromFile(); err != nil {
        return nil, err
    }
    
    // 3. Override with environment variables
    cfg.LoadFromEnv()
    
    // 4. Override with CLI flags
    cfg.LoadFromFlags(flags)
    
    return cfg, nil
}
```

### 6. API/Server Mode (Optional but Recommended)

For Docker deployment and cross-platform usage, add an HTTP API mode:

```go
// server/server.go

type Server struct {
    db     *sql.DB
    config *config.Config
    scans  *scan.Orchestrator
}

func (s *Server) Routes() http.Handler {
    r := mux.NewRouter()
    
    // Disk management
    r.HandleFunc("/api/disks", s.ListDisks).Methods("GET")
    r.HandleFunc("/api/disks", s.CreateDisk).Methods("POST")
    r.HandleFunc("/api/disks/{id}", s.GetDisk).Methods("GET")
    r.HandleFunc("/api/disks/{id}", s.UpdateDisk).Methods("PUT")
    r.HandleFunc("/api/disks/{id}", s.DeleteDisk).Methods("DELETE")
    
    // Scan jobs
    r.HandleFunc("/api/scans", s.ListScanJobs).Methods("GET")
    r.HandleFunc("/api/scans", s.CreateScanJob).Methods("POST")
    r.HandleFunc("/api/scans/{id}", s.GetScanJob).Methods("GET")
    r.HandleFunc("/api/scans/{id}/cancel", s.CancelScanJob).Methods("POST")
    
    // Search
    r.HandleFunc("/api/search", s.Search).Methods("POST")
    
    // Configuration
    r.HandleFunc("/api/config", s.GetConfig).Methods("GET")
    r.HandleFunc("/api/config", s.UpdateConfig).Methods("PUT")
    
    // WebSocket for real-time scan progress
    r.HandleFunc("/ws/scans/{id}", s.ScanProgressWebSocket)
    
    return r
}
```

**Example API Requests**:
```bash
# Add a disk
curl -X POST http://localhost:8080/api/disks \
  -H "Content-Type: application/json" \
  -d '{"name": "Photos 2024", "path": "/photos/2024", "exclude_patterns": ["*.tmp"]}'

# Start a scan
curl -X POST http://localhost:8080/api/scans \
  -H "Content-Type: application/json" \
  -d '{"disk_id": "uuid-here", "force": false}'

# Search
curl -X POST http://localhost:8080/api/search \
  -H "Content-Type: application/json" \
  -d '{
    "query_image_path": "/query/image.jpg",
    "threshold": 0.85,
    "disk_ids": ["uuid-1", "uuid-2"],
    "limit": 50
  }'
```

---

## Migration Path

### Phase 1: Schema Extension (Backward Compatible)
1. Add new tables (`disks`, `scan_jobs`)
2. Add `disk_id` column to `images` table (nullable initially)
3. Create compatibility view `images` for old CLI
4. Support both old and new CLI commands

### Phase 2: Configuration System
1. Add config file support
2. Add environment variable support
3. Create `config` command group

### Phase 3: New Commands
1. Add `disk` command group
2. Add enhanced `scan` commands with job tracking
3. Enhance `search` with disk filtering

### Phase 4: Server Mode (Optional)
1. Add HTTP API
2. Add WebSocket support for progress
3. Create web UI (or port Swift UI)

---

## Database Migration Script Example

```sql
-- Migration: v2_disk_centric_schema.sql

-- 1. Create disks table
CREATE TABLE IF NOT EXISTS disks (
    id BLOB PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT NOT NULL UNIQUE,
    description TEXT,
    exclude_patterns TEXT,  -- JSON array
    include_hidden_files BOOLEAN DEFAULT 0,
    status TEXT DEFAULT 'idle',
    last_scan_at DATETIME,
    last_scan_duration_seconds INTEGER,
    total_images INTEGER DEFAULT 0,
    total_size_bytes INTEGER DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 2. Create scan_jobs table
CREATE TABLE IF NOT EXISTS scan_jobs (
    id BLOB PRIMARY KEY,
    disk_id BLOB NOT NULL REFERENCES disks(id) ON DELETE CASCADE,
    status TEXT DEFAULT 'queued',
    progress INTEGER DEFAULT 0,
    current_file TEXT,
    total_files INTEGER DEFAULT 0,
    processed_files INTEGER DEFAULT 0,
    skipped_files INTEGER DEFAULT 0,
    error_files INTEGER DEFAULT 0,
    new_images INTEGER DEFAULT 0,
    started_at DATETIME,
    completed_at DATETIME,
    error_message TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- 3. Migrate existing images to new schema
-- Create a default disk for existing data
INSERT INTO disks (id, name, path, status, total_images, created_at, updated_at)
SELECT 
    lower(hex(randomblob(16))),  -- Generate UUID
    COALESCE(source_prefix, 'Legacy'),
    '/unknown',                   -- Path unknown for legacy data
    'idle',
    COUNT(*),
    MIN(created_at),
    MAX(modified_at)
FROM images
GROUP BY source_prefix;

-- Create new scanned_images table
CREATE TABLE scanned_images (
    id BLOB PRIMARY KEY,
    disk_id BLOB NOT NULL REFERENCES disks(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    relative_path TEXT NOT NULL DEFAULT '',
    filename TEXT NOT NULL DEFAULT '',
    format TEXT,
    width INTEGER,
    height INTEGER,
    file_size INTEGER DEFAULT 0,
    file_modified_at DATETIME,
    average_hash TEXT,
    perceptual_hash TEXT,
    thumbnail_path TEXT,
    thumbnail_generated BOOLEAN DEFAULT 0,
    processing_status TEXT DEFAULT 'processed',
    processing_error TEXT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Migrate data from old images table
INSERT INTO scanned_images (
    id, disk_id, path, filename, format, width, height,
    file_size, file_modified_at, average_hash, perceptual_hash,
    processing_status, created_at
)
SELECT 
    lower(hex(randomblob(16))),
    d.id,
    i.path,
    REPLACE(i.path, rtrim(i.path, replace(i.path, '/', '')), ''),
    i.format,
    i.width,
    i.height,
    i.size,
    i.modified_at,
    i.average_hash,
    i.perceptual_hash,
    'processed',
    i.created_at
FROM images i
JOIN disks d ON d.name = COALESCE(i.source_prefix, 'Legacy');

-- Create indexes
CREATE UNIQUE INDEX idx_scanned_images_disk_path ON scanned_images(disk_id, path);
CREATE INDEX idx_scanned_images_average_hash ON scanned_images(average_hash);
CREATE INDEX idx_scanned_images_perceptual_hash ON scanned_images(perceptual_hash);
CREATE INDEX idx_scanned_images_processing_status ON scanned_images(processing_status);

-- Create compatibility view
DROP VIEW IF EXISTS images;
CREATE VIEW images AS
SELECT
    id,
    path,
    lower(hex(disk_id)) as source_prefix,
    format,
    width,
    height,
    created_at,
    file_modified_at as modified_at,
    file_size as size,
    average_hash,
    perceptual_hash
FROM scanned_images
WHERE processing_status = 'processed';

-- Keep old table as backup (optional)
-- ALTER TABLE images RENAME TO images_backup;
```

---

## Summary

To match Swift ImageMatch's architecture, the Go program would need:

1. **Multi-table database schema** with relationships between disks, images, and scan jobs
2. **UUID-based identifiers** instead of auto-increment integers
3. **Configuration management system** supporting file, env, and CLI sources
4. **Disk management commands** for CRUD operations on storage locations
5. **Scan job tracking** with persistent state and progress
6. **Enhanced search** with disk-level filtering
7. **Settings persistence** for performance and display options
8. **Optional HTTP API** for Docker/cross-platform deployment

**Estimated effort**: 4-6 weeks for full implementation, 2-3 weeks for minimal viable version (schema + basic commands).
