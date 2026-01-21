package imageprocessor

import (
	"bytes"
	"image"
	"os"
	"os/exec"

	"imagefinder/logging"
)

// Utility functions used across the various image loaders

// Check if exiftool is available on the system
func hasExiftool() bool {
	_, err := exec.LookPath("exiftool")
	return err == nil
}

// Check if dcraw is available on the system
func hasDcraw() bool {
	_, err := exec.LookPath("dcraw")
	return err == nil
}

// Try to load an image using Go's standard image packages
func tryGoImagePackages(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	return img, err
}

// fileExistsLegacy is kept for compatibility with existing code
// Use the function from loaders.go for new code
func fileExistsLegacy(path string) bool {
	return fileExists(path)
}

// hasFileContentLegacy is kept for compatibility with existing code
// Use hasFileContent from loaders.go for new code
func hasFileContentLegacy(path string) bool {
	return hasFileContent(path)
}

// Extract preview image with exiftool
func extractPreviewWithExiftool(path string, tempFilename string) error {
	if !hasExiftool() {
		return os.ErrNotExist
	}

	cmd := exec.Command("exiftool", "-b", "-PreviewImage", path)

	outFile, err := os.Create(tempFilename)
	if err != nil {
		logging.LogWarning("Failed to create temp file for exiftool preview: %v", err)
		return err
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		logging.LogWarning("exiftool preview extraction failed: %v, stderr: %s", err, stderr.String())
		return err
	}

	return nil
}

// Convert with dcraw using auto-brightness
func convertWithDcrawAutoBright(path string, tempFilename string) error {
	if !hasDcraw() {
		return os.ErrNotExist
	}

	cmd := exec.Command("dcraw", "-c", "-a", "-q", "3", path)

	outFile, err := os.Create(tempFilename)
	if err != nil {
		logging.LogWarning("Failed to create temp file for dcraw conversion: %v", err)
		return err
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		logging.LogWarning("dcraw auto-brightness conversion failed: %v, stderr: %s", err, stderr.String())
		return err
	}

	return nil
}

// Convert with dcraw using camera white balance
func convertWithDcrawCameraWB(path string, tempFilename string) error {
	if !hasDcraw() {
		return os.ErrNotExist
	}

	cmd := exec.Command("dcraw", "-c", "-w", "-q", "3", path)

	outFile, err := os.Create(tempFilename)
	if err != nil {
		logging.LogWarning("Failed to create temp file for dcraw conversion: %v", err)
		return err
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		logging.LogWarning("dcraw camera WB conversion failed: %v, stderr: %s", err, stderr.String())
		return err
	}

	return nil
}

// Convert with rawtherapee
func convertWithRawtherapee(path string, tempFilename string) error {
	_, err := exec.LookPath("rawtherapee-cli")
	if err != nil {
		return os.ErrNotExist
	}

	cmd := exec.Command("rawtherapee-cli", "-o", tempFilename, "-c", path)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		logging.LogWarning("rawtherapee conversion failed: %v, stderr: %s", err, stderr.String())
		return err
	}

	return nil
}
