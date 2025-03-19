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

// ScanAndStoreFolder scans a folder and stores image information in the database
func ScanAndStoreFolder(db *sql.DB, options ScanOptions) error {
	// Initialize components for parallel processing
	var wg sync.WaitGroup
	resultsChan := make(chan ProcessImageResult, 100)
	semaphore := make(chan struct{}, 8) // Limit concurrent goroutines

	// Count and classify files before processing
	fileStats := countFilesToProcess(options)

	// Display initial information
	printStartupInfo(fileStats, options)

	// Set up progress tracking
	progressTracker := setupProgressTracker(fileStats, resultsChan)
	defer progressTracker.stop()

	// Process files
	startTime := time.Now()
	err := walkAndProcessFiles(db, options, &wg, resultsChan, semaphore)

	// Wait for all processing to complete
	wg.Wait()
	close(resultsChan)
	close(semaphore)

	// Print final statistics
	printCompletionStats(progressTracker, startTime, options)

	return err
}

// FileStats tracks information about files to be processed
type FileStats struct {
	totalFiles int
	rawFiles   int
	tifFiles   int
}

// countFilesToProcess counts and classifies files to be processed
func countFilesToProcess(options ScanOptions) FileStats {
	stats := FileStats{}
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
			stats.totalFiles++

			// Check if it's a RAW file
			ext := strings.ToLower(filepath.Ext(path))
			rawFormats := []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf"}
			for _, format := range rawFormats {
				if ext == format {
					stats.rawFiles++
					break
				}
			}

			// Check if it's a TIF file
			if ext == ".tif" || ext == ".tiff" {
				stats.tifFiles++
			}
		}
		return nil
	})

	return stats
}

