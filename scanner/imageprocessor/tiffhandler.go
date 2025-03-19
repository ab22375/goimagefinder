package imageprocessor

import (
	"fmt"
	"path/filepath"
	"strings"

	"imagefinder/logging"

	"gocv.io/x/gocv"
)

// TiffImageHandler handles TIFF image processing
type TiffImageHandler struct {
	DebugMode bool
}

// NewTiffImageHandler creates a new TiffImageHandler instance
func NewTiffImageHandler(debugMode bool) *TiffImageHandler {
	return &TiffImageHandler{
		DebugMode: debugMode,
	}
}

// LoadAndProcess loads a TIFF image and processes it
func (t *TiffImageHandler) LoadAndProcess(path string) (gocv.Mat, error) {
	if t.DebugMode {
		logging.DebugLog("Processing TIFF image with specialized TIFF loader: %s", path)
	}

	// Use specialized TIFF loader
	tiffLoader := NewTiffImageLoader()
	img, err := tiffLoader.LoadImage(path)

	// If specialized loader fails, fall back to standard loader
	if err != nil {
		if t.DebugMode {
			logging.LogWarning("TIFF specialized loader failed: %v, falling back to standard loader", err)
		}
		return LoadImage(path)
	} else if t.DebugMode {
		logging.DebugLog("Successfully loaded TIFF image: %s", path)
	}

	return img, nil
}

// TiffImageLoader is an interface for loading TIFF images
type TiffImageLoader interface {
	LoadImage(path string) (gocv.Mat, error)
}

// StandardTiffLoader implements both TiffImageLoader and ImageLoader interfaces
type StandardTiffLoader struct{}

// NewTiffImageLoader creates a new TIFF image loader
func NewTiffImageLoader() TiffImageLoader {
	return &StandardTiffLoader{}
}

// CanLoadFile checks if this loader can load the given file
func (l *StandardTiffLoader) CanLoadFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".tif" || ext == ".tiff"
}

// LoadImage loads a TIFF image
func (l *StandardTiffLoader) LoadImage(path string) (gocv.Mat, error) {
	// Use OpenCV's TIFF loader with special flags for multi-layer TIFFs
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, fmt.Errorf("failed to load TIFF image: %s", path)
	}
	return img, nil
}
