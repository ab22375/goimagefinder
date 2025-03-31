// format_loader_implementations.go contains implementations for loading images of specific formats.
package imageprocessor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"imagefinder/logging"

	"gocv.io/x/gocv"
)

// LoadImage implementations for each format-specific loader

func (l *RAFImageLoader) LoadImage(path string) (gocv.Mat, error) {
	logging.LogInfo("Loading RAF image with specialized loader: %s", path)

	// Create a unique temporary filename for the converted image
	tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("raf_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempFilename) // Clean up temp file when done

	// Try different methods for RAF conversion in order of preference

	// First, try extracting the embedded preview image (often highest quality for Fuji)
	logging.LogInfo("Trying to extract RAF preview with exiftool")
	if err := extractPreviewWithExiftool(path, tempFilename); err == nil {
		if hasFileContent(tempFilename) {
			img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
			if !img.Empty() {
				logging.LogInfo("Successfully extracted RAF preview")
				return img, nil
			}
		}
	}

	// Try RAF-specific conversion
	logging.LogInfo("Trying RAF-specific conversion")
	if err := l.tryRAFSpecific(path, tempFilename); err == nil {
		if hasFileContent(tempFilename) {
			img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
			if !img.Empty() {
				logging.LogInfo("RAF-specific conversion successful")
				return img, nil
			}
		}
	}

	// Try with dcraw auto-brightness
	logging.LogInfo("Trying RAF with dcraw auto-brightness")
	if err := convertWithDcrawAutoBright(path, tempFilename); err == nil {
		if hasFileContent(tempFilename) {
			img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
			if !img.Empty() {
				logging.LogInfo("dcraw auto-brightness successful for RAF")
				return img, nil
			}
		}
	}

	// Try with dcraw camera white balance
	logging.LogInfo("Trying RAF with dcraw camera WB")
	if err := convertWithDcrawCameraWB(path, tempFilename); err == nil {
		if hasFileContent(tempFilename) {
			img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
			if !img.Empty() {
				logging.LogInfo("dcraw camera WB successful for RAF")
				return img, nil
			}
		}
	}

	// Try with rawtherapee
	logging.LogInfo("Trying RAF with rawtherapee")
	if err := convertWithRawtherapee(path, tempFilename); err == nil {
		if hasFileContent(tempFilename) {
			img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
			if !img.Empty() {
				logging.LogInfo("rawtherapee successful for RAF")
				return img, nil
			}
		}
	}

	// Custom fallback for Fuji RAF files
	logging.LogInfo("Trying special RAF fallback methods")

	// Try with Fuji-specific dcraw options
	cmd := exec.Command("dcraw", "-c", "-a", "-q", "0", path)
	tempFile := filepath.Join(l.TempDir, fmt.Sprintf("raf_fallback_%d.ppm", time.Now().UnixNano()))
	defer os.Remove(tempFile)

	outFile, err := os.Create(tempFile)
	if err == nil {
		cmd.Stdout = outFile
		err = cmd.Run()
		outFile.Close()

		if err == nil && hasFileContent(tempFile) {
			img := gocv.IMRead(tempFile, gocv.IMReadGrayScale)
			if !img.Empty() {
				logging.LogInfo("RAF special fallback successful")
				return img, nil
			}
		}
	}

	// Try direct load as absolute last resort
	logging.LogInfo("All RAF conversion methods failed, attempting direct load")
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		// Last resort - try to use standard Go image packages
		logging.LogInfo("Direct load failed, trying Go standard image packages")
		if goImg, err := tryGoImagePackages(path); err == nil {
			// Convert Go image to OpenCV Mat
			return gocvMatFromGoImage(goImg)
		}

		return img, fmt.Errorf("failed to load RAF image: %s (all conversion methods failed)", path)
	}

	return img, nil
}

func (l *NEFImageLoader) LoadImage(path string) (gocv.Mat, error) {
	// Create a unique temporary filename for the converted image
	tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("nef_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempFilename) // Clean up temp file when done

	// Try different methods for NEF conversion in order of preference
	methods := []func(string, string) error{
		extractPreviewWithExiftool, // Extract embedded preview
		l.tryNEFSpecific,           // NEF-specific conversion
		convertWithDcrawAutoBright, // Use dcraw with auto-brightness
		convertWithDcrawCameraWB,   // Use dcraw with camera white balance
		convertWithRawtherapee,     // Use rawtherapee as fallback
	}

	for _, method := range methods {
		err := method(path, tempFilename)
		if err == nil {
			// Check if file exists and has content
			if hasFileContent(tempFilename) {
				img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
				if !img.Empty() {
					return img, nil
				}
			}
		}
	}

	// If all methods fail, try direct load (unlikely to work)
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		// Try with standard Go image packages as last resort
		if goImg, err := tryGoImagePackages(path); err == nil {
			return gocvMatFromGoImage(goImg)
		}

		return img, fmt.Errorf("failed to load NEF image: %s (all conversion methods failed)", path)
	}

	return img, nil
}

