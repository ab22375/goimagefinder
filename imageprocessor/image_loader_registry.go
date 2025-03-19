package imageprocessor

import (
	"path/filepath"
	"strings"
	"sync"

	"gocv.io/x/gocv"
)

// ImageLoaderRegistry maintains a registry of image loaders
type ImageLoaderRegistry struct {
	loaders       map[string]ImageLoader
	defaultLoader ImageLoader
	mutex         sync.RWMutex
}

// NewImageLoaderRegistry creates a new image loader registry
func NewImageLoaderRegistry() *ImageLoaderRegistry {
	registry := &ImageLoaderRegistry{
		loaders: make(map[string]ImageLoader),
	}

	// Register standard image loaders for common formats
	registry.registerStandardLoaders()

	// Register RAW format loaders
	registry.registerRawLoaders()

	// Register TIF format loader
	registry.registerTiffLoader()

	return registry
}

// registerStandardLoaders registers loaders for standard image formats
func (r *ImageLoaderRegistry) registerStandardLoaders() {
	// Create a default image loader for standard formats
	defaultLoader := &StandardImageLoader{}

	// Register for common image formats
	r.RegisterLoader(".jpg", defaultLoader)
	r.RegisterLoader(".jpeg", defaultLoader)
	r.RegisterLoader(".png", defaultLoader)
	r.RegisterLoader(".bmp", defaultLoader)
	r.RegisterLoader(".gif", defaultLoader)
	r.RegisterLoader(".webp", defaultLoader)

	// Set the default loader
	r.defaultLoader = defaultLoader
}

// registerRawLoaders registers loaders for RAW camera formats
func (r *ImageLoaderRegistry) registerRawLoaders() {
	// Register RAW format loaders
	r.RegisterLoader(".raf", NewRAFImageLoader())
	r.RegisterLoader(".nef", NewNEFImageLoader())
	r.RegisterLoader(".arw", NewARWImageLoader())
	r.RegisterLoader(".cr2", NewCR2ImageLoader())
	r.RegisterLoader(".cr3", NewCR3ImageLoader())
	r.RegisterLoader(".dng", NewDNGImageLoader())
	r.RegisterLoader(".nrw", NewRawImageLoader()) // Fallback for Nikon NRW
	r.RegisterLoader(".srf", NewRawImageLoader()) // Fallback for Sony SRF
}

// registerTiffLoader registers loader for TIFF images
func (r *ImageLoaderRegistry) registerTiffLoader() {
	tiffLoader := NewTiffImageLoader()
	r.RegisterLoader(".tif", tiffLoader)
	r.RegisterLoader(".tiff", tiffLoader)
}

// RegisterLoader registers a new loader for a specific file extension
func (r *ImageLoaderRegistry) RegisterLoader(ext string, loader ImageLoader) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	ext = strings.ToLower(ext)
	r.loaders[ext] = loader
}

// GetLoader returns the appropriate loader for the given path
func (r *ImageLoaderRegistry) GetLoader(path string) ImageLoader {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	ext := strings.ToLower(filepath.Ext(path))
	if loader, ok := r.loaders[ext]; ok {
		return loader
	}

	return r.defaultLoader
}

// CanLoadFile checks if any registered loader can handle the given file
func (r *ImageLoaderRegistry) CanLoadFile(path string) bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	ext := strings.ToLower(filepath.Ext(path))
	_, ok := r.loaders[ext]
	return ok
}

// StandardImageLoader is the default loader for common image formats
type StandardImageLoader struct{}

// CanLoad checks if this loader can handle the given file
func (l *StandardImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	supportedExts := []string{".jpg", ".jpeg", ".png", ".bmp", ".gif", ".webp"}

	for _, supported := range supportedExts {
		if ext == supported {
			return fileExists(path)
		}
	}

	return false
}

// LoadImage loads a standard image format
func (l *StandardImageLoader) LoadImage(path string) (gocv.Mat, error) {
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, newImageLoadError("failed to load image", path)
	}
	return img, nil
}
