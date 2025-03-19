package imageprocessor

import (
	"fmt"

	"imagefinder/logging"

	"gocv.io/x/gocv"
)

// ImageProcessor handles image processing operations
type ImageProcessor struct {
	DebugMode    bool
	RawConverter *RawImageConverter
	TiffHandler  *TiffImageHandler
}

// NewImageProcessor creates a new ImageProcessor instance
func NewImageProcessor(debugMode bool) *ImageProcessor {
	return &ImageProcessor{
		DebugMode:    debugMode,
		RawConverter: NewRawImageConverter(debugMode),
		TiffHandler:  NewTiffImageHandler(debugMode),
	}
}

// LoadImage loads an image from a file path
func LoadImage(path string) (gocv.Mat, error) {
	// Load the image with OpenCV
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, fmt.Errorf("failed to load image: %s", path)
	}
	return img, nil
}

// ProcessImage processes an image based on its type
func (p *ImageProcessor) ProcessImage(path string, isRaw bool, isTiff bool) (gocv.Mat, error) {
	var img gocv.Mat
	var err error

	// Use defer to recover from any panics during image loading
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during image loading: %v", r)
			logging.LogError("Panic during image loading: %v, file: %s", r, path)
			if !img.Empty() {
				img.Close()
			}
			img = gocv.NewMat() // Return an empty Mat to prevent further issues
		}
	}()

	// Process based on image type
	if isRaw {
		if p.DebugMode {
			logging.DebugLog("Converting RAW image to JPG for consistent hashing: %s", path)
		}
		img, err = p.RawConverter.ConvertAndLoad(path)
	} else if isTiff {
		img, err = p.TiffHandler.LoadAndProcess(path)
	} else {
		// For standard image formats
		img, err = LoadImage(path)
	}

	if err != nil {
		return gocv.NewMat(), err
	}

	// Skip empty images
	if img.Empty() {
		return img, fmt.Errorf("image is empty after loading: %s", path)
	}

	return img, nil
}

// ComputeAverageHash computes an average hash for an image
func ComputeAverageHash(img gocv.Mat) (string, error) {
	// Implementation of average hash computation
	// This is a placeholder - you'd use the actual implementation from your imageprocessor package
	return "averagehash-placeholder", nil
}

// ComputePerceptualHash computes a perceptual hash for an image
func ComputePerceptualHash(img gocv.Mat) (string, error) {
	// Implementation of perceptual hash computation
	// This is a placeholder - you'd use the actual implementation from your imageprocessor package
	return "perceptualhash-placeholder", nil
}

// ComputeImageHashes computes both average and perceptual hashes for an image
func (p *ImageProcessor) ComputeImageHashes(img gocv.Mat, path string, fileFormat string, isRaw bool, isTiff bool) (struct {
	AvgHash string
	PHash   string
}, error) {
	var hashes struct {
		AvgHash string
		PHash   string
	}

	// Compute average hash
	avgHash, err := ComputeAverageHash(img)
	if err != nil {
		return hashes, fmt.Errorf("cannot compute average hash for %s: %v", path, err)
	}
	hashes.AvgHash = avgHash

	// Compute perceptual hash
	pHash, err := ComputePerceptualHash(img)
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
