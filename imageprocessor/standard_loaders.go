package imageprocessor

import (
	"fmt"
	"os/exec"
	"strings"

	"gocv.io/x/gocv"
	"imagefinder/logging"
)

// StandardImageLoader handles common image formats like JPEG, PNG, etc.
type StandardImageLoader struct {
	BaseImageLoader
}

// NewStandardImageLoader creates a new loader for standard image formats
func NewStandardImageLoader() *StandardImageLoader {
	return &StandardImageLoader{
		BaseImageLoader: BaseImageLoader{
			SupportedFormats: []FormatType{
				FormatJPEG,
				FormatPNG,
				FormatGIF,
				FormatBMP,
				FormatWEBP,
			},
		},
	}
}

// LoadImage loads a standard image format
func (l *StandardImageLoader) LoadImage(path string) (gocv.Mat, error) {
	return l.DefaultLoadImage(path)
}

// TiffImageLoader specializes in TIFF format loading
type TiffImageLoader struct {
	BaseImageLoader
}

// NewTiffImageLoader creates a new TIFF image loader
func NewTiffImageLoader() *TiffImageLoader {
	return &TiffImageLoader{
		BaseImageLoader: BaseImageLoader{
			SupportedFormats: []FormatType{FormatTIFF},
		},
	}
}

// LoadImage implements specialized loading for TIFF images
func (l *TiffImageLoader) LoadImage(path string) (gocv.Mat, error) {
	// Standard OpenCV loading works for most TIFF files
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if !img.Empty() {
		return img, nil
	}
	
	// If standard loading failed, could implement specialized TIFF processing here
	// For now, just return the empty mat with an error
	return img, newImageLoadError("failed to load TIFF image", path)
}

// SimpleRawImageLoader is a simplified loader for RAW formats
// This is a basic implementation that can be used when specialized loaders fail
type SimpleRawImageLoader struct {
	BaseImageLoader
}

// NewSimpleRawImageLoader creates a new basic RAW image loader
func NewSimpleRawImageLoader() *SimpleRawImageLoader {
	return &SimpleRawImageLoader{
		BaseImageLoader: BaseImageLoader{
			SupportedFormats: []FormatType{
				FormatRAW,
				FormatCR2,
				FormatCR3,
				FormatNEF,
				FormatARW,
				FormatDNG,
			},
		},
	}
}

// LoadImage provides a simple implementation for RAW image loading
func (l *SimpleRawImageLoader) LoadImage(path string) (gocv.Mat, error) {
	// Try to use dcraw to convert the RAW file to a format OpenCV can read
	tempPath := path + ".jpg"
	
	// Use a basic dcraw command that works with most RAW formats
	cmd := exec.Command("dcraw", "-c", "-b", "8", path)
	output, err := cmd.CombinedOutput()
	
	if err != nil {
		// Check if it's a missing tool error
		if strings.Contains(err.Error(), "executable file not found") {
			return gocv.NewMat(), fmt.Errorf("dcraw not found, please install it for RAW image support: %v", err)
		}
		logging.LogError("RAW conversion failed for %s: %v\nOutput: %s", path, err, string(output))
		return gocv.NewMat(), fmt.Errorf("dcraw conversion failed: %v", err)
	}
	
	// Try to load the converted image
	img := gocv.IMRead(tempPath, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, newImageLoadError("failed to load converted RAW image", path)
	}
	
	return img, nil
}

// checkExiftoolCommandAvailable checks if exiftool command is available
func checkExiftoolCommandAvailable() bool {
	// Check if exiftool is available for specialized CR3 loading
	_, err := exec.LookPath("exiftool")
	return err == nil
}