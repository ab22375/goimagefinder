package imageprocessor

import (
	"image"
	"image/jpeg"
	"io"

	"github.com/disintegration/imaging"
)

// WriteJPEGThumbnail writes a JPEG thumbnail of the given image to the writer
func WriteJPEGThumbnail(w io.Writer, img image.Image, maxSize int) error {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate new dimensions maintaining aspect ratio
	var newWidth, newHeight int
	if width > height {
		newWidth = maxSize
		newHeight = int(float64(height) * float64(maxSize) / float64(width))
	} else {
		newHeight = maxSize
		newWidth = int(float64(width) * float64(maxSize) / float64(height))
	}

	// Ensure minimum size
	if newWidth < 1 {
		newWidth = 1
	}
	if newHeight < 1 {
		newHeight = 1
	}

	// Use imaging library for high-quality resizing
	thumbnail := imaging.Resize(img, newWidth, newHeight, imaging.Lanczos)

	// Encode as JPEG with reasonable quality
	return jpeg.Encode(w, thumbnail, &jpeg.Options{Quality: 85})
}

// WriteJPEGThumbnailFromImage is an alias for WriteJPEGThumbnail for backwards compatibility
func WriteJPEGThumbnailFromImage(w io.Writer, img image.Image, maxSize int) error {
	return WriteJPEGThumbnail(w, img, maxSize)
}
