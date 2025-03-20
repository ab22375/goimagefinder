package imageprocessor

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"imagefinder/logging"
)

// Format-specific loader types
// Each handles a specific RAW camera format

// RAFImageLoader handles Fujifilm RAF format
type RAFImageLoader struct {
	TempDir string
}

// NEFImageLoader handles Nikon NEF format
type NEFImageLoader struct {
	TempDir string
}

// ARWImageLoader handles Sony ARW format
type ARWImageLoader struct {
	TempDir string
}

// CR2ImageLoader handles Canon CR2 format
type CR2ImageLoader struct {
	TempDir string
}

// CR3ImageLoader handles Canon CR3 format
type CR3ImageLoader struct {
	TempDir string
}

// DNGImageLoader handles Adobe DNG format
type DNGImageLoader struct {
	TempDir string
}

// Factory functions for each format-specific loader

// NewRAFImageLoader creates a new loader for RAF files
func NewRAFImageLoader() *RAFImageLoader {
	tempDir := os.TempDir()
	return &RAFImageLoader{
		TempDir: tempDir,
	}
}

// NewNEFImageLoader creates a new loader for NEF files
func NewNEFImageLoader() *NEFImageLoader {
	tempDir := os.TempDir()
	return &NEFImageLoader{
		TempDir: tempDir,
	}
}

// NewARWImageLoader creates a new loader for ARW files
func NewARWImageLoader() *ARWImageLoader {
	tempDir := os.TempDir()
	return &ARWImageLoader{
		TempDir: tempDir,
	}
}

// NewCR2ImageLoader creates a new loader for CR2 files
func NewCR2ImageLoader() *CR2ImageLoader {
	tempDir := os.TempDir()
	return &CR2ImageLoader{
		TempDir: tempDir,
	}
}

// NewCR3ImageLoader creates a new loader for CR3 files
func NewCR3ImageLoader() *CR3ImageLoader {
	tempDir := os.TempDir()
	return &CR3ImageLoader{
		TempDir: tempDir,
	}
}

// NewDNGImageLoader creates a new loader for DNG files
func NewDNGImageLoader() *DNGImageLoader {
	tempDir := os.TempDir()
	return &DNGImageLoader{
		TempDir: tempDir,
	}
}

// CanLoad implementations for each format-specific loader

func (l *RAFImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".raf" && fileExists(path)
}

func (l *NEFImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".nef" && fileExists(path)
}

func (l *ARWImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".arw" && fileExists(path)
}

func (l *CR2ImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".cr2" && fileExists(path)
}

func (l *CR3ImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".cr3" && fileExists(path)
}

func (l *DNGImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".dng" && fileExists(path)
}

// Format-specific loader method implementations

// ARW-specific conversion method
func (l *ARWImageLoader) tryARWSpecific(path string, tempFilename string) error {
	// Sony ARW files sometimes need special handling
	// Try with specific ARW options for dcraw
	cmd := exec.Command("dcraw", "-w", "-a", "-q", "3", "-j", "-O", tempFilename, path)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		logging.LogWarning("ARW-specific conversion failed: %v, stderr: %s", err, stderr.String())
		return err
	}

	return nil
}

// RAF-specific conversion method
func (l *RAFImageLoader) tryRAFSpecific(path string, tempFilename string) error {
	// Fujifilm X-Trans sensor RAF files sometimes need special handling
	// Try X-Trans specific parameters for dcraw
	cmd := exec.Command("dcraw", "-w", "-a", "-q", "3", "-f", "-o", "5", "-O", tempFilename, path)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		logging.LogWarning("RAF-specific conversion failed: %v, stderr: %s", err, stderr.String())
		return err
	}

	return nil
}

// NEF-specific conversion method
func (l *NEFImageLoader) tryNEFSpecific(path string, tempFilename string) error {
	// Nikon NEF files sometimes need special handling for different sensor types
	// Try with specific NEF options for dcraw
	cmd := exec.Command("dcraw", "-w", "-a", "-q", "3", "-b", "2.0", "-O", tempFilename, path)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		logging.LogWarning("NEF-specific conversion failed: %v, stderr: %s", err, stderr.String())
		return err
	}

	return nil
}

// CR2-specific conversion method
func (l *CR2ImageLoader) tryCR2Specific(path string, tempFilename string) error {
	// Canon CR2 files can sometimes need specific handling
	// Try with specific CR2 options for dcraw
	cmd := exec.Command("dcraw", "-w", "-a", "-q", "3", "-H", "1", "-O", tempFilename, path)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		logging.LogWarning("CR2-specific conversion failed: %v, stderr: %s", err, stderr.String())
		return err
	}

	return nil
}

// CR3 special methods
func (l *CR3ImageLoader) extractCR3LargePreview(path string, tempFilename string) error {
	// Try to extract the largest preview available
	cmd := exec.Command("exiftool", "-b", "-LargestImagePreview", path)

	outFile, err := os.Create(tempFilename)
	if err != nil {
		logging.LogWarning("Failed to create temp file for CR3 large preview: %v", err)
		return err
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	err = cmd.Run()

	if err != nil {
		logging.LogWarning("Failed to extract CR3 large preview: %v", err)
		return err
	}

	return nil
}

func (l *CR3ImageLoader) extractCR3Preview(path string, tempFilename string) error {
	// Try to extract standard preview
	cmd := exec.Command("exiftool", "-b", "-PreviewImage", path)

	outFile, err := os.Create(tempFilename)
	if err != nil {
		logging.LogWarning("Failed to create temp file for CR3 preview: %v", err)
		return err
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	err = cmd.Run()

	if err != nil {
		logging.LogWarning("Failed to extract CR3 preview: %v", err)
		return err
	}

	return nil
}

func (l *CR3ImageLoader) tryCR3WithExiftool(path string, tempFilename string) error {
	// Try with alternative exiftool tags that might work for CR3
	cmd := exec.Command("exiftool", "-b", "-ThumbnailImage", path)

	outFile, err := os.Create(tempFilename)
	if err != nil {
		return err
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	err = cmd.Run()

	if err != nil {
		return err
	}

	return nil
}

// Add this to format_specific_loaders.go