// printStartupInfo displays information about the scan before starting
func printStartupInfo(stats FileStats, options ScanOptions) {
	fmt.Printf("Starting image indexing...\nTotal image files to process: %d (including %d RAW files and %d TIF files)\n",
		stats.totalFiles, stats.rawFiles, stats.tifFiles)
	fmt.Printf("Force rewrite mode: %v\n", options.ForceRewrite)

	if options.SourcePrefix != "" {
		fmt.Printf("Source prefix: %s\n", options.SourcePrefix)
	}

	if options.DebugMode {
		fmt.Printf("Debug mode: enabled\n")
		logging.DebugLog("Found %d image files to process (%d RAW files, %d TIF files)",
			stats.totalFiles, stats.rawFiles, stats.tifFiles)
	}
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

// setupProgressTracker initializes the progress tracker
func setupProgressTracker(stats FileStats, resultsChan chan ProcessImageResult) *ProgressTracker {
	tracker := &ProgressTracker{
		ticker:     time.NewTicker(500 * time.Millisecond),
		done:       make(chan bool),
		totalFiles: stats.totalFiles,
		rawFiles:   stats.rawFiles,
		tifFiles:   stats.tifFiles,
	}

	// Start progress display goroutine
	go tracker.displayProgress()

	// Start result processor goroutine
	go tracker.processResults(resultsChan)

	return tracker
}

// displayProgress shows the progress periodically
func (p *ProgressTracker) displayProgress() {
	for {
		select {
		case <-p.done:
			return
		case <-p.ticker.C:
			p.mu.Lock()
			if p.errors > 0 {
				fmt.Printf("\rProgress: %d/%d (Errors: %d, RAW: %d/%d, TIF: %d/%d)",
					p.processed, p.totalFiles, p.errors, p.rawProcessed, p.rawFiles, p.tifProcessed, p.tifFiles)
			} else {
				fmt.Printf("\rProgress: %d/%d (RAW: %d/%d, TIF: %d/%d)",
					p.processed, p.totalFiles, p.rawProcessed, p.rawFiles, p.tifProcessed, p.tifFiles)
			}
			p.mu.Unlock()
		}
	}
}

// processResults updates the tracker state based on processing results
func (p *ProgressTracker) processResults(resultsChan chan ProcessImageResult) {
	for result := range resultsChan {
		p.mu.Lock()
		p.processed++

		// Check file type
		ext := strings.ToLower(filepath.Ext(result.Path))

		// Check if it's a RAW file
		isRawFile := false
		rawFormats := []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf"}
		for _, format := range rawFormats {
			if ext == format {
				isRawFile = true
				p.rawProcessed++
				break
			}
		}

		// Check if it's a TIF file
		isTifFile := ext == ".tif" || ext == ".tiff"
		if isTifFile {
			p.tifProcessed++
		}

		if !result.Success {
			p.errors++
			if isRawFile {
				p.rawErrors++
			}
			if isTifFile {
				p.tifErrors++
			}
			// Log the error if debug mode is enabled
			if result.Error != nil {
				logging.LogImageProcessed(result.Path, false, result.Error.Error())
			}
		} else {
			logging.LogImageProcessed(result.Path, true, "")
		}

		p.mu.Unlock()
	}
}

// stop ends the progress tracking
func (p *ProgressTracker) stop() {
	p.ticker.Stop()
	p.done <- true
}

// walkAndProcessFiles traverses the directory and processes each file
func walkAndProcessFiles(db *sql.DB, options ScanOptions, wg *sync.WaitGroup, resultsChan chan ProcessImageResult, semaphore chan struct{}) error {
	registry := imageprocessor.NewImageLoaderRegistry()

	// Walk through directory and process files
	return filepath.Walk(options.FolderPath, func(path string, info os.FileInfo, err error) error {
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
}

// printCompletionStats displays statistics after scan completion
func printCompletionStats(tracker *ProgressTracker, startTime time.Time, options ScanOptions) {
	elapsed := time.Since(startTime)

	// Log final statistics
	if options.DebugMode {
		logging.DebugLog("Scan completed in %v. Processed: %d, Errors: %d, RAW files: %d, RAW errors: %d, TIF files: %d, TIF errors: %d",
			elapsed, tracker.processed, tracker.errors, tracker.rawProcessed, tracker.rawErrors,
			tracker.tifProcessed, tracker.tifErrors)
	}

	fmt.Println("\nIndexing complete.")
	fmt.Printf("Processed %d images in %v.\n", tracker.processed, elapsed.Round(time.Second))

	if tracker.rawProcessed > 0 {
		fmt.Printf("Successfully processed %d/%d RAW image files.\n",
			tracker.rawProcessed-tracker.rawErrors, tracker.rawFiles)
	}

	if tracker.tifProcessed > 0 {
		fmt.Printf("Successfully processed %d/%d TIF image files.\n",
			tracker.tifProcessed-tracker.tifErrors, tracker.tifFiles)
	}

	if tracker.errors > 0 {
		fmt.Printf("Encountered %d errors during indexing.\n", tracker.errors)
		fmt.Println("Check the log file for details.")
	}
}

// processAndStoreImage processes a single image and stores it in the database
func processAndStoreImage(db *sql.DB, path string, sourcePrefix string, options ScanOptions) ProcessImageResult {
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

	fileFormat := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
	isRawImage := isRawFormat(path)
	isTifImage := isTifFormat(path)

	// Load the image based on its type
	img, err := loadImageBasedOnType(path, isRawImage, isTifImage, options.DebugMode)
	if err != nil {
		result.Error = fmt.Errorf("failed to load image %s: %v", path, err)
		return result
	}
	defer img.Close()

	// Compute hashes
	imageHashes, err := computeImageHashes(img, path, fileFormat, isRawImage, isTifImage, options.DebugMode)
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
		AverageHash:    imageHashes.avgHash,
		PerceptualHash: imageHashes.pHash,
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

// ImageHashes contains computed hashes for an image
type ImageHashes struct {
	avgHash string
	pHash   string
}

// computeImageHashes computes average and perceptual hashes for an image
func computeImageHashes(img gocv.Mat, path string, fileFormat string, isRawImage bool, isTifImage bool, debugMode bool) (ImageHashes, error) {
	var hashes ImageHashes

	// Compute average hash
	avgHash, err := imageprocessor.ComputeAverageHash(img)
	if err != nil {
		return hashes, fmt.Errorf("cannot compute average hash for %s: %v", path, err)
	}
	hashes.avgHash = avgHash

	// Compute perceptual hash
	pHash, err := imageprocessor.ComputePerceptualHash(img)
	if err != nil {
		return hashes, fmt.Errorf("cannot compute perceptual hash for %s: %v", path, err)
	}
	hashes.pHash = pHash

	// Log hash information for debugging special images
	if debugMode && (isRawImage || isTifImage) {
		logging.DebugLog("%s image hashes - %s - avgHash: %s, pHash: %s",
			fileFormat, path, avgHash, pHash)
	}

	return hashes, nil
}

// loadImageBasedOnType loads an image using appropriate method based on file type
func loadImageBasedOnType(path string, isRawImage bool, isTifImage bool, debugMode bool) (gocv.Mat, error) {
	var img gocv.Mat
	var err error

	if isRawImage {
		if debugMode {
			logging.DebugLog("Converting RAW image to JPG for consistent hashing: %s", path)
		}

		// First try our dedicated RAW to JPG conversion
		img, err = convertRawToJpgAndLoad(path)

		// If conversion fails, fall back to standard loader
		if err != nil {
			if debugMode {
				logging.LogWarning("RAW to JPG conversion failed: %v, falling back to standard loader", err)
			}
			img, err = imageprocessor.LoadImage(path)
		} else if debugMode {
			logging.DebugLog("Successfully converted RAW to JPG for: %s", path)
		}
	} else if isTifImage {
		if debugMode {
			logging.DebugLog("Processing TIFF image with specialized TIFF loader: %s", path)
		}

		// Use specialized TIFF loader
		tiffLoader := imageprocessor.NewTiffImageLoader()
		img, err = tiffLoader.LoadImage(path)

		// If specialized loader fails, fall back to standard loader
		if err != nil {
			if debugMode {
				logging.LogWarning("TIFF specialized loader failed: %v, falling back to standard loader", err)
			}
			img, err = imageprocessor.LoadImage(path)
		} else if debugMode {
			logging.DebugLog("Successfully loaded TIFF image: %s", path)
		}
	} else {
		// For non-RAW and non-TIF files, load normally
		img, err = imageprocessor.LoadImage(path)
	}

	return img, err
}

// Helper to check if a file is in TIF format
func isTifFormat(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".tif" || ext == ".tiff"
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

// convertRawToJpgAndLoad converts a RAW file to JPG and loads it for hashing
func convertRawToJpgAndLoad(path string) (gocv.Mat, error) {
	tempDir := os.TempDir()
	tempJpg := filepath.Join(tempDir, fmt.Sprintf("std_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempJpg) // Clean up temp file when done

	// Special handling for CR3 files
	if strings.ToLower(filepath.Ext(path)) == ".cr3" {
		err := convertCR3WithExiftool(path, tempJpg)
		if err == nil {
			// Check if the file was created successfully
			_, statErr := os.Stat(tempJpg)
			if statErr == nil {
				// Load the standard JPG representation
				img := gocv.IMRead(tempJpg, gocv.IMReadGrayScale)
				if !img.Empty() {
					return img, nil
				}
			}
		}
	}

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

// convertCR3WithExiftool specialized function for CR3 files which often need special handling
func convertCR3WithExiftool(path, outputPath string) error {
	// CR3 files often have multiple preview images
	// Try extracting the largest preview image
	cmd := exec.Command("exiftool", "-b", "-LargePreviewImage", "-w", outputPath, path)
	err := cmd.Run()

	// Check if the output file was created and has content
	if err == nil {
		info, statErr := os.Stat(outputPath)
		if statErr == nil && info.Size() > 0 {
			return nil
		}
	}

	// Try alternative tags for preview images
	tags := []string{
		"PreviewImage",
		"OtherImage",
		"ThumbnailImage",
		"FullPreviewImage",
	}

	for _, tag := range tags {
		cmd := exec.Command("exiftool", "-b", "-"+tag, "-w", outputPath, path)
		err := cmd.Run()

		// Check if successful
		if err == nil {
			info, statErr := os.Stat(outputPath)
			if statErr == nil && info.Size() > 0 {
				return nil
			}
		}
	}

	return fmt.Errorf("could not extract any preview image from CR3 file")
}
