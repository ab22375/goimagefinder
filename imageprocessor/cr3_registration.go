package imageprocessor

import "imagefinder/logging"

// RegisterCR3Loaders registers all CR3 loader implementations with the registry
// in order of preference. Call this from your main initialization code.
func RegisterCR3Loaders(registry *ImageLoaderRegistry) {
	// First try the go-exiftool loader if available
	if checkGoExiftoolAvailable() {
		registry.RegisterLoader(".cr3", NewCR3ExiftoolLoader())
		logging.LogInfo("Registered CR3ExiftoolLoader")
	} else {
		// Then register the pure Go parser if exiftool is not available
		registry.RegisterLoader(".cr3", NewCR3Parser())
		logging.LogInfo("Registered CR3Parser")
	}

	// Register the enhanced loader under a different key for fallback
	registry.RegisterLoader(".cr3_enhanced", NewEnhancedCR3ImageLoader())
	logging.LogInfo("Registered EnhancedCR3ImageLoader")
}

// Adding this function to image_loader_registry.go's NewImageLoaderRegistry() function:
// func NewImageLoaderRegistry() *ImageLoaderRegistry {
//    registry := &ImageLoaderRegistry{
//        loaders: make(map[string]ImageLoader),
//    }
//
//    // Register standard image loaders for common formats
//    registry.registerStandardLoaders()
//
//    // Register RAW format loaders
//    registry.registerRawLoaders()
//
//    // Register TIF format loader
//    registry.registerTiffLoader()
//
//    // Add this line to register CR3 loaders
//    RegisterCR3Loaders(registry)
//
//    return registry
// }
