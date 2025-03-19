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

	// Get file info
	fileInfo, err := os.Stat(path)
	if err != nil {
		result.Error = fmt.Errorf("cannot stat file %s: %v", path, err)
		return result
	}

	// Get file format from extension (without the dot)
	fileFormat := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")

	// Detect if this is a RAW image or TIF image
	isRawImage := isRawFormat(path)
	isTifImage := isTifFormat(path)

	// Load and process the image - for RAW/TIF files, handle specially
	var img gocv.Mat

	if isRawImage {
		if options.DebugMode {
			logging.DebugLog("Converting RAW image to JPG for consistent hashing: %s", path)
		}

		// First try our dedicated RAW to JPG conversion
		img, err = convertRawToJpgAndLoad(path)

		// If conversion fails, fall back to standard loader
		if err != nil {
			if options.DebugMode {
				logging.LogWarning("RAW to JPG conversion failed: %v, falling back to standard loader", err)
			}
			img, err = imageprocessor.LoadImage(path)
		} else if options.DebugMode {
			logging.DebugLog("Successfully converted RAW to JPG for: %s", path)
		}
	} else if isTifImage {
		if options.DebugMode {
			logging.DebugLog("Processing TIFF image with specialized TIFF loader: %s", path)
		}

		// Use specialized TIFF loader
		tiffLoader := imageprocessor.NewTiffImageLoader()
		img, err = tiffLoader.LoadImage(path)

		// If specialized loader fails, fall back to standard loader
		if err != nil {
			if options.DebugMode {
				logging.LogWarning("TIFF specialized loader failed: %v, falling back to standard loader", err)
			}
			img, err = imageprocessor.LoadImage(path)
		} else if options.DebugMode {
			logging.DebugLog("Successfully loaded TIFF image: %s", path)
		}
	} else {
		// For non-RAW and non-TIF files, load normally
		img, err = imageprocessor.LoadImage(path)
	}

	if err != nil {
		result.Error = fmt.Errorf("failed to load image %s: %v", path, err)
		return result
	}
	defer img.Close()

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

	// Log hash information for debugging special images
	if options.DebugMode && (isRawImage || isTifImage) {
		logging.DebugLog("%s image hashes - %s - avgHash: %s, pHash: %s",
			fileFormat, path, avgHash, pHash)
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

// loadTifImage loads a TIFF image using specialized methods
func loadTifImage(path string, debugMode bool) (gocv.Mat, error) {
	// First try direct loading with gocv (may not work with all TIFFs)
	img := gocv.IMRead(path, gocv.IMReadUnchanged)
	if !img.Empty() {
		return img, nil
	}

	// If direct loading fails, convert to temporary JPG and load
	tempDir := os.TempDir()
	tempJpg := filepath.Join(tempDir, fmt.Sprintf("tif_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempJpg) // Clean up temp file when done

	if debugMode {
		logging.DebugLog("Converting TIF image to JPG for consistent loading: %s", path)
	}

	// Try multiple TIF to JPG conversion methods
	methods := []func(string, string) error{
		convertTifWithImageMagick,
		convertTifWithVips,
		convertTifWithGdal,
	}

	var lastError error
	for _, method := range methods {
		err := method(path, tempJpg)
		if err == nil {
			// Check if the file was created successfully
			_, statErr := os.Stat(tempJpg)
			if statErr == nil {
				// Load the standard JPG representation
				convertedImg := gocv.IMRead(tempJpg, gocv.IMReadGrayScale)
				if !convertedImg.Empty() {
					return convertedImg, nil
				}
			}
		}
		lastError = err
	}

	// If all methods fail, return the error
	return gocv.NewMat(), fmt.Errorf("failed to load TIF image: %v", lastError)
}

// convertTifWithImageMagick converts a TIF file to JPG using ImageMagick
func convertTifWithImageMagick(path, outputPath string) error {
	// Use ImageMagick to convert TIF to JPG
	cmd := exec.Command("convert", path, outputPath)
	return cmd.Run()
}

// convertTifWithVips converts a TIF file to JPG using libvips
func convertTifWithVips(path, outputPath string) error {
	// Use vips to convert TIF to JPG
	cmd := exec.Command("vips", "copy", path, outputPath)
	return cmd.Run()
}

// convertTifWithGdal converts a TIF file to JPG using GDAL (good for geospatial TIFFs)
func convertTifWithGdal(path, outputPath string) error {
	// Use gdal_translate to convert TIF to JPG
	cmd := exec.Command("gdal_translate", "-of", "JPEG", "-co", "QUALITY=90", path, outputPath)
	return cmd.Run()
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
	var tifFiles int
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

			// Check if it's a RAW file
			rawFormats := []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf"}
			for _, format := range rawFormats {
				if ext == format {
					rawFiles++
					break
				}
			}

			// Check if it's a TIF file
			if ext == ".tif" || ext == ".tiff" {
				tifFiles++
			}
		}
		return nil
	})

	fmt.Printf("Starting image indexing...\nTotal image files to process: %d (including %d RAW files and %d TIF files)\n",
		totalFiles, rawFiles, tifFiles)
	fmt.Printf("Force rewrite mode: %v\n", options.ForceRewrite)
	if options.SourcePrefix != "" {
		fmt.Printf("Source prefix: %s\n", options.SourcePrefix)
	}
	if options.DebugMode {
		fmt.Printf("Debug mode: enabled\n")
		logging.DebugLog("Found %d image files to process (%d RAW files, %d TIF files)", totalFiles, rawFiles, tifFiles)
	}

	// Create a ticker for progress indicator
	ticker := time.NewTicker(500 * time.Millisecond)
	done := make(chan bool)
	processed := 0
	errors := 0
	rawProcessed := 0
	rawErrors := 0
	tifProcessed := 0
	tifErrors := 0
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
					fmt.Printf("\rProgress: %d/%d (Errors: %d, RAW: %d/%d, TIF: %d/%d)",
						processed, totalFiles, errors, rawProcessed, rawFiles, tifProcessed, tifFiles)
				} else {
					fmt.Printf("\rProgress: %d/%d (RAW: %d/%d, TIF: %d/%d)",
						processed, totalFiles, rawProcessed, rawFiles, tifProcessed, tifFiles)
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

			// Check file type
			ext := strings.ToLower(filepath.Ext(result.Path))

			// Check if it's a RAW file
			isRawFile := false
			rawFormats := []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf"}
			for _, format := range rawFormats {
				if ext == format {
					isRawFile = true
					rawProcessed++
					break
				}
			}

			// Check if it's a TIF file
			isTifFile := ext == ".tif" || ext == ".tiff"
			if isTifFile {
				tifProcessed++
			}

			if !result.Success {
				errors++
				if isRawFile {
					rawErrors++
				}
				if isTifFile {
					tifErrors++
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
		logging.DebugLog("Scan completed in %v. Processed: %d, Errors: %d, RAW files: %d, RAW errors: %d, TIF files: %d, TIF errors: %d",
			elapsed, processed, errors, rawProcessed, rawErrors, tifProcessed, tifErrors)
	}

	fmt.Printf("Processed %d images in %v.\n", processed, elapsed.Round(time.Second))
	if rawProcessed > 0 {
		fmt.Printf("Successfully processed %d/%d RAW image files.\n", rawProcessed-rawErrors, rawFiles)
	}
	if tifProcessed > 0 {
		fmt.Printf("Successfully processed %d/%d TIF image files.\n", tifProcessed-tifErrors, tifFiles)
	}

	if errors > 0 {
		fmt.Printf("Encountered %d errors during indexing.\n", errors)
		fmt.Println("Check the log file for details.")
	}

	return err
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
