package imageprocessor

import (
	"fmt"
	"math"

	"imagefinder/imageprocessor"
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
	// Use the shared implementation from the parent imageprocessor package
	return imageprocessor.ComputeAverageHash(img)
}

// ComputePerceptualHash computes a perceptual hash for an image
func ComputePerceptualHash(img gocv.Mat) (string, error) {
	// Use the shared implementation from the parent imageprocessor package
	return imageprocessor.ComputePerceptualHash(img)
}

// applyDCT applies a Discrete Cosine Transform to an image
// Simplified implementation when OpenCV's DCT is not available
func applyDCT(img gocv.Mat) gocv.Mat {
	rows, cols := img.Rows(), img.Cols()
	result := gocv.NewMatWithSize(rows, cols, gocv.MatTypeCV32F)

	for u := 0; u < rows; u++ {
		for v := 0; v < cols; v++ {
			sum := float32(0.0)
			for i := 0; i < rows; i++ {
				for j := 0; j < cols; j++ {
					// DCT-II formula
					cosU := float32(math.Cos(float64(math.Pi*float64(u)*(2*float64(i)+1)) / (2 * float64(rows))))
					cosV := float32(math.Cos(float64(math.Pi*float64(v)*(2*float64(j)+1)) / (2 * float64(cols))))
					sum += img.GetFloatAt(i, j) * cosU * cosV
				}
			}

			// Apply scaling factors
			scaleU := float32(1.0)
			if u == 0 {
				scaleU = 1.0 / float32(math.Sqrt(2.0))
			}

			scaleV := float32(1.0)
			if v == 0 {
				scaleV = 1.0 / float32(math.Sqrt(2.0))
			}

			scaleFactor := (2.0 * scaleU * scaleV) / float32(math.Sqrt(float64(rows*cols)))
			result.SetFloatAt(u, v, sum*scaleFactor)
		}
	}

	return result
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
