package scanner

import (
	"database/sql"
	"fmt"
	"os"
	"time"

	"imagefinder/database"
	"imagefinder/logging"
)

// checkAndSkipIfUnchanged checks if an image can be skipped because it hasn't changed
func checkAndSkipIfUnchanged(db *sql.DB, path string, sourcePrefix string, options ScanOptions) *ProcessImageResult {
	exists, storedModTime, err := database.CheckImageExists(db, path, sourcePrefix)
	if err != nil {
		return &ProcessImageResult{
			Path:    path,
			Success: false,
			Error:   fmt.Errorf("database error for %s: %v", path, err),
		}
	}

	if exists {
		// Image already indexed, check if it needs update
		fileInfo, err := os.Stat(path)
		if err != nil {
			return &ProcessImageResult{
				Path:    path,
				Success: false,
				Error:   fmt.Errorf("cannot stat file %s: %v", path, err),
			}
		}

		// Parse stored time and compare with file modified time
		storedTime, err := time.Parse(time.RFC3339, storedModTime)
		if err != nil {
			return &ProcessImageResult{
				Path:    path,
				Success: false,
				Error:   fmt.Errorf("cannot parse stored time for %s: %v", path, err),
			}
		}

		// If file hasn't been modified, skip processing
		if !fileInfo.ModTime().After(storedTime) {
			if options.DebugMode {
				logging.DebugLog("Skipping unchanged image: %s", path)
			}
			return &ProcessImageResult{
				Path:    path,
				Success: true,
			}
		}
	}

	return nil
}
