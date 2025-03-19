// Package imageprocessor provides tools for loading and processing various image formats.
package imageprocessor

import "gocv.io/x/gocv"

// ImageLoader is the interface that all image loaders must implement
type ImageLoader interface {
	// CanLoad checks if the loader can handle the given file
	CanLoad(path string) bool

	// LoadImage loads and returns the image
	LoadImage(path string) (gocv.Mat, error)
}
