package imageprocessor

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"
	
	"gocv.io/x/gocv"
	"golang.org/x/image/draw"
)

// WriteJPEGThumbnail writes a JPEG thumbnail of the given gocv.Mat to the writer
func WriteJPEGThumbnail(w io.Writer, mat gocv.Mat, maxSize int) error {
	// Convert gocv.Mat to image.Image
	img, err := mat.ToImage()
	if err != nil {
		return fmt.Errorf("failed to convert mat to image: %w", err)
	}
	
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
	
	// Create a new image for the thumbnail
	thumbnail := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))
	
	// Use high-quality resizing
	draw.BiLinear.Scale(thumbnail, thumbnail.Bounds(), img, bounds, draw.Over, nil)
	
	// Encode as JPEG with reasonable quality
	return jpeg.Encode(w, thumbnail, &jpeg.Options{Quality: 85})
}