package imageprocessor

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"imagefinder/logging"

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

	// Register specialized format loaders
	registry.registerSpecializedLoaders()

	return registry
}

// registerStandardLoaders registers loaders for standard image formats
func (r *ImageLoaderRegistry) registerStandardLoaders() {
	// Create a default image loader for standard formats
	standardLoader := NewStandardImageLoader()

	// Register for common image formats
	r.RegisterLoader(".jpg", standardLoader)
	r.RegisterLoader(".jpeg", standardLoader)
	r.RegisterLoader(".png", standardLoader)
	r.RegisterLoader(".bmp", standardLoader)
	r.RegisterLoader(".gif", standardLoader)
	r.RegisterLoader(".webp", standardLoader)

	// Set the default loader
	r.defaultLoader = standardLoader
}

// registerSpecializedLoaders registers loaders for specialized formats
func (r *ImageLoaderRegistry) registerSpecializedLoaders() {
	// Register TIFF format loader
	tiffLoader := NewTiffImageLoader()
	r.RegisterLoader(".tif", tiffLoader)
	r.RegisterLoader(".tiff", tiffLoader)

	// Register RAW format loaders using a simple implementation for now
	// This will be enhanced in specialized files
	simpleRawLoader := NewSimpleRawImageLoader()
	
	// Register for common RAW formats
	r.RegisterLoader(".raw", simpleRawLoader)
	r.RegisterLoader(".raf", simpleRawLoader)
	r.RegisterLoader(".nef", simpleRawLoader)
	r.RegisterLoader(".arw", simpleRawLoader)
	r.RegisterLoader(".cr2", simpleRawLoader)
	r.RegisterLoader(".dng", simpleRawLoader)
	r.RegisterLoader(".nrw", simpleRawLoader)
	r.RegisterLoader(".srf", simpleRawLoader)
	
	// Register specialized CR3 loader if available
	if checkExiftoolCommandAvailable() {
		// If exiftool is available, use the specialized loader
		r.RegisterLoader(".cr3", NewCR3ExiftoolLoader())
		logging.LogInfo("Registered specialized CR3ExiftoolLoader")
	} else {
		// Otherwise fallback to simple loader
		r.RegisterLoader(".cr3", simpleRawLoader)
		logging.LogInfo("Registered fallback RAW loader for CR3")
	}
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

// LoadImage loads an image using the appropriate registered loader
func (r *ImageLoaderRegistry) LoadImage(path string) (gocv.Mat, error) {
	loader := r.GetLoader(path)
	if loader == nil {
		return gocv.NewMat(), fmt.Errorf("no suitable loader found for: %s", path)
	}
	
	return loader.LoadImage(path)
}