func (l *ARWImageLoader) LoadImage(path string) (gocv.Mat, error) {
	// Create a unique temporary filename for the converted image
	tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("arw_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempFilename) // Clean up temp file when done

	// Try different methods for ARW conversion in order of preference
	methods := []func(string, string) error{
		extractPreviewWithExiftool, // Extract embedded preview
		l.tryARWSpecific,           // ARW-specific conversion
		convertWithDcrawAutoBright, // Use dcraw with auto-brightness
		convertWithDcrawCameraWB,   // Use dcraw with camera white balance
		convertWithRawtherapee,     // Use rawtherapee as fallback
	}

	for _, method := range methods {
		err := method(path, tempFilename)
		if err == nil {
			// Check if file exists and has content
			if hasFileContent(tempFilename) {
				img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
				if !img.Empty() {
					return img, nil
				}
			}
		}
	}

	// If all methods fail, try direct load (unlikely to work)
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		// Try with standard Go image packages as last resort
		if goImg, err := tryGoImagePackages(path); err == nil {
			return gocvMatFromGoImage(goImg)
		}

		return img, fmt.Errorf("failed to load ARW image: %s (all conversion methods failed)", path)
	}

	return img, nil
}

func (l *CR2ImageLoader) LoadImage(path string) (gocv.Mat, error) {
	// Create a unique temporary filename for the converted image
	tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("cr2_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempFilename) // Clean up temp file when done

	// Try different methods for CR2 conversion in order of preference
	methods := []func(string, string) error{
		extractPreviewWithExiftool, // Extract embedded preview
		l.tryCR2Specific,           // CR2-specific conversion
		convertWithDcrawAutoBright, // Use dcraw with auto-brightness
		convertWithDcrawCameraWB,   // Use dcraw with camera white balance
		convertWithRawtherapee,     // Use rawtherapee as fallback
	}

	for _, method := range methods {
		err := method(path, tempFilename)
		if err == nil {
			// Check if file exists and has content
			if hasFileContent(tempFilename) {
				img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
				if !img.Empty() {
					return img, nil
				}
			}
		}
	}

	// If all methods fail, try direct load (unlikely to work)
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		// Try with standard Go image packages as last resort
		if goImg, err := tryGoImagePackages(path); err == nil {
			return gocvMatFromGoImage(goImg)
		}

		return img, fmt.Errorf("failed to load CR2 image: %s (all conversion methods failed)", path)
	}

	return img, nil
}

func (l *CR3ImageLoader) LoadImage(path string) (gocv.Mat, error) {
	// Create a unique temporary filename for the converted image
	tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("cr3_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempFilename) // Clean up temp file when done

	// CR3 files often need special handling with specific tools
	methods := []func(string, string) error{
		l.extractCR3LargePreview, // Extract largest preview image
		l.extractCR3Preview,      // Extract standard preview
		l.tryCR3WithExiftool,     // Try other exiftool methods
		convertWithRawtherapee,   // Use rawtherapee as fallback
	}

	for _, method := range methods {
		err := method(path, tempFilename)
		if err == nil {
			// Check if file exists and has content
			if hasFileContent(tempFilename) {
				img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
				if !img.Empty() {
					return img, nil
				}
			}
		}
	}

	// If all methods fail, try direct load (unlikely to work)
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		// Try with standard Go image packages as last resort
		if goImg, err := tryGoImagePackages(path); err == nil {
			return gocvMatFromGoImage(goImg)
		}

		return img, fmt.Errorf("failed to load CR3 image: %s (all conversion methods failed)", path)
	}

	return img, nil
}

func (l *DNGImageLoader) LoadImage(path string) (gocv.Mat, error) {
	// Create a unique temporary filename for the converted image
	tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("dng_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempFilename) // Clean up temp file when done

	// Try different methods for DNG conversion in order of preference
	methods := []func(string, string) error{
		extractPreviewWithExiftool, // Extract embedded preview
		convertWithDcrawAutoBright, // Use dcraw with auto-brightness
		convertWithDcrawCameraWB,   // Use dcraw with camera white balance
		convertWithRawtherapee,     // Use rawtherapee as fallback
	}

	for _, method := range methods {
		err := method(path, tempFilename)
		if err == nil {
			// Check if file exists and has content
			if hasFileContent(tempFilename) {
				img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
				if !img.Empty() {
					return img, nil
				}
			}
		}
	}

	// If all methods fail, try direct load (unlikely to work)
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		// Try with standard Go image packages as last resort
		if goImg, err := tryGoImagePackages(path); err == nil {
			return gocvMatFromGoImage(goImg)
		}

		return img, fmt.Errorf("failed to load DNG image: %s (all conversion methods failed)", path)
	}

	return img, nil
}
