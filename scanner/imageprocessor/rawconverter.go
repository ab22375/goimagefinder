package imageprocessor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"imagefinder/logging"

	"gocv.io/x/gocv"
)

// RawImageConverter handles RAW image conversion
type RawImageConverter struct {
	DebugMode bool
}

// NewRawImageConverter creates a new RawImageConverter instance
func NewRawImageConverter(debugMode bool) *RawImageConverter {
	return &RawImageConverter{
		DebugMode: debugMode,
	}
}

// ConvertAndLoad converts a RAW file to JPG and loads it for hashing
func (r *RawImageConverter) ConvertAndLoad(path string) (gocv.Mat, error) {
	tempDir := os.TempDir()
	tempJpg := filepath.Join(tempDir, fmt.Sprintf("std_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempJpg) // Clean up temp file when done

	// Special handling for CR3 files
	if strings.ToLower(filepath.Ext(path)) == ".cr3" {
		err := r.convertCR3WithExiftool(path, tempJpg)
		if err == nil {
			// Check if the file was created successfully
			_, statErr := os.Stat(tempJpg)
			if statErr == nil {
				// Load the standard JPG representation
				img := gocv.IMRead(tempJpg, gocv.IMReadGrayScale)
				if !img.Empty() {
					return img, nil
				}
			}
		}
	}

	// Try different conversion methods in order of preference
	methods := []struct {
		name string
		fn   func(string, string) error
	}{
		{"extractPreviewWithExiftool", r.extractPreviewWithExiftool}, // Extract embedded preview (best match for camera JPGs)
		{"convertWithDcrawAutoBright", r.convertWithDcrawAutoBright}, // Use dcraw with auto-brightness
		{"convertWithDcrawCameraWB", r.convertWithDcrawCameraWB},     // Use dcraw with camera white balance
	}

	var lastError error
	for _, method := range methods {
		err := method.fn(path, tempJpg)
		if err == nil {
			// Check if the file was created successfully
			_, err = os.Stat(tempJpg)
			if err == nil {
				// Load the standard JPG representation
				img := gocv.IMRead(tempJpg, gocv.IMReadGrayScale)
				if !img.Empty() {
					if r.DebugMode {
						logging.DebugLog("Successfully converted RAW to JPG using %s: %s", method.name, path)
					}
					return img, nil
				}
			}
		}
		lastError = err
	}

	// If all methods fail, return the error
	return gocv.NewMat(), fmt.Errorf("failed to convert RAW to JPG: %v", lastError)
}

// Extract the embedded preview JPEG from the RAW file using exiftool
func (r *RawImageConverter) extractPreviewWithExiftool(path, outputPath string) error {
	// Use exiftool to extract the preview image
	// -b = output in binary mode
	// -PreviewImage = extract the preview image
	cmd := exec.Command("exiftool", "-b", "-PreviewImage", "-w", outputPath, path)
	return cmd.Run()
}

// Convert using dcraw with auto-brightness, which often matches camera output
func (r *RawImageConverter) convertWithDcrawAutoBright(path, outputPath string) error {
	// -w = use camera white balance
	// -a = auto-brightness (mimics camera)
	// -q 3 = high-quality interpolation
	// -O = output to specified file
	cmd := exec.Command("dcraw", "-w", "-a", "-q", "3", "-O", outputPath, path)
	return cmd.Run()
}

// Convert using dcraw with camera white balance, no auto-brightness
func (r *RawImageConverter) convertWithDcrawCameraWB(path, outputPath string) error {
	// -w = use camera white balance
	// -q 3 = high-quality interpolation
	// -O = output to specified file
	cmd := exec.Command("dcraw", "-w", "-q", "3", "-O", outputPath, path)
	return cmd.Run()
}

// convertCR3WithExiftool specialized function for CR3 files which often need special handling
func (r *RawImageConverter) convertCR3WithExiftool(path, outputPath string) error {
	// CR3 files often have multiple preview images
	// Try extracting the largest preview image
	cmd := exec.Command("exiftool", "-b", "-LargePreviewImage", "-w", outputPath, path)
	err := cmd.Run()

	// Check if the output file was created and has content
	if err == nil {
		info, statErr := os.Stat(outputPath)
		if statErr == nil && info.Size() > 0 {
			return nil
		}
	}

	// Try alternative tags for preview images
	tags := []string{
		"PreviewImage",
		"OtherImage",
		"ThumbnailImage",
		"FullPreviewImage",
	}

	for _, tag := range tags {
		cmd := exec.Command("exiftool", "-b", "-"+tag, "-w", outputPath, path)
		err := cmd.Run()

		// Check if successful
		if err == nil {
			info, statErr := os.Stat(outputPath)
			if statErr == nil && info.Size() > 0 {
				return nil
			}
		}
	}

	return fmt.Errorf("could not extract any preview image from CR3 file")
}
