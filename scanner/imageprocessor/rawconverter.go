package imageprocessor

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
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

// Convert using dcraw with auto-brightness, which often matches camera output
func (r *RawImageConverter) convertWithDcrawAutoBright(path, outputPath string) error {
	// -w = use camera white balance
	// -a = auto-brightness (mimics camera)
	// -q 3 = high-quality interpolation
	// -O = output to specified file
	cmd := exec.Command("dcraw", "-w", "-a", "-q", "3", "-O", outputPath, path)
	stderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dcraw failed: %v, stderr: %s", err, string(stderr))
	}
	return nil
}

// Convert using dcraw with camera white balance, no auto-brightness
func (r *RawImageConverter) convertWithDcrawCameraWB(path, outputPath string) error {
	// -w = use camera white balance
	// -q 3 = high-quality interpolation
	// -O = output to specified file
	cmd := exec.Command("dcraw", "-w", "-q", "3", "-O", outputPath, path)
	stderr, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("dcraw (camera wb) failed: %v, stderr: %s", err, string(stderr))
	}
	return nil
}

// convertCR3WithExiftool specialized function for CR3 files which often need special handling
func (r *RawImageConverter) convertCR3WithExiftool(path, outputPath string) error {
	// CR3 files often have multiple preview images
	// Try extracting the largest preview image
	cmd := exec.Command("exiftool", "-b", "-LargePreviewImage", "-w", outputPath, path)
	stderr, err := cmd.CombinedOutput()
	if err != nil && r.DebugMode {
		logging.DebugLog("Exiftool error for CR3: %v, stderr: %s", err, string(stderr))
	}

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
		stderr, err := cmd.CombinedOutput()
		if err != nil && r.DebugMode {
			logging.DebugLog("Exiftool error for tag %s: %v, stderr: %s", tag, err, string(stderr))
		}

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

// ConvertAndLoad converts a RAW file to JPG and loads it for hashing
func (r *RawImageConverter) ConvertAndLoad(path string) (gocv.Mat, error) {
	tempDir := os.TempDir()
	tempJpg := filepath.Join(tempDir, fmt.Sprintf("std_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempJpg) // Clean up temp file when done

	methods := []struct {
		name string
		fn   func(string, string) error
	}{
		{"extractRAFPreviewWithExiftool", r.extractRAFPreviewWithExiftool}, // Add this new RAF-specific method first
		{"extractPreviewWithExiftool", r.extractPreviewWithExiftool},
		{"convertWithDcrawAutoBright", r.convertWithDcrawAutoBright},
		{"convertWithDcrawCameraWB", r.convertWithDcrawCameraWB},
		{"convertWithRawTherapee", r.convertWithRawTherapee},
		{"convertWithLibRaw", r.convertWithLibRaw},
		{"convertWithJPGFallback", r.convertWithJPGFallback},
	}

	var lastError error
	for _, method := range methods {
		if r.DebugMode {
			logging.DebugLog("Trying RAW conversion method: %s for %s", method.name, path)
		}

		err := method.fn(path, tempJpg)
		if err != nil {
			if r.DebugMode {
				logging.DebugLog("Method %s failed: %v", method.name, err)
			}
			lastError = err
			continue
		}

		// Check if the file was created successfully
		fileInfo, err := os.Stat(tempJpg)
		if err != nil {
			if r.DebugMode {
				logging.DebugLog("Error stating temp file %s: %v", tempJpg, err)
			}
			continue
		}

		if fileInfo.Size() == 0 {
			if r.DebugMode {
				logging.DebugLog("Output file %s is empty", tempJpg)
			}
			continue
		}

		// Load the standard JPG representation
		img := gocv.IMRead(tempJpg, gocv.IMReadGrayScale)
		if img.Empty() {
			if r.DebugMode {
				logging.DebugLog("Converted image could not be read using %s", method.name)
			}
			continue
		}

		if r.DebugMode {
			logging.DebugLog("Successfully converted RAW to JPG using %s: %s", method.name, path)
		}
		return img, nil
	}

	// Add a direct loading attempt as a last resort
	if r.DebugMode {
		logging.DebugLog("All conversion methods failed, trying direct loading: %s", path)
	}

	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if !img.Empty() {
		if r.DebugMode {
			logging.DebugLog("Successfully loaded RAW directly: %s", path)
		}
		return img, nil
	}

	// If all methods fail, return the error
	return gocv.NewMat(), fmt.Errorf("failed to convert RAW to JPG: %v", lastError)
}

// Additional conversion methods

// convertWithRawTherapee uses RawTherapee if available
func (r *RawImageConverter) convertWithRawTherapee(path, outputPath string) error {
	cmd := exec.Command("rawtherapee-cli", "-o", outputPath, "-c", path)
	output, err := cmd.CombinedOutput()
	if err != nil && r.DebugMode {
		logging.DebugLog("RawTherapee error: %v, output: %s", err, string(output))
	}
	return err
}

// convertWithLibRaw uses LibRaw's simple conversion tool if available
func (r *RawImageConverter) convertWithLibRaw(path, outputPath string) error {
	cmd := exec.Command("dcraw_emu", "-e", "-c", path)
	jpgData, err := cmd.Output()
	if err != nil {
		stderr, _ := cmd.StderrPipe()
		var errOut []byte
		if stderr != nil {
			errOut, _ = os.ReadFile(filepath.Base(path) + ".stderr")
		}
		return fmt.Errorf("libraw error: %v, stderr: %s", err, string(errOut))
	}

	return os.WriteFile(outputPath, jpgData, 0644)
}

// convertWithJPGFallback tries to read embedded JPG data directly
func (r *RawImageConverter) convertWithJPGFallback(path, outputPath string) error {
	// Try to extract any embedded JPG data using exiftool with various tag options
	tags := []string{
		"JpgFromRaw",
		"PreviewImage",
		"OtherImage",
		"ThumbnailImage",
		"FullPreviewImage",
		"EmbeddedImage",
	}

	for _, tag := range tags {
		cmd := exec.Command("exiftool", "-b", "-"+tag, "-w", outputPath, path)
		stderr, err := cmd.CombinedOutput()
		if err != nil && r.DebugMode {
			logging.DebugLog("Exiftool fallback error for tag %s: %v, stderr: %s", tag, err, string(stderr))
		}

		if err == nil {
			// Check if file was created and has content
			info, statErr := os.Stat(outputPath)
			if statErr == nil && info.Size() > 0 {
				return nil
			}
		}
	}

	return fmt.Errorf("no embedded JPG data found")
}

// Update this function
func (r *RawImageConverter) extractPreviewWithExiftool(path, outputPath string) error {
	// Make sure outputPath is properly created
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	// Use -o instead of -w to directly specify output file
	cmd := exec.Command("exiftool", "-b", "-PreviewImage", "-o", outputPath, path)
	output, err := cmd.CombinedOutput()
	if err != nil && r.DebugMode {
		logging.DebugLog("Exiftool error: %v, output: %s", err, string(output))
	}

	// Verify file exists and has content
	if err == nil {
		info, statErr := os.Stat(outputPath)
		if statErr != nil {
			return fmt.Errorf("failed to stat output file: %v", statErr)
		}
		if info.Size() == 0 {
			return fmt.Errorf("extracted preview is empty")
		}
	}

	return err
}

// Add to your rawconverter.go or ARW-specific file
func extractSonyARWPreview(path, outputPath string) error {
	// Sony specific tags to try, in order of preference
	tags := []string{
		"JpgFromRaw",
		"PreviewImage",
		"OtherImage",
		"ThumbnailImage",
		"LargePreviewImage",
	}

	for _, tag := range tags {
		cmd := exec.Command("exiftool", "-b", "-"+tag, "-o", outputPath, path)
		stderr, err := cmd.CombinedOutput()
		if err != nil {
			logging.DebugLog("Sony ARW preview extraction error: %v, stderr: %s", err, string(stderr))
			continue
		}

		// Check if file exists and has content
		if fi, err := os.Stat(outputPath); err == nil && fi.Size() > 0 {
			return nil
		}
	}

	return fmt.Errorf("no usable embedded preview found")
}

// Add this method to rawconverter.go
func (r *RawImageConverter) extractRAFPreviewWithExiftool(path, outputPath string) error {
	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	// Run exiftool and pipe output directly to file
	cmd := exec.Command("exiftool", "-b", "-PreviewImage", path)
	cmd.Stdout = outFile
	stderr, err := cmd.StderrPipe()

	err = cmd.Run()
	if err != nil {
		var errOut []byte
		if stderr != nil {
			errOut, _ = io.ReadAll(stderr)
		}
		return fmt.Errorf("exiftool error: %v, stderr: %s", err, string(errOut))
	}

	// Verify file has content
	if fi, err := os.Stat(outputPath); err != nil || fi.Size() == 0 {
		return fmt.Errorf("extracted preview is empty")
	}

	return nil
}
