package imageprocessor

import (
	"fmt"
	"os"

	"gocv.io/x/gocv"
)

// ImageLoader interface defines methods for image loading
type ImageLoader interface {
	// CanLoad determines if this loader can handle the given file
	CanLoad(path string) bool
	
	// LoadImage loads an image and returns the gocv.Mat representation
	LoadImage(path string) (gocv.Mat, error)
}

// BaseImageLoader provides common functionality for all image loaders
type BaseImageLoader struct {
	// Formats this loader can handle
	SupportedFormats []FormatType
}

// CanLoad checks if this loader supports the file's format
func (l *BaseImageLoader) CanLoad(path string) bool {
	format := GetFileFormat(path)
	
	// Check if format is supported
	for _, supported := range l.SupportedFormats {
		if format == supported {
			return fileExists(path)
		}
	}
	
	return false
}

// DefaultLoadImage provides a standard image loading implementation
// This can be used by loaders that support standard formats
func (l *BaseImageLoader) DefaultLoadImage(path string) (gocv.Mat, error) {
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, fmt.Errorf("failed to load image: %s", path)
	}
	return img, nil
}

// Utility functions for loaders

// fileExists checks if a file exists and is accessible
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// hasFileContent checks if a file exists and has a non-zero size
func hasFileContent(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// newImageLoadError creates a standardized error for image loading failures
func newImageLoadError(message, path string) error {
	return fmt.Errorf("%s: %s", message, path)
}