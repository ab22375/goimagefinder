package scanner

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"imagefinder/database"
	"imagefinder/imageprocessor"
	"imagefinder/logging"
	"imagefinder/types"
)

// ScanOptions defines the options for scanning
type ScanOptions struct {
	FolderPath   string
	SourcePrefix string
	ForceRewrite bool
	DebugMode    bool
	DbPath       string
}

// ProcessImageResult holds the result of processing an image
type ProcessImageResult struct {
	Path    string
	Success bool
	Error   error
}

// processAndStoreImage processes a single image and stores it in the database
func processAndStoreImage(db *sql.DB, path string, sourcePrefix string, options ScanOptions) ProcessImageResult {
	result := ProcessImageResult{
		Path:    path,
		Success: false,
	}

	// Skip processing if the image already exists and hasn't been modified
	if !options.ForceRewrite {
		exists, storedModTime, err := database.CheckImageExists(db, path, sourcePrefix)
		if err != nil {
			result.Error = fmt.Errorf("database error for %s: %v", path, err)
			return result
		}

		if exists {
			// Image already indexed, check if it needs update
			fileInfo, err := os.Stat(path)
			if err != nil {
				result.Error = fmt.Errorf("cannot stat file %s: %v", path, err)
				return result
			}

			// Parse stored time and compare with file modified time
			storedTime, err := time.Parse(time.RFC3339, storedModTime)
			if err != nil {
				result.Error = fmt.Errorf("cannot parse stored time for %s: %v", path, err)
				return result
			}

			// If file hasn't been modified, skip processing
			if !fileInfo.ModTime().After(storedTime) {
				if options.DebugMode {
					logging.DebugLog("Skipping unchanged image: %s", path)
				}
				result.Success = true
				return result
			}
		}
	}

	// Load and process the image
	img, err := imageprocessor.LoadImage(path)
	if err != nil {
		result.Error = fmt.Errorf("failed to load image %s: %v", path, err)
		return result
	}
	defer img.Close()

	// Get file info
	fileInfo, err := os.Stat(path)
	if err != nil {
		result.Error = fmt.Errorf("cannot stat file %s: %v", path, err)
		return result
	}

	// Compute hashes
	avgHash, err := imageprocessor.ComputeAverageHash(img)
	if err != nil {
		result.Error = fmt.Errorf("cannot compute average hash for %s: %v", path, err)
		return result
	}

	pHash, err := imageprocessor.ComputePerceptualHash(img)
	if err != nil {
		result.Error = fmt.Errorf("cannot compute perceptual hash for %s: %v", path, err)
		return result
	}

	// Get file format from extension
	fileFormat := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")

	// Create ImageInfo object
	imageInfo := types.ImageInfo{
		Path:           path,
		SourcePrefix:   sourcePrefix,
		Format:         fileFormat,
		Width:          img.Cols(),
		Height:         img.Rows(),
		ModifiedAt:     fileInfo.ModTime().Format(time.RFC3339),
		Size:           fileInfo.Size(),
		AverageHash:    avgHash,
		PerceptualHash: pHash,
	}

	// Store in database
	err = database.StoreImageInfo(db, imageInfo, options.ForceRewrite)
	if err != nil {
		result.Error = fmt.Errorf("cannot store data for %s: %v", path, err)
		return result
	}

	result.Success = true
	return result
}

// ScanAndStoreFolder scans a folder and stores image information in the database
func ScanAndStoreFolder(db *sql.DB, options ScanOptions) error {
	var wg sync.WaitGroup

	// Channel for collecting errors without blocking
	resultsChan := make(chan ProcessImageResult, 100)

	// Semaphore to limit concurrent goroutines
	semaphore := make(chan struct{}, 8)

	// Count total files before starting
	var totalFiles int
	registry := imageprocessor.NewImageLoaderRegistry()

	if options.DebugMode {
		logging.DebugLog("Starting image scan on folder: %s", options.FolderPath)
		logging.DebugLog("Force rewrite: %v, Source prefix: %s", options.ForceRewrite, options.SourcePrefix)
	}

	filepath.Walk(options.FolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if any loader can handle this file
		if registry.CanLoadFile(path) {
			totalFiles++
		}
		return nil
	})

	fmt.Printf("Starting image indexing...\nTotal image files to process: %d\n", totalFiles)
	fmt.Printf("Force rewrite mode: %v\n", options.ForceRewrite)
	if options.SourcePrefix != "" {
		fmt.Printf("Source prefix: %s\n", options.SourcePrefix)
	}
	if options.DebugMode {
		fmt.Printf("Debug mode: enabled\n")
		logging.DebugLog("Found %d image files to process", totalFiles)
	}

	// Create a ticker for progress indicator
	ticker := time.NewTicker(500 * time.Millisecond)
	done := make(chan bool)
	processed := 0
	errors := 0
	var mu sync.Mutex

	// Progress display goroutine
	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				if errors > 0 {
					fmt.Printf("\rProgress: %d/%d (Errors: %d)", processed, totalFiles, errors)
				} else {
					fmt.Printf("\rProgress: %d/%d", processed, totalFiles)
				}
				mu.Unlock()
			}
		}
	}()

	// Result processor goroutine
	go func() {
		for result := range resultsChan {
			mu.Lock()
			processed++

			if !result.Success {
				errors++
				if options.DebugMode {
					logging.LogImageProcessed(result.Path, false, result.Error.Error())
				}
			} else if options.DebugMode {
				logging.LogImageProcessed(result.Path, true, "")
			}

			mu.Unlock()
		}
	}()

	// Process files
	startTime := time.Now()
	err := filepath.Walk(options.FolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			if err != nil && options.DebugMode {
				logging.LogError("Error accessing path %s: %v", path, err)
			}
			return nil
		}

		// Check if we have a loader for this file
		registry := imageprocessor.NewImageLoaderRegistry()
		if registry.CanLoadFile(path) {
			wg.Add(1)
			// Acquire semaphore
			semaphore <- struct{}{}

			go func(p string) {
				defer wg.Done()
				defer func() { <-semaphore }() // Release semaphore when done

				result := processAndStoreImage(db, p, options.SourcePrefix, options)
				resultsChan <- result
			}(path)
		}

		return nil
	})

	// Wait for all goroutines to complete
	wg.Wait()
	close(resultsChan)
	close(semaphore)

	// Stop the progress indicator
	ticker.Stop()
	done <- true
	fmt.Println("\nIndexing complete.")

	// Log final statistics
	elapsed := time.Since(startTime)
	if options.DebugMode {
		logging.DebugLog("Scan completed in %v. Processed: %d, Errors: %d",
			elapsed, processed, errors)
	}

	if errors > 0 {
		fmt.Printf("Encountered %d errors during indexing.\n", errors)
		fmt.Println("Check the log file for details.")
	}

	return err
}
