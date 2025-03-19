package database

import (
	"database/sql"
	"fmt"
	"time"

	"imagefinder/logging"
	"imagefinder/types"

	_ "github.com/mattn/go-sqlite3"
)

// InitDatabase initializes and returns a database connection
func InitDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS images (
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
	CREATE INDEX IF NOT EXISTS idx_path ON images(path);
	CREATE INDEX IF NOT EXISTS idx_average_hash ON images(average_hash);
	CREATE INDEX IF NOT EXISTS idx_perceptual_hash ON images(perceptual_hash);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return nil, err
	}

	// Check if format column exists, add it if it doesn't
	var hasFormatColumn bool
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('images') WHERE name='format'").Scan(&hasFormatColumn)
	if err != nil {
		return nil, fmt.Errorf("error checking for format column: %v", err)
	}

	if !hasFormatColumn {
		// Add format column to existing table
		_, err = db.Exec("ALTER TABLE images ADD COLUMN format TEXT;")
		if err != nil {
			return nil, fmt.Errorf("error adding format column: %v", err)
		}
		logging.DebugLog("Added 'format' column to existing database schema")
	}

	// Check if source_prefix column exists, add it if it doesn't
	var hasSourcePrefixColumn bool
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('images') WHERE name='source_prefix'").Scan(&hasSourcePrefixColumn)
	if err != nil {
		return nil, fmt.Errorf("error checking for source_prefix column: %v", err)
	}

	if !hasSourcePrefixColumn {
		// Add source_prefix column to existing table
		_, err = db.Exec("ALTER TABLE images ADD COLUMN source_prefix TEXT;")
		if err != nil {
			return nil, fmt.Errorf("error adding source_prefix column: %v", err)
		}
		logging.DebugLog("Added 'source_prefix' column to existing database schema")

		// If we're adding this column to an existing DB, we need to
		// update the uniqueness constraint (can't directly modify in SQLite)
		// In a real app, you'd create a new table and migrate the data
		logging.DebugLog("Note: To fully update schema, consider rebuilding the database.")
	}

	return db, nil
}

// OpenDatabase opens an existing database connection
func OpenDatabase(dbPath string) (*sql.DB, error) {
	return sql.Open("sqlite3", dbPath)
}

// CheckImageExists checks if an image already exists in the database
func CheckImageExists(db *sql.DB, path string, sourcePrefix string) (bool, string, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM images WHERE path = ? AND source_prefix = ?", path, sourcePrefix).Scan(&count)
	if err != nil {
		return false, "", fmt.Errorf("database error for %s: %v", path, err)
	}

	if count == 0 {
		return false, "", nil
	}

	// Get the stored modification time
	var storedModTime string
	err = db.QueryRow("SELECT modified_at FROM images WHERE path = ? AND source_prefix = ?", path, sourcePrefix).Scan(&storedModTime)
	if err != nil {
		return true, "", fmt.Errorf("cannot get modified time for %s: %v", path, err)
	}

	return true, storedModTime, nil
}

// StoreImageInfo stores image information in the database
func StoreImageInfo(db *sql.DB, imageInfo types.ImageInfo, forceRewrite bool) error {
	// Store in database
	now := time.Now().Format(time.RFC3339)

	// Prepare statement to avoid SQL injection
	var stmt *sql.Stmt
	var insertErr error

	if forceRewrite {
		// Always use INSERT OR REPLACE when force rewrite is enabled
		stmt, insertErr = db.Prepare(`
			INSERT OR REPLACE INTO images (
				path, source_prefix, format, width, height, created_at, modified_at, size, average_hash, perceptual_hash
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
	} else {
		// Use conditional insert or update
		stmt, insertErr = db.Prepare(`
			INSERT OR IGNORE INTO images (
				path, source_prefix, format, width, height, created_at, modified_at, size, average_hash, perceptual_hash
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
	}

	if insertErr != nil {
		return fmt.Errorf("cannot prepare statement for %s: %v", imageInfo.Path, insertErr)
	}
	defer stmt.Close()

	_, err := stmt.Exec(
		imageInfo.Path,
		imageInfo.SourcePrefix,
		imageInfo.Format,
		imageInfo.Width,
		imageInfo.Height,
		now,
		imageInfo.ModifiedAt,
		imageInfo.Size,
		imageInfo.AverageHash,
		imageInfo.PerceptualHash,
	)

	if err != nil {
		return fmt.Errorf("cannot insert data for %s: %v", imageInfo.Path, err)
	}

	return nil
}

// QueryPotentialMatches retrieves potential image matches based on source prefix
func QueryPotentialMatches(db *sql.DB, sourcePrefix string) (*sql.Rows, error) {
	var query string
	var args []interface{}

	if sourcePrefix != "" {
		// Filter by source prefix if specified
		query = `SELECT path, source_prefix, average_hash, perceptual_hash FROM images WHERE source_prefix = ?`
		args = []interface{}{sourcePrefix}
	} else {
		// No source prefix filter
		query = `SELECT path, source_prefix, average_hash, perceptual_hash FROM images`
	}

	// Query database for potential matches
	return db.Query(query, args...)
}

// ScanStats contains statistics from a scan operation
type ScanStats struct {
	TotalImages  int
	ErrorCount   int
	UniqueHashes int
}

// GetScanStats retrieves statistics about scanned images
func GetScanStats(db *sql.DB, sourcePrefix string) (*ScanStats, error) {
	var stats ScanStats
	var err error

	// Count total images
	var totalQuery string
	var args []interface{}

	if sourcePrefix != "" {
		totalQuery = "SELECT COUNT(*) FROM images WHERE source_prefix = ?"
		args = append(args, sourcePrefix)
	} else {
		totalQuery = "SELECT COUNT(*) FROM images"
	}

	err = db.QueryRow(totalQuery, args...).Scan(&stats.TotalImages)
	if err != nil {
		return nil, fmt.Errorf("failed to get total images: %v", err)
	}

	// Count unique hashes
	var hashQuery string
	if sourcePrefix != "" {
		hashQuery = "SELECT COUNT(DISTINCT average_hash) FROM images WHERE source_prefix = ?"
	} else {
		hashQuery = "SELECT COUNT(DISTINCT average_hash) FROM images"
	}

	err = db.QueryRow(hashQuery, args...).Scan(&stats.UniqueHashes)
	if err != nil {
		return nil, fmt.Errorf("failed to get unique hashes: %v", err)
	}

	// Count errors (assuming you track errors in the database)
	// If you don't have an explicit error field, this can be omitted or adapted
	stats.ErrorCount = 0

	return &stats, nil
}
