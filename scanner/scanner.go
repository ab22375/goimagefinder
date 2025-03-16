package scanner

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"imagefinder/database"
	"imagefinder/imageprocessor"
	"imagefinder/logging"
	"imagefinder/types"

	"gocv.io/x/gocv"
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

	// Detect if this is a RAW image
	isRawImage := isRawFormat(path)

	// Get file info
	fileInfo, err := os.Stat(path)
	if err != nil {
		result.Error = fmt.Errorf("cannot stat file %s: %v", path, err)
		return result
	}

	// Get file format from extension (without the dot)
	fileFormat := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")

	// Load and process the image - for RAW files, convert to JPG first
	var img gocv.Mat

	if isRawImage {
		if options.DebugMode {
			logging.DebugLog("Converting RAW image to JPG for consistent hashing: %s", path)
		}

		// Try to use our standard RAW loader first, which already has fallback mechanisms
		img, err = imageprocessor.LoadImage(path)

		// If standard loading fails, we don't attempt the conversion
		if err != nil {
			result.Error = fmt.Errorf("failed to load RAW image %s: %v", path, err)
			return result
		}

		// Successfully loaded the RAW image using one of the existing methods
		if options.DebugMode {
			logging.DebugLog("Successfully loaded RAW image using standard loader: %s", path)
		}
	} else {
		// For non-RAW files, load normally
		img, err = imageprocessor.LoadImage(path)
		if err != nil {
			result.Error = fmt.Errorf("failed to load image %s: %v", path, err)
			return result
		}
	}

	defer img.Close()

	// Compute hashes - now always based on JPG representation for RAW files
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

	// Log hash information for debugging raw images
	if options.DebugMode && isRawImage {
		logging.DebugLog("RAW image (converted to JPG) hashes - %s - avgHash: %s, pHash: %s", path, avgHash, pHash)
	}

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
		IsRawFormat:    isRawImage, // This field needs to be in your types.ImageInfo struct
	}

	// Store in database
	err = database.StoreImageInfo(db, imageInfo, options.ForceRewrite)
	if err != nil {
		result.Error = fmt.Errorf("cannot store data for %s: %v", path, err)
		return result
	}

	if options.DebugMode && isRawImage {
		logging.DebugLog("Successfully indexed RAW image (using JPG conversion): %s", path)
	}

	result.Success = true
	return result
}

// convertRawToJpgAndLoad converts a RAW file to JPG and loads it for hashing
func convertRawToJpgAndLoad(path string) (gocv.Mat, error) {
	tempDir := os.TempDir()
	tempJpg := filepath.Join(tempDir, fmt.Sprintf("std_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempJpg) // Clean up temp file when done

	// Try different conversion methods in order of preference
	methods := []func(string, string) error{
		extractPreviewWithExiftool, // Extract embedded preview (best match for camera JPGs)
		convertWithDcrawAutoBright, // Use dcraw with auto-brightness
		convertWithDcrawCameraWB,   // Use dcraw with camera white balance
	}

	var lastError error
	for _, method := range methods {
		err := method(path, tempJpg)
		if err == nil {
			// Check if the file was created successfully
			_, err = os.Stat(tempJpg)
			if err == nil {
				// Load the standard JPG representation
				img := gocv.IMRead(tempJpg, gocv.IMReadGrayScale)
				if !img.Empty() {
					return img, nil
				}
			}
		}
		lastError = err
	}

	// If all methods fail, return the error
	return gocv.NewMat(), fmt.Errorf("failed to convert RAW to JPG: %v", lastError)
}

// Extract the embedded preview JPEG from the RAW file using exiftool
func extractPreviewWithExiftool(path, outputPath string) error {
	// Use exiftool to extract the preview image
	// -b = output in binary mode
	// -PreviewImage = extract the preview image
	cmd := exec.Command("exiftool", "-b", "-PreviewImage", "-w", outputPath, path)
	return cmd.Run()
}

// Convert using dcraw with auto-brightness, which often matches camera output
func convertWithDcrawAutoBright(path, outputPath string) error {
	// -w = use camera white balance
	// -a = auto-brightness (mimics camera)
	// -q 3 = high-quality interpolation
	// -O = output to specified file
	cmd := exec.Command("dcraw", "-w", "-a", "-q", "3", "-O", outputPath, path)
	return cmd.Run()
}

// Convert using dcraw with camera white balance, no auto-brightness
func convertWithDcrawCameraWB(path, outputPath string) error {
	// -w = use camera white balance
	// -q 3 = high-quality interpolation
	// -O = output to specified file
	cmd := exec.Command("dcraw", "-w", "-q", "3", "-O", outputPath, path)
	return cmd.Run()
}

// Helper to check if a file is in RAW format
func isRawFormat(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	rawFormats := []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf"}
	for _, format := range rawFormats {
		if ext == format {
			return true
		}
	}
	return false
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
	var rawFiles int
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
			// Count RAW images separately
			ext := strings.ToLower(filepath.Ext(path))
			rawFormats := []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf"}
			for _, format := range rawFormats {
				if ext == format {
					rawFiles++
					break
				}
			}
		}
		return nil
	})

	fmt.Printf("Starting image indexing...\nTotal image files to process: %d (including %d RAW files)\n", totalFiles, rawFiles)
	fmt.Printf("Force rewrite mode: %v\n", options.ForceRewrite)
	if options.SourcePrefix != "" {
		fmt.Printf("Source prefix: %s\n", options.SourcePrefix)
	}
	if options.DebugMode {
		fmt.Printf("Debug mode: enabled\n")
		logging.DebugLog("Found %d image files to process (%d RAW files)", totalFiles, rawFiles)
	}

	// Create a ticker for progress indicator
	ticker := time.NewTicker(500 * time.Millisecond)
	done := make(chan bool)
	processed := 0
	errors := 0
	rawProcessed := 0
	rawErrors := 0
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
					fmt.Printf("\rProgress: %d/%d (Errors: %d, RAW: %d/%d)", processed, totalFiles, errors, rawProcessed, rawFiles)
				} else {
					fmt.Printf("\rProgress: %d/%d (RAW: %d/%d)", processed, totalFiles, rawProcessed, rawFiles)
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

			// Check if this is a RAW file
			ext := strings.ToLower(filepath.Ext(result.Path))
			isRawFile := false
			rawFormats := []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf"}
			for _, format := range rawFormats {
				if ext == format {
					isRawFile = true
					rawProcessed++
					break
				}
			}

			if !result.Success {
				errors++
				if isRawFile {
					rawErrors++
				}
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
		logging.DebugLog("Scan completed in %v. Processed: %d, Errors: %d, RAW files: %d, RAW errors: %d",
			elapsed, processed, errors, rawProcessed, rawErrors)
	}

	fmt.Printf("Processed %d images in %v.\n", processed, elapsed.Round(time.Second))
	if rawProcessed > 0 {
		fmt.Printf("Successfully processed %d/%d RAW image files.\n", rawProcessed-rawErrors, rawFiles)
	}

	if errors > 0 {
		fmt.Printf("Encountered %d errors during indexing.\n", errors)
		fmt.Println("Check the log file for details.")
	}

	return err
}
