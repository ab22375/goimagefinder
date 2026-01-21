package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// JSONWriter handles JSON output to stdout with thread-safety
type JSONWriter struct {
	mu      sync.Mutex
	encoder *json.Encoder
}

// NewJSONWriter creates a new JSON writer for stdout
func NewJSONWriter() *JSONWriter {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	return &JSONWriter{encoder: encoder}
}

// Write outputs a JSON object as a single line
func (w *JSONWriter) Write(v interface{}) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.encoder.Encode(v)
}

// --- Scan Progress Types ---

// ScanProgressEvent represents a progress update during scanning
type ScanProgressEvent struct {
	Type       string `json:"type"`                  // "progress", "complete", "error"
	Processed  int    `json:"processed"`             // Number of images processed so far
	Total      int    `json:"total"`                 // Total number of images to process
	Current    string `json:"current,omitempty"`     // Current file being processed
	Status     string `json:"status,omitempty"`      // "processing", "success", "skipped", "error"
	Error      string `json:"error,omitempty"`       // Error message if status is "error"
	NewImages  int    `json:"new_images,omitempty"`  // For "complete" type
	Skipped    int    `json:"skipped,omitempty"`     // For "complete" type
	Errors     int    `json:"errors,omitempty"`      // For "complete" type
	RawImages  int    `json:"raw_images,omitempty"`  // RAW files processed
	TiffImages int    `json:"tiff_images,omitempty"` // TIFF files processed
}

// ScanCompleteResult represents the final scan result
type ScanCompleteResult struct {
	Type         string  `json:"type"`          // "complete"
	Success      bool    `json:"success"`       // Overall success
	Processed    int     `json:"processed"`     // Total processed
	Total        int     `json:"total"`         // Total files
	NewImages    int     `json:"new_images"`    // Newly indexed images
	Skipped      int     `json:"skipped"`       // Skipped (already indexed)
	Errors       int     `json:"errors"`        // Error count
	RawProcessed int     `json:"raw_processed"` // RAW files processed
	TiffProcessed int    `json:"tiff_processed"`// TIFF files processed
	DatabasePath string  `json:"database_path"` // Path to database
	DurationSecs float64 `json:"duration_secs"` // Time taken in seconds
}

// --- Search Result Types ---

// SearchMatch represents a single image match
type SearchMatch struct {
	Path         string  `json:"path"`
	Score        float64 `json:"score"`
	SourcePrefix string  `json:"source_prefix,omitempty"`
	Width        int     `json:"width,omitempty"`
	Height       int     `json:"height,omitempty"`
	Format       string  `json:"format,omitempty"`
}

// SearchResult represents the complete search result
type SearchResult struct {
	Success      bool          `json:"success"`
	Query        string        `json:"query"`
	Matches      []SearchMatch `json:"matches"`
	Total        int           `json:"total"`
	Threshold    float64       `json:"threshold"`
	DurationSecs float64       `json:"duration_secs"`
	Error        string        `json:"error,omitempty"`
}

// --- Database Info Types ---

// DatabaseInfo represents database statistics
type DatabaseInfo struct {
	Success          bool   `json:"success"`
	DatabasePath     string `json:"database_path"`
	TotalImages      int    `json:"total_images"`
	UniqueHashes     int    `json:"unique_hashes"`
	DatabaseSizeBytes int64 `json:"database_size_bytes"`
	LastModified     string `json:"last_modified,omitempty"`
	Error            string `json:"error,omitempty"`
}

// --- Error Types ---

// ErrorResult represents a JSON error output
type ErrorResult struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    int    `json:"code"`
}

// --- Helper Functions ---

// PrintJSON outputs any value as formatted JSON to stdout
func PrintJSON(v interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(v)
}

// PrintJSONLine outputs a value as a single JSON line (for streaming)
func PrintJSONLine(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// PrintError outputs an error in JSON format
func PrintError(message string, code int) {
	result := ErrorResult{
		Success: false,
		Error:   message,
		Code:    code,
	}
	PrintJSON(result)
}
