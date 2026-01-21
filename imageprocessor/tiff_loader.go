package imageprocessor

import (
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/disintegration/imaging"
	"imagefinder/logging"
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
func (l *EnhancedTiffImageLoader) LoadImage(path string) (image.Image, error) {
	logging.LogInfo("Loading TIFF image with specialized loader: %s", path)

	// First try direct loading with imaging library
	// This works for many standard TIFF files
	img, err := imaging.Open(path)
	if err == nil {
		logging.LogInfo("Successfully loaded TIFF using direct load: %s", path)
		return imaging.Grayscale(img), nil
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
				img, err := imaging.Open(tempFilename)
				if err == nil {
					return imaging.Grayscale(img), nil
				}
			}
		}
	}

	// If all conversion methods fail, try with standard Go image packages
	logging.LogInfo("All TIFF conversion methods failed, trying Go standard image packages")
	if goImg, err := tryGoImagePackages(path); err == nil {
		return imaging.Grayscale(goImg), nil
	}

	return nil, fmt.Errorf("failed to load TIFF image (all methods failed): %s", path)
}

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
