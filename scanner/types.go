package scanner

import (
	"sync"
	"time"
)

// ScanOptions defines the options for scanning
type ScanOptions struct {
	FolderPath   string
	SourcePrefix string
	ForceRewrite bool
	DebugMode    bool
	DbPath       string
	LogPath      string
	TotalImages  int // Optional pre-counted total
	MaxWorkers   int // Optional worker limit
}

// ProcessImageResult holds the result of processing an image
type ProcessImageResult struct {
	Path    string
	Success bool
	Error   error
	IsRaw   bool
	IsTif   bool
}

// FileStats tracks information about files to be processed
type FileStats struct {
	totalFiles int
	rawFiles   int
	tifFiles   int
}

// ImageHashes contains computed hashes for an image
type ImageHashes struct {
	avgHash string
	pHash   string
}

// ProgressTracker tracks progress of the scan operation
type ProgressTracker struct {
	processed    int
	errors       int
	rawProcessed int
	rawErrors    int
	tifProcessed int
	tifErrors    int
	ticker       *time.Ticker
	done         chan bool
	mu           sync.Mutex
	totalFiles   int
	rawFiles     int
	tifFiles     int
}
