package imageprocessor

import (
	"fmt"
	"image"
	"path/filepath"
	"strings"

	"imagefinder/logging"

	"gocv.io/x/gocv"
)

// SearchOptions defines the options for searching
type SearchOptions struct {
	QueryPath    string
	Threshold    float64
	SourcePrefix string
	DebugMode    bool
}

// ImageMatch represents a matching image with similarity score
type ImageMatch struct {
	Path         string
	SourcePrefix string
	SSIMScore    float64
}

// LoadImage loads an image using the appropriate loader based on file type
func LoadImage(path string) (gocv.Mat, error) {
	// Get a loader registry
	registry := NewImageLoaderRegistry()

	// Get file extension
	ext := strings.ToLower(filepath.Ext(path))

	// Try to get a specialized loader
	loader := registry.GetLoader(ext)

	// Check if the loader exists and can load this file
	if loader != nil && loader.CanLoad(path) {
		return loader.LoadImage(path)
	}

	// Fallback to standard loading method
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, newImageLoadError("failed to load image", path)
	}

	return img, nil
}

// ComputeAverageHash calculates a simple average hash for the image
func ComputeAverageHash(img gocv.Mat) (string, error) {
	if img.Empty() {
		return "", fmt.Errorf("cannot compute hash for empty image")
	}

	// Resize to 8x8
	resized := gocv.NewMat()
	defer resized.Close()

	gocv.Resize(img, &resized, image.Point{X: 8, Y: 8}, 0, 0, gocv.InterpolationLinear)

	// Convert to grayscale if not already
	gray := gocv.NewMat()
	defer gray.Close()

	if img.Channels() != 1 {
		gocv.CvtColor(resized, &gray, gocv.ColorBGRToGray)
	} else {
		resized.CopyTo(&gray)
	}

	// Calculate mean pixel value manually
	var sum uint64
	var count int

	for y := 0; y < gray.Rows(); y++ {
		for x := 0; x < gray.Cols(); x++ {
			pixel := gray.GetUCharAt(y, x)
			sum += uint64(pixel)
			count++
		}
	}

	// Calculate average
	var threshold float64
	if count > 0 {
		threshold = float64(sum) / float64(count)
	}

	// Compute hash
	hash := ""
	for y := 0; y < gray.Rows(); y++ {
		for x := 0; x < gray.Cols(); x++ {
			pixel := gray.GetUCharAt(y, x)
			if float64(pixel) >= threshold {
				hash += "1"
			} else {
				hash += "0"
			}
		}
	}

	return hash, nil
}

// ComputePerceptualHash computes a DCT-based perceptual hash for the image
func ComputePerceptualHash(img gocv.Mat) (string, error) {
	if img.Empty() {
		return "", fmt.Errorf("cannot compute hash for empty image")
	}

	// Resize to 32x32
	resized := gocv.NewMat()
	defer resized.Close()

	gocv.Resize(img, &resized, image.Point{X: 32, Y: 32}, 0, 0, gocv.InterpolationLinear)

	// Convert to grayscale if not already
	gray := gocv.NewMat()
	defer gray.Close()

	if img.Channels() != 1 {
		gocv.CvtColor(resized, &gray, gocv.ColorBGRToGray)
	} else {
		resized.CopyTo(&gray)
	}

	// Convert to float for DCT
	floatImg := gocv.NewMat()
	defer floatImg.Close()
	gray.ConvertTo(&floatImg, gocv.MatTypeCV32F)

	// Apply DCT
	dct := gocv.NewMat()
	defer dct.Close()
	gocv.DCT(floatImg, &dct, 0)

	// Extract 8x8 low frequency components
	lowFreq := dct.Region(image.Rect(0, 0, 8, 8))
	defer lowFreq.Close()

	// Calculate median value
	values := make([]float32, 64)
	idx := 0
	for y := 0; y < lowFreq.Rows(); y++ {
		for x := 0; x < lowFreq.Cols(); x++ {
			values[idx] = lowFreq.GetFloatAt(y, x)
			idx++
		}
	}

	// Simple median calculation
	median := calculateMedian(values)

	// Compute hash
	hash := ""
	for y := 0; y < lowFreq.Rows(); y++ {
		for x := 0; x < lowFreq.Cols(); x++ {
			val := lowFreq.GetFloatAt(y, x)
			if val >= median {
				hash += "1"
			} else {
				hash += "0"
			}
		}
	}

	return hash, nil
}

// FindSimilarImages finds similar images in the database
func FindSimilarImages(db interface{}, options SearchOptions) ([]ImageMatch, error) {
	// This is a placeholder implementation that should be completed
	// based on your actual database structure and search logic
	logging.LogInfo("Searching for similar images to %s with threshold %f", options.QueryPath, options.Threshold)

	// Load query image
	queryImg, err := LoadImage(options.QueryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load query image: %v", err)
	}
	defer queryImg.Close()

	// Compute hashes for query image
	avgHash, err := ComputeAverageHash(queryImg)
	if err != nil {
		return nil, fmt.Errorf("failed to compute average hash: %v", err)
	}

	pHash, err := ComputePerceptualHash(queryImg)
	if err != nil {
		return nil, fmt.Errorf("failed to compute perceptual hash: %v", err)
	}

	logging.LogInfo("Query image hashes: avgHash=%s, pHash=%s", avgHash, pHash)

	// Placeholder - in a real implementation, you would:
	// 1. Query the database for potential matches based on hash similarity
	// 2. Calculate SSIM scores for the best hash matches
	// 3. Sort and return the top results

	return []ImageMatch{}, nil
}

// Utility function to calculate the median of a float array
func calculateMedian(values []float32) float32 {
	// Make a copy to avoid modifying the original slice
	valuesCopy := make([]float32, len(values))
	copy(valuesCopy, values)

	// Sort the values
	for i := 0; i < len(valuesCopy); i++ {
		for j := i + 1; j < len(valuesCopy); j++ {
			if valuesCopy[i] > valuesCopy[j] {
				valuesCopy[i], valuesCopy[j] = valuesCopy[j], valuesCopy[i]
			}
		}
	}

	// Calculate median
	length := len(valuesCopy)
	if length%2 == 0 {
		return (valuesCopy[length/2-1] + valuesCopy[length/2]) / 2
	}

	return valuesCopy[length/2]
}

// Helper function to create standardized image load errors
func newImageLoadError(message, path string) error {
	return fmt.Errorf("%s: %s", message, path)
}
