package imageprocessor

import (
	"path/filepath"
	"strings"

	"gocv.io/x/gocv"
)

// ImageLoader is an interface for loading images
type ImageLoader interface {
	CanLoadFile(path string) bool
	LoadImage(path string) (gocv.Mat, error)
}

// ImageLoaderRegistry manages available image loaders
type ImageLoaderRegistry struct {
	loaders []ImageLoader
}

// NewImageLoaderRegistry creates a new image loader registry with all available loaders
func NewImageLoaderRegistry() *ImageLoaderRegistry {
	return &ImageLoaderRegistry{
		loaders: []ImageLoader{
			&StandardImageLoader{},
			&StandardTiffLoader{},
			// Add other loaders here as needed
		},
	}
}

// CanLoadFile checks if any registered loader can handle this file
func (r *ImageLoaderRegistry) CanLoadFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	// Quick check for common image formats
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return true
	case ".cr2", ".nef", ".arw", ".orf", ".rw2", ".pef", ".dng", ".raw", ".raf", ".cr3", ".nrw", ".srf":
		return true
	case ".tif", ".tiff":
		return true
	}

	// If not a common format, check with each loader
	for _, loader := range r.loaders {
		if loader.CanLoadFile(path) {
			return true
		}
	}

	return false
}

// StandardImageLoader implements ImageLoader for standard image formats
type StandardImageLoader struct{}

// CanLoadFile checks if this loader can load the given file
func (l *StandardImageLoader) CanLoadFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return true
	default:
		return false
	}
}

// LoadImage loads an image
func (l *StandardImageLoader) LoadImage(path string) (gocv.Mat, error) {
	return LoadImage(path)
}
