package processor

import (
	"fmt"
	"image"
	"runtime/debug"

	"imagefinder/imageprocessor"
	"imagefinder/logging"
)

// ImageProcessor is an adapter that simplifies interactions between the scanner
// and the imageprocessor package
type ImageProcessor struct {
	DebugMode bool
	registry  *imageprocessor.ImageLoaderRegistry
}

// NewImageProcessor creates a new ImageProcessor with appropriate configuration
func NewImageProcessor(debugMode bool) *ImageProcessor {
	return &ImageProcessor{
		DebugMode: debugMode,
		registry:  imageprocessor.NewImageLoaderRegistry(),
	}
}

// ProcessImage loads and processes an image based on its type
func (p *ImageProcessor) ProcessImage(path string, isRaw bool, isTiff bool) (image.Image, error) {
	var img image.Image
	var err error

	// Use defer to recover from any panics during image loading
	defer func() {
		if r := recover(); r != nil {
			stackTrace := debug.Stack()
			err = fmt.Errorf("panic during image loading: %v\nStack trace: %s", r, string(stackTrace))
			logging.LogError("Panic during image loading: %v, file: %s\nStack trace: %s", r, path, string(stackTrace))
			img = nil // Return nil to prevent further issues
		}
	}()

	// Load the image using the registry
	img, err = p.registry.LoadImage(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load image %s: %v", path, err)
	}

	// Skip nil images
	if img == nil {
		return nil, fmt.Errorf("image is nil after loading: %s", path)
	}

	// Log debug information if requested
	if p.DebugMode {
		format := "standard"
		if isRaw {
			format = "RAW"
		} else if isTiff {
			format = "TIFF"
		}
		logging.DebugLog("Successfully loaded %s format image: %s", format, path)
	}

	return img, nil
}

// ComputeImageHashes computes both average and perceptual hashes for an image
func (p *ImageProcessor) ComputeImageHashes(img image.Image, path string, fileFormat string, isRaw bool, isTiff bool) (struct {
	AvgHash string
	PHash   string
}, error) {
	var hashes struct {
		AvgHash string
		PHash   string
	}

	// Compute average hash with improved error handling
	avgHash, err := imageprocessor.ComputeAverageHash(img)
	if err != nil {
		return hashes, fmt.Errorf("cannot compute average hash for %s: %v", path, err)
	}
	hashes.AvgHash = avgHash

	// Compute perceptual hash with improved error handling
	pHash, err := imageprocessor.ComputePerceptualHash(img)
	if err != nil {
		return hashes, fmt.Errorf("cannot compute perceptual hash for %s: %v", path, err)
	}
	hashes.PHash = pHash

	// Log hash information for debugging special images
	if p.DebugMode && (isRaw || isTiff) {
		logging.DebugLog("%s image hashes - %s - avgHash: %s, pHash: %s",
			fileFormat, path, avgHash, pHash)
	}

	return hashes, nil
}

// IsImageFile checks if the path is a recognized image file
func (p *ImageProcessor) IsImageFile(path string) bool {
	return imageprocessor.IsImageFile(path)
}

// IsRawFormat checks if the path is a RAW format file
func (p *ImageProcessor) IsRawFormat(path string) bool {
	return imageprocessor.IsRawFormat(path)
}

// IsTiffFormat checks if the path is a TIFF format file
func (p *ImageProcessor) IsTiffFormat(path string) bool {
	return imageprocessor.IsTiffFormat(path)
}
