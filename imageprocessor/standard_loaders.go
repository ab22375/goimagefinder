package imageprocessor

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
	// Use a temporary file for the converted image
	tempPath := filepath.Join(os.TempDir(), filepath.Base(path)+".jpg")
	
	// Try multiple approaches for RAW conversion, starting with extraction of embedded preview
	methods := []func(string, string) error{
		tryExiftoolPreviewExtraction,   // Try extracting preview with exiftool first
		tryDcrawConversionStandard,     // Standard dcraw conversion
		tryDcrawConversionWithOptions,  // Try dcraw with different options
		tryLibRawConversion,            // Try libraw-based conversion if available
	}
	
	// Try each method in order until one succeeds
	for _, method := range methods {
		err := method(path, tempPath)
		if err == nil {
			// Check if the file exists and has content
			if hasFileContent(tempPath) {
				// Try to load the converted image
				img := gocv.IMRead(tempPath, gocv.IMReadGrayScale)
				if !img.Empty() {
					// Clean up the temp file when done
					defer os.Remove(tempPath)
					return img, nil
				}
			}
			// If we get here, the conversion produced a file but OpenCV couldn't read it
			// Continue to the next method
			logging.LogWarning("Method produced output file for %s but OpenCV couldn't read it, trying next method", path)
		}
	}
	
	// If all methods failed, try direct loading as a last resort
	logging.LogWarning("All RAW conversion methods failed for %s, attempting direct load", path)
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if !img.Empty() {
		return img, nil
	}
	
	// If we get here, all methods failed
	return gocv.NewMat(), newImageLoadError("failed to load RAW image after trying all methods", path)
}

// tryExiftoolPreviewExtraction tries to extract embedded preview image with exiftool
func tryExiftoolPreviewExtraction(path, outputPath string) error {
	// Check if exiftool is available
	_, err := exec.LookPath("exiftool")
	if err != nil {
		return fmt.Errorf("exiftool not available: %v", err)
	}
	
	// First try to extract the largest preview image
	cmd := exec.Command("exiftool", "-b", "-LargestImagePreview", path)
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()
	
	cmd.Stdout = outFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	
	err = cmd.Run()
	if err != nil || !hasFileContent(outputPath) {
		// If the largest preview extraction failed, try the standard preview
		logging.LogWarning("Largest preview extraction failed for %s, trying standard preview", path)
		cmd = exec.Command("exiftool", "-b", "-PreviewImage", path)
		outFile, err = os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		defer outFile.Close()
		
		cmd.Stdout = outFile
		cmd.Stderr = &stderr
		
		err = cmd.Run()
		if err != nil || !hasFileContent(outputPath) {
			// If standard preview failed, try thumbnail
			logging.LogWarning("Standard preview extraction failed for %s, trying thumbnail", path)
			cmd = exec.Command("exiftool", "-b", "-ThumbnailImage", path)
			outFile, err = os.Create(outputPath)
			if err != nil {
				return fmt.Errorf("failed to create output file: %v", err)
			}
			defer outFile.Close()
			
			cmd.Stdout = outFile
			cmd.Stderr = &stderr
			
			err = cmd.Run()
			if err != nil || !hasFileContent(outputPath) {
				return fmt.Errorf("all exiftool preview extraction methods failed: %v", err)
			}
		}
	}
	
	return nil
}

// tryDcrawConversionStandard tries standard dcraw conversion
func tryDcrawConversionStandard(path, outputPath string) error {
	// Check if dcraw is available
	_, err := exec.LookPath("dcraw")
	if err != nil {
		return fmt.Errorf("dcraw not available: %v", err)
	}
	
	// Use dcraw to convert the RAW file directly to a temp file
	cmd := exec.Command("dcraw", "-c", "-b", "8", path)
	
	// Create the temporary file
	tempFile, err := os.Create(outputPath)
	if err != nil {
		logging.LogError("Failed to create temp file for RAW conversion: %v", err)
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	defer tempFile.Close()
	
	// Redirect dcraw output to the temp file
	cmd.Stdout = tempFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	
	err = cmd.Run()
	if err != nil {
		logging.LogWarning("Standard dcraw conversion failed for %s: %v\nStderr: %s", path, err, stderr.String())
		return err
	}
	
	return nil
}

// tryDcrawConversionWithOptions tries dcraw with different options
func tryDcrawConversionWithOptions(path, outputPath string) error {
	// Check if dcraw is available
	_, err := exec.LookPath("dcraw")
	if err != nil {
		return fmt.Errorf("dcraw not available: %v", err)
	}
	
	// Different sets of options to try
	optionSets := [][]string{
		{"-c", "-a", "-q", "0", path},               // Auto-brightness, low quality (faster)
		{"-c", "-w", "-q", "0", path},               // Camera white balance, low quality
		{"-c", "-w", "-a", "-q", "0", path},         // Camera WB + auto brightness, low quality
		{"-c", "-h", path},                          // Half-size, faster
		{"-c", "-o", "0", path},                     // Linear (no colorspace conversion)
		{"-e", path},                                // Extract embedded thumbnail
	}
	
	// Try each set of options
	for _, options := range optionSets {
		cmd := exec.Command("dcraw", options...)
		
		// Create the output file
		tempFile, err := os.Create(outputPath)
		if err != nil {
			logging.LogWarning("Failed to create output file: %v", err)
			continue
		}
		
		cmd.Stdout = tempFile
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		
		err = cmd.Run()
		tempFile.Close()
		
		if err == nil && hasFileContent(outputPath) {
			logging.LogInfo("Successfully converted RAW with options: %v", options)
			return nil
		}
		
		logging.LogWarning("dcraw with options %v failed: %v", options, err)
	}
	
	return fmt.Errorf("all dcraw option sets failed")
}

// tryLibRawConversion tries to use libraw or other available tools
func tryLibRawConversion(path, outputPath string) error {
	// Check for alternative RAW conversion tools
	tools := map[string][]string{
		"darktable-cli": {path, outputPath, "--width", "1024", "--height", "1024"},
		"rawtherapee-cli": {"-o", outputPath, "-c", path},
		"ufraw-batch": {"--out-type=jpg", "--output=" + outputPath, path},
	}
	
	for tool, args := range tools {
		_, err := exec.LookPath(tool)
		if err == nil {
			cmd := exec.Command(tool, args...)
			err = cmd.Run()
			if err == nil && hasFileContent(outputPath) {
				logging.LogInfo("Successfully converted RAW with %s", tool)
				return nil
			}
			logging.LogWarning("%s conversion failed: %v", tool, err)
		}
	}
	
	return fmt.Errorf("no alternative RAW conversion tools available or all failed")
}

// checkExiftoolCommandAvailable checks if exiftool command is available
func checkExiftoolCommandAvailable() bool {
	// Check if exiftool is available for specialized CR3 loading
	_, err := exec.LookPath("exiftool")
	return err == nil
}