package imageprocessor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"imagefinder/logging"

	"github.com/barasher/go-exiftool"

	"gocv.io/x/gocv"
)

// CR3ExiftoolLoader is a specialized loader for CR3 files using go-exiftool
type CR3ExiftoolLoader struct {
	TempDir string
}

// NewCR3ExiftoolLoader creates a new CR3 loader that uses go-exiftool
func NewCR3ExiftoolLoader() *CR3ExiftoolLoader {
	tempDir := os.TempDir()
	return &CR3ExiftoolLoader{
		TempDir: tempDir,
	}
}

func (l *CR3ExiftoolLoader) CanLoad(path string) bool {
	ext := filepath.Ext(path)
	return ext == ".cr3" && fileExists(path)
}

func (l *CR3ExiftoolLoader) LoadImage(path string) (gocv.Mat, error) {
	logging.LogInfo("Loading CR3 image with go-exiftool: %s", path)

	// Initialize exiftool
	et, err := exiftool.NewExiftool()
	if err != nil {
		logging.LogError("Failed to initialize exiftool: %v", err)
		return gocv.NewMat(), err
	}
	defer et.Close()

	// Extract metadata
	fileInfos := et.ExtractMetadata(path)
	if len(fileInfos) == 0 {
		return gocv.NewMat(), fmt.Errorf("no metadata extracted")
	}

	fileInfo := fileInfos[0]
	if fileInfo.Err != nil {
		logging.LogError("Error extracting metadata: %v", fileInfo.Err)
		return gocv.NewMat(), fileInfo.Err
	}

	// Get available preview tags
	previewTags := []string{
		"LargestImagePreview",
		"PreviewImage",
		"OtherImage",
		"ThumbnailImage",
		"JpgFromRaw",
	}

	// Try to extract preview images and return the first successful one
	for _, tag := range previewTags {
		tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("cr3_preview_%s_%s.jpg",
			filepath.Base(path), tag))

		// Try to extract preview using exiftool command line
		// Since go-exiftool doesn't directly support binary extraction
		if err := l.extractPreview(path, tempFilename, tag); err == nil {
			img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
			os.Remove(tempFilename) // Clean up

			if !img.Empty() {
				logging.LogInfo("Successfully extracted %s from CR3", tag)
				return img, nil
			}
		}
	}

	// If direct extractions fail, try additional approach
	tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("cr3_alt_%s.jpg",
		filepath.Base(path)))

	// Try extracting embedded preview with different exiftool command
	if err := extractUsingExiftoolCommand(path, tempFilename); err == nil {
		img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
		os.Remove(tempFilename) // Clean up

		if !img.Empty() {
			logging.LogInfo("Successfully extracted CR3 preview using alternate method")
			return img, nil
		}
	}

	return gocv.NewMat(), fmt.Errorf("failed to extract any preview from CR3 file")
}

// extractPreview extracts a specific preview from a CR3 file
func (l *CR3ExiftoolLoader) extractPreview(path, outputPath, tag string) error {
	// Use exiftool command directly since go-exiftool doesn't support binary extraction
	cmd := exec.Command("exiftool", "-b", "-"+tag, "-w", outputPath, path)
	err := cmd.Run()
	return err
}

// extractUsingExiftoolCommand tries multiple exiftool commands to extract previews
func extractUsingExiftoolCommand(path, outputPath string) error {
	// Alternative exiftool command variations
	commands := [][]string{
		{"-b", "-PreviewImage", path},
		{"-b", "-JpgFromRaw", path},
		{"-b", "-ThumbnailImage", path},
		{"-b", "-LargestImagePreview", path},
		// Special command for CR3
		{"-b", "-ifd0:all", path},
	}

	for _, args := range commands {
		if err := runExiftoolExtract(args, outputPath); err == nil {
			// Verify it's a valid image
			if validateImageFile(outputPath) {
				return nil
			}
		}
	}

	return fmt.Errorf("all exiftool extraction methods failed")
}

// runExiftoolExtract runs an exiftool command and saves output to a file
func runExiftoolExtract(args []string, outputPath string) error {
	// Use exec.Command instead of exiftool.Command
	cmd := exec.Command("exiftool", args...)
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// Save binary output to file
	return os.WriteFile(outputPath, output, 0644)
}

// validateImageFile checks if a file is a valid image
func validateImageFile(path string) bool {
	// Check if file exists and has content
	if !hasFileContent(path) {
		return false
	}

	// Try to read with OpenCV
	img := gocv.IMRead(path, gocv.IMReadUnchanged)
	defer img.Close()

	return !img.Empty()
}

// Check if go-exiftool can be imported and used
func checkGoExiftoolAvailable() bool {
	_, err := exiftool.NewExiftool()
	return err == nil
}
