package imageprocessor

import (
	"bytes"
	"image"
	"os"
	"os/exec"

	"imagefinder/logging"

	"gocv.io/x/gocv"
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

// Convert a Go standard library image to OpenCV Mat
func gocvMatFromGoImage(img image.Image) (gocv.Mat, error) {
	// This is a simplified version - you might need a more sophisticated conversion
	// depending on your requirements
	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	mat := gocv.NewMatWithSize(height, width, gocv.MatTypeCV8UC3)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			// Convert from 0-65535 to 0-255
			mat.SetUCharAt3(y, x, 0, uint8(b>>8))
			mat.SetUCharAt3(y, x, 1, uint8(g>>8))
			mat.SetUCharAt3(y, x, 2, uint8(r>>8))
		}
	}

	// Convert to grayscale to match expected output
	grayMat := gocv.NewMat()
	gocv.CvtColor(mat, &grayMat, gocv.ColorBGRToGray)
	mat.Close()

	return grayMat, nil
}

// Check if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Check if a file exists and has content
func fileHasContent(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
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
