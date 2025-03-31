package imageprocessor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"imagefinder/logging"

	"gocv.io/x/gocv"
)

// EnhancedTiffImageLoader is a more advanced TIFF loader with specialized conversion methods
type EnhancedTiffImageLoader struct {
	BaseImageLoader
	TempDir string
}

// NewEnhancedTiffImageLoader creates a new enhanced loader for TIFF files
func NewEnhancedTiffImageLoader() *EnhancedTiffImageLoader {
	tempDir := os.TempDir()
	return &EnhancedTiffImageLoader{
		BaseImageLoader: BaseImageLoader{
			SupportedFormats: []FormatType{FormatTIFF},
		},
		TempDir: tempDir,
	}
}

// LoadImage loads a TIFF image with advanced methods
func (l *EnhancedTiffImageLoader) LoadImage(path string) (gocv.Mat, error) {
	logging.LogInfo("Loading TIFF image with specialized loader: %s", path)

	// First try direct loading with OpenCV
	// This works for many standard TIFF files
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if !img.Empty() {
		logging.LogInfo("Successfully loaded TIFF using direct load: %s", path)
		return img, nil
	}

	// If direct loading fails, try conversion methods
	tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("tiff_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempFilename) // Clean up temp file when done

	// Try different methods for TIFF conversion in order of preference
	methods := []func(string, string) error{
		l.convertTiffWithImageMagick,
		l.convertTiffWithVips,
		l.convertTiffWithGdal,
	}

	for _, method := range methods {
		err := method(path, tempFilename)
		if err == nil {
			// Check if file exists and has content
			if hasFileContent(tempFilename) {
				img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
				if !img.Empty() {
					return img, nil
				}
			}
		}
	}

	// If all conversion methods fail, try with standard Go image packages
	logging.LogInfo("All TIFF conversion methods failed, trying Go standard image packages")
	if goImg, err := tryGoImagePackages(path); err == nil {
		// Convert Go image to OpenCV Mat
		return gocvMatFromGoImage(goImg)
	}

	return gocv.NewMat(), fmt.Errorf("failed to load TIFF image (all methods failed): %s", path)
}

// Check if file has content using the utility function

// convertTiffWithImageMagick converts a TIFF file to JPEG using ImageMagick
func (l *EnhancedTiffImageLoader) convertTiffWithImageMagick(path, outputPath string) error {
	_, err := exec.LookPath("convert")
	if err != nil {
		return os.ErrNotExist
	}

	cmd := exec.Command("convert", path, outputPath)
	return cmd.Run()
}

// convertTiffWithVips converts a TIFF file to JPEG using libvips
func (l *EnhancedTiffImageLoader) convertTiffWithVips(path, outputPath string) error {
	_, err := exec.LookPath("vips")
	if err != nil {
		return os.ErrNotExist
	}

	cmd := exec.Command("vips", "copy", path, outputPath)
	return cmd.Run()
}

// convertTiffWithGdal converts a TIFF file to JPEG using GDAL (good for geospatial TIFFs)
func (l *EnhancedTiffImageLoader) convertTiffWithGdal(path, outputPath string) error {
	_, err := exec.LookPath("gdal_translate")
	if err != nil {
		return os.ErrNotExist
	}

	cmd := exec.Command("gdal_translate", "-of", "JPEG", "-co", "QUALITY=90", path, outputPath)
	return cmd.Run()
}