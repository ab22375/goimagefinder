package imageprocessor

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/disintegration/imaging"
	"imagefinder/logging"
)

// RawImageLoader handles RAW camera formats
type RawImageLoader struct {
	TempDir string
}

// NewRawImageLoader creates a new loader for RAW files
func NewRawImageLoader() *RawImageLoader {
	tempDir := os.TempDir()
	return &RawImageLoader{
		TempDir: tempDir,
	}
}

func (l *RawImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	// Explicitly include all requested formats: DNG, RAF, ARW, NEF, CR2, CR3
	rawFormats := []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf"}
	for _, format := range rawFormats {
		if ext == format {
			// Check if file exists and is readable
			_, err := os.Stat(path)
			return err == nil
		}
	}
	return false
}

func (l *RawImageLoader) LoadImage(path string) (image.Image, error) {
	logging.LogInfo("Loading RAW image: %s", path)

	// Create a unique temporary filename for the converted image
	tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("raw_conv_%d.tiff", time.Now().UnixNano()))
	defer os.Remove(tempFilename) // Clean up temp file when done

	// Check if it's a CR3 file specifically
	if strings.ToLower(filepath.Ext(path)) == ".cr3" {
		logging.LogInfo("Detected CR3 format, using specialized loader")
		if img, ok := l.tryCR3(path, tempFilename); ok {
			return img, nil
		}
	}

	// Check for RAF file to use specialized RAF loader
	if strings.ToLower(filepath.Ext(path)) == ".raf" {
		logging.LogInfo("Detected RAF format, using specialized RAF loader")
		rafLoader := NewRAFImageLoader()
		return rafLoader.LoadImage(path)
	}

	// First try with dcraw
	logging.LogInfo("Trying to load RAW with dcraw")
	if img, ok := l.tryDcraw(path, tempFilename); ok {
		return img, nil
	}

	// If dcraw fails, try libraw fallback
	logging.LogInfo("Trying to load RAW with libraw")
	if img, ok := l.tryLibRaw(path, tempFilename); ok {
		return img, nil
	}

	// Check if exiftool is available and try extracting preview
	logging.LogInfo("Trying to extract preview with exiftool")
	if hasExiftool() {
		if img, ok := l.tryExtractPreview(path, tempFilename); ok {
			return img, nil
		}
	}

	// Final fallback - try direct load (unlikely to work for most RAW formats)
	logging.LogInfo("All conversion methods failed, attempting direct load as last resort")
	img, err := imaging.Open(path)
	if err == nil {
		return imaging.Grayscale(img), nil
	}

	// Last resort - try to use standard Go image packages which might support some RAW formats
	logging.LogInfo("Direct load failed, trying Go standard image packages")
	if goImg, err := tryGoImagePackages(path); err == nil {
		return imaging.Grayscale(goImg), nil
	}

	return nil, fmt.Errorf("failed to load RAW image: %s (all conversion methods failed)", path)
}

// Try to extract preview image with exiftool
func (l *RawImageLoader) tryExtractPreview(path string, tempFilename string) (image.Image, bool) {
	cmd := exec.Command("exiftool", "-b", "-PreviewImage", path)

	outFile, err := os.Create(tempFilename)
	if err != nil {
		logging.LogWarning("Failed to create temp file for exiftool preview: %v", err)
		return nil, false
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	err = cmd.Run()

	if err == nil {
		// Check if file has content
		info, err := os.Stat(tempFilename)
		if err == nil && info.Size() > 0 {
			img, err := imaging.Open(tempFilename)
			if err == nil {
				return imaging.Grayscale(img), true
			}
		}
	}

	return nil, false
}

func (l *RawImageLoader) tryDcraw(path string, tempFilename string) (image.Image, bool) {
	// Check if dcraw is available
	if !hasDcraw() {
		logging.LogWarning("dcraw not found on system, skipping dcraw conversion")
		return nil, false
	}

	// Convert RAW to TIFF using dcraw
	// -T = output TIFF
	// -c = output to stdout (we redirect to file)
	// -w = use camera white balance
	// -q 3 = use high-quality interpolation
	cmd := exec.Command("dcraw", "-T", "-c", "-w", "-q", "3", path)

	// Create the output file
	outFile, err := os.Create(tempFilename)
	if err != nil {
		logging.LogWarning("Failed to create temp file for dcraw conversion: %v", err)
		return nil, false
	}
	defer outFile.Close()

	// Set stdout to our file
	cmd.Stdout = outFile

	// Capture stderr for error reporting
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()
	if err != nil {
		logging.LogWarning("dcraw conversion failed: %v, stderr: %s", err, stderr.String())
		return nil, false
	}

	// Load the converted TIFF
	img, err := imaging.Open(tempFilename)
	if err != nil {
		return nil, false
	}

	return imaging.Grayscale(img), true
}

func (l *RawImageLoader) tryLibRaw(path string, tempFilename string) (image.Image, bool) {
	// Try with rawtherapee-cli as an alternative for RAW conversion
	// Example: rawtherapee-cli -o /tmp/output.jpg -c /path/to/raw/file.CR2
	cmd := exec.Command("rawtherapee-cli", "-o", tempFilename, "-c", path)

	// Capture stderr for error reporting
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		logging.LogWarning("rawtherapee conversion failed: %v, stderr: %s", err, stderr.String())
		return nil, false
	}

	img, err := imaging.Open(tempFilename)
	if err != nil {
		return nil, false
	}

	return imaging.Grayscale(img), true
}

func (l *RawImageLoader) tryCR3(path string, tempFilename string) (image.Image, bool) {
	// CR3 files often need different handling

	// Try with exiftool to extract preview image (often works for CR3)
	cmd := exec.Command("exiftool", "-b", "-PreviewImage", path)

	outFile, err := os.Create(tempFilename)
	if err != nil {
		logging.LogWarning("Failed to create temp file for CR3 conversion: %v", err)
		return nil, false
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	err = cmd.Run()

	if err == nil {
		// Check if file has content
		info, err := os.Stat(tempFilename)
		if err == nil && info.Size() > 0 {
			img, err := imaging.Open(tempFilename)
			if err == nil {
				return imaging.Grayscale(img), true
			}
		}
	}

	// If extracting preview failed, try alternative approach using libraw
	cmd = exec.Command("libraw_unpack", "-O", tempFilename, path)
	err = cmd.Run()
	if err == nil {
		img, err := imaging.Open(tempFilename)
		if err == nil {
			return imaging.Grayscale(img), true
		}
	}

	return nil, false
}
