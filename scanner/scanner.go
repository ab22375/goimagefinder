package scanner

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"imagefinder/database"
	"imagefinder/logging"
	"imagefinder/scanner/imageprocessor"
	"imagefinder/types"
)

// ScanAndStoreFolder scans a folder and stores image information in the database
func ScanAndStoreFolder(db *sql.DB, options ScanOptions) error {
	// Determine concurrency limit
	maxWorkers := 8 // Default
	if options.MaxWorkers > 0 {
		maxWorkers = options.MaxWorkers
	}

	// Initialize components for parallel processing
	var wg sync.WaitGroup
	resultsChan := make(chan ProcessImageResult, 100)
	semaphore := make(chan struct{}, maxWorkers)

	// Count and classify files before processing
	fileStats := countFilesToProcess(options)

	// Display initial information
	PrintStartupInfo(fileStats, options)

	// Set up progress tracking
	progressTracker := NewProgressTracker(fileStats, resultsChan)
	defer progressTracker.Stop()

	// Process files
	startTime := time.Now()
	err := walkAndProcessFiles(db, options, &wg, resultsChan, semaphore)

	// Wait for all processing to complete
	wg.Wait()
	close(resultsChan)
	close(semaphore)

	// Print final statistics
	PrintCompletionStats(progressTracker, startTime, options)

	return err
}

// countFilesToProcess counts and classifies files to be processed
func countFilesToProcess(options ScanOptions) FileStats {
	stats := FileStats{}
	loaderRegistry := imageprocessor.NewImageLoaderRegistry()

	if options.DebugMode {
		logging.DebugLog("Starting image scan on folder: %s", options.FolderPath)
		logging.DebugLog("Force rewrite: %v, Source prefix: %s", options.ForceRewrite, options.SourcePrefix)
	}

	filepath.Walk(options.FolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if this is an image file we can process
		if loaderRegistry.CanLoadFile(path) || IsImageFile(path) {
			stats.totalFiles++

			// Check if it's a RAW file
			if IsRawFormat(path) {
				stats.rawFiles++
			}

			// Check if it's a TIF file
			if IsTiffFormat(path) {
				stats.tifFiles++
			}
		}
		return nil
	})

	return stats
}

// walkAndProcessFiles processes all image files in a directory
func walkAndProcessFiles(db *sql.DB, options ScanOptions, wg *sync.WaitGroup, resultsChan chan ProcessImageResult, semaphore chan struct{}) error {
	// Create image processor
	processor := imageprocessor.NewImageProcessor(options.DebugMode)

	// Create registry to identify image files
	loaderRegistry := imageprocessor.NewImageLoaderRegistry()

	// Walk through the directory tree
	return filepath.Walk(options.FolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if options.DebugMode {
				logging.LogError("Failed to access path %s: %v", path, err)
			}
			return nil // Continue with other files
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Skip files that we can't handle
		if !loaderRegistry.CanLoadFile(path) && !IsImageFile(path) {
			return nil
		}

		// Add to wait group and launch goroutine for processing
		wg.Add(1)
		go func(filePath string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Check file type
			isRawImage := IsRawFormat(filePath)
			isTifImage := IsTiffFormat(filePath)

			// Process image with panic recovery
			var result ProcessImageResult

			func() {
				// Use defer to catch panics
				defer func() {
					if r := recover(); r != nil {
						err := fmt.Errorf("panic during image processing: %v", r)
						result = ProcessImageResult{
							Path:    filePath,
							Success: false,
							Error:   err,
							IsRaw:   isRawImage,
							IsTif:   isTifImage,
						}
						logging.LogError("Recovered from panic processing %s: %v", filePath, r)
					}
				}()

				// Process the image
				result = processAndStoreImage(db, filePath, options.SourcePrefix, options, processor)
				result.IsRaw = isRawImage
				result.IsTif = isTifImage
			}()

			// Send result to channel
			resultsChan <- result
		}(path)

		return nil
	})
}

// processAndStoreImage processes a single image and stores it in the database
func processAndStoreImage(db *sql.DB, path string, sourcePrefix string, options ScanOptions, processor *imageprocessor.ImageProcessor) ProcessImageResult {
	result := ProcessImageResult{
		Path:    path,
		Success: false,
	}

	// Skip processing if the image already exists and hasn't been modified
	if !options.ForceRewrite {
		if skipResult := checkAndSkipIfUnchanged(db, path, sourcePrefix, options); skipResult != nil {
			return *skipResult
		}
	}

	// Get file info and format
	fileInfo, err := os.Stat(path)
	if err != nil {
		result.Error = fmt.Errorf("cannot stat file %s: %v", path, err)
		return result
	}

	fileFormat := GetFileFormat(path)
	isRawImage := IsRawFormat(path)
	isTifImage := IsTiffFormat(path)

	// Load and process the image
	img, err := processor.ProcessImage(path, isRawImage, isTifImage)
	if err != nil {
		result.Error = fmt.Errorf("failed to load image %s: %v", path, err)
		return result
	}
	defer img.Close()

	// Skip empty images
	if img.Empty() {
		result.Error = fmt.Errorf("image is empty after loading: %s", path)
		return result
	}

	// Compute hashes
	imageHashes, err := processor.ComputeImageHashes(img, path, fileFormat, isRawImage, isTifImage)
	if err != nil {
		result.Error = err
		return result
	}

	// Create and store image info
	imageInfo := types.ImageInfo{
		Path:           path,
		SourcePrefix:   sourcePrefix,
		Format:         fileFormat,
		Width:          img.Cols(),
		Height:         img.Rows(),
		ModifiedAt:     fileInfo.ModTime().Format(time.RFC3339),
		Size:           fileInfo.Size(),
		AverageHash:    imageHashes.AvgHash,
		PerceptualHash: imageHashes.PHash,
		IsRawFormat:    isRawImage,
	}

	// Store in database
	err = database.StoreImageInfo(db, imageInfo, options.ForceRewrite)
	if err != nil {
		result.Error = fmt.Errorf("cannot store data for %s: %v", path, err)
		return result
	}

	if options.DebugMode && (isRawImage || isTifImage) {
		logging.DebugLog("Successfully indexed %s image: %s", fileFormat, path)
	}

	result.Success = true
	return result
}
