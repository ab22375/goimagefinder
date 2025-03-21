package imageprocessor

import (
	"fmt"
	"os/exec"
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

	// If specialized loader fails, try using exiftool to extract a preview image
	if err != nil || img.Empty() {
		if t.DebugMode {
			logging.LogWarning("TIFF specialized loader failed: %v, trying exiftool extraction", err)
		}

		img, err = t.extractWithExiftool(path)
		if err != nil || img.Empty() {
			if t.DebugMode {
				logging.LogWarning("Exiftool extraction failed: %v, falling back to standard loader", err)
			}

			// Fall back to standard loader as last resort
			img = gocv.IMRead(path, gocv.IMReadGrayScale)
			if img.Empty() {
				return img, fmt.Errorf("all TIFF loading methods failed for %s: %v", path, err)
			}
		}
	} else if t.DebugMode {
		logging.DebugLog("Successfully loaded TIFF image: %s", path)
	}

	return img, nil
}

// extractWithExiftool tries to extract a preview image from a TIFF file
func (t *TiffImageHandler) extractWithExiftool(path string) (gocv.Mat, error) {
	// Try to create a temporary JPEG from the TIFF using exiftool
	tempPath := filepath.Join(filepath.Dir(path),
		fmt.Sprintf("temp_%s.jpg", filepath.Base(path)))

	cmd := exec.Command("exiftool", "-b", "-PreviewImage", "-o", tempPath, path)
	stderr, err := cmd.CombinedOutput()

	if err != nil {
		return gocv.NewMat(), fmt.Errorf("exiftool extraction failed: %v, stderr: %s", err, string(stderr))
	}

	// Try to read the temporary JPEG
	img := gocv.IMRead(tempPath, gocv.IMReadGrayScale)
	if img.Empty() {
		// Try extracting ThumbnailImage instead
		cmd = exec.Command("exiftool", "-b", "-ThumbnailImage", "-o", tempPath, path)
		stderr, err = cmd.CombinedOutput()

		if err != nil {
			return gocv.NewMat(), fmt.Errorf("exiftool thumbnail extraction failed: %v, stderr: %s", err, string(stderr))
		}

		img = gocv.IMRead(tempPath, gocv.IMReadGrayScale)
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
		// Try alternative approach with ANYDEPTH flag
		img = gocv.IMRead(path, gocv.IMReadGrayScale|gocv.IMReadAnyDepth)

		if img.Empty() {
			return img, fmt.Errorf("failed to load TIFF image: %s", path)
		}
	}
	return img, nil
}
