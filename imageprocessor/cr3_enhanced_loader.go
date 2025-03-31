package imageprocessor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"imagefinder/logging"

	"gocv.io/x/gocv"
)

// EnhancedCR3ImageLoader handles Canon CR3 format with improved methods
type EnhancedCR3ImageLoader struct {
	TempDir string
}

// EnhancedBox is a Box structure for the enhanced CR3 loader
type EnhancedBox struct {
	Size         uint32
	Type         string
	ExtendedSize uint64
}

// readBox reads a box header from an ISOBMFF file
// This is similar to readISOBoxHeader in cr3_parser.go but defined here for independence
func readBox(r io.Reader) (*EnhancedBox, error) {
	var sizeBytes [4]byte
	if _, err := io.ReadFull(r, sizeBytes[:]); err != nil {
		return nil, err
	}

	var typeBytes [4]byte
	if _, err := io.ReadFull(r, typeBytes[:]); err != nil {
		return nil, err
	}

	box := &EnhancedBox{
		Size: binary.BigEndian.Uint32(sizeBytes[:]),
		Type: string(typeBytes[:]),
	}

	// Handle extended size
	if box.Size == 1 {
		var extSizeBytes [8]byte
		if _, err := io.ReadFull(r, extSizeBytes[:]); err != nil {
			return nil, err
		}
		box.ExtendedSize = binary.BigEndian.Uint64(extSizeBytes[:])
	}

	return box, nil
}

// NewEnhancedCR3ImageLoader creates a new enhanced loader for CR3 files
func NewEnhancedCR3ImageLoader() *EnhancedCR3ImageLoader {
	tempDir := os.TempDir()
	return &EnhancedCR3ImageLoader{
		TempDir: tempDir,
	}
}

func (l *EnhancedCR3ImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".cr3" && fileExists(path)
}

// LoadImage loads a CR3 image with enhanced methods
func (l *EnhancedCR3ImageLoader) LoadImage(path string) (gocv.Mat, error) {
	logging.LogInfo("Loading CR3 image with enhanced loader: %s", path)

	// Create a unique temporary filename for the converted image
	tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("cr3_conv_%d.jpg", time.Now().UnixNano()))
	defer os.Remove(tempFilename) // Clean up temp file when done

	// Try using go-exiftool first (if available)
	if success, img := l.tryGoExiftool(path, tempFilename); success {
		logging.LogInfo("Successfully loaded CR3 using go-exiftool")
		return img, nil
	}

	// Try to extract various preview images with exiftool
	previewTags := []string{
		"LargestImagePreview", // This is often the highest quality preview
		"PreviewImage",        // Standard preview
		"OtherImagePreview",   // Additional preview that might be available
		"ThumbnailImage",      // Lower quality thumbnail
		"JpgFromRaw",          // JPEG embedded in RAW file
	}

	for _, tag := range previewTags {
		logging.LogInfo("Trying to extract CR3 %s", tag)
		if err := l.extractWithExiftool(path, tempFilename, tag); err == nil {
			if hasFileContent(tempFilename) {
				img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
				if !img.Empty() {
					logging.LogInfo("Successfully extracted CR3 %s", tag)
					return img, nil
				}
			}
		}
	}

	// Try CR3 native parser (pure Go implementation)
	logging.LogInfo("Trying CR3 native parser")
	if success, img := l.tryCR3NativeParser(path, tempFilename); success {
		logging.LogInfo("Successfully loaded CR3 using native parser")
		return img, nil
	}

	// Try with libheif (CR3 can contain HEIF/HEIC images)
	logging.LogInfo("Trying CR3 with libheif")
	if success, img := l.tryLibheif(path, tempFilename); success {
		logging.LogInfo("Successfully loaded CR3 using libheif")
		return img, nil
	}

	// Try with rawtherapee as fallback
	logging.LogInfo("Trying CR3 with rawtherapee")
	if err := convertWithRawtherapee(path, tempFilename); err == nil {
		if hasFileContent(tempFilename) {
			img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
			if !img.Empty() {
				logging.LogInfo("Successfully loaded CR3 using rawtherapee")
				return img, nil
			}
		}
	}

	// If all else fails, try direct load (unlikely to work)
	logging.LogInfo("All CR3 methods failed, attempting direct load as last resort")
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		// Try with standard Go image packages as last resort
		if goImg, err := tryGoImagePackages(path); err == nil {
			logging.LogInfo("Successfully loaded CR3 using Go image packages")
			return gocvMatFromGoImage(goImg)
		}

		return img, fmt.Errorf("failed to load CR3 image: %s (all methods failed)", path)
	}

	return img, nil
}

// Extract specific preview with exiftool
func (l *EnhancedCR3ImageLoader) extractWithExiftool(path string, outputPath string, tag string) error {
	if !hasExiftool() {
		return fmt.Errorf("exiftool not found")
	}

	cmd := exec.Command("exiftool", "-b", "-"+tag, path)

	outFile, err := os.Create(outputPath)
	if err != nil {
		logging.LogWarning("Failed to create temp file for CR3 %s extraction: %v", tag, err)
		return err
	}
	defer outFile.Close()

	var stderr bytes.Buffer
	cmd.Stdout = outFile
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		logging.LogWarning("Failed to extract CR3 %s: %v, stderr: %s", tag, err, stderr.String())
		return err
	}

	// Verify the output is a valid JPEG
	if !isValidJpeg(outputPath) {
		return fmt.Errorf("extracted content is not a valid JPEG")
	}

	return nil
}

// tryCR3NativeParser implements a basic pure Go CR3 parser to extract embedded JPEG
func (l *EnhancedCR3ImageLoader) tryCR3NativeParser(path string, outputPath string) (bool, gocv.Mat) {
	// Open the CR3 file
	file, err := os.Open(path)
	if err != nil {
		logging.LogWarning("Failed to open CR3 file: %v", err)
		return false, gocv.NewMat()
	}
	defer file.Close()

	// Check file signature to verify it's a CR3 file
	signature := make([]byte, 8)
	if _, err := io.ReadFull(file, signature); err != nil {
		return false, gocv.NewMat()
	}

	// Reset file pointer
	file.Seek(0, io.SeekStart)

	// CR3 files should have "ftyp" box as first box
	box, err := readBox(file)
	if err != nil || box.Type != "ftyp" {
		logging.LogWarning("Not a valid CR3 file (missing ftyp box)")
		return false, gocv.NewMat()
	}

	// Skip to the next box after ftyp
	file.Seek(int64(box.Size), io.SeekStart)

	// Buffer to store the JPEG signature
	jpegSignature := []byte{0xFF, 0xD8, 0xFF}

	// Search for JPEG signature throughout the file
	buf := make([]byte, 4096)
	var jpegStartPos int64 = -1
	var currentPos int64 = int64(box.Size)

	for {
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			logging.LogWarning("Error reading CR3 file: %v", err)
			return false, gocv.NewMat()
		}

		// Search for JPEG signature in the buffer
		for i := 0; i <= n-3; i++ {
			if buf[i] == jpegSignature[0] && buf[i+1] == jpegSignature[1] && buf[i+2] == jpegSignature[2] {
				jpegStartPos = currentPos + int64(i)
				break
			}
		}

		if jpegStartPos != -1 {
			break
		}

		currentPos += int64(n)
	}

	if jpegStartPos == -1 {
		logging.LogWarning("No JPEG preview found in CR3 file")
		return false, gocv.NewMat()
	}

	// Seek to JPEG start position
	file.Seek(jpegStartPos, io.SeekStart)

	// Extract the JPEG data to the output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		logging.LogWarning("Failed to create output file: %v", err)
		return false, gocv.NewMat()
	}
	defer outFile.Close()

	// Write the JPEG signature
	outFile.Write(jpegSignature)

	// Continue copying the rest of the JPEG data
	if _, err := io.Copy(outFile, file); err != nil {
		logging.LogWarning("Failed to copy JPEG data: %v", err)
		return false, gocv.NewMat()
	}

	// Check if we have a valid JPEG now
	if !isValidJpeg(outputPath) {
		// Try to fix the JPEG if needed
		fixJpeg(outputPath)
	}

	// Load the extracted image
	img := gocv.IMRead(outputPath, gocv.IMReadGrayScale)
	if !img.Empty() {
		return true, img
	}

	return false, gocv.NewMat()
}

// tryGoExiftool attempts to use the go-exiftool library if available
func (l *EnhancedCR3ImageLoader) tryGoExiftool(path string, outputPath string) (bool, gocv.Mat) {
	// This is a placeholder for go-exiftool integration
	// You would need to import "github.com/barasher/go-exiftool" and implement the actual calls

	// Example implementation would use the imported library
	// For now just return false to skip this method
	return false, gocv.NewMat()
}

// tryLibheif attempts to use libheif to extract HEIF/HEIC images from CR3
func (l *EnhancedCR3ImageLoader) tryLibheif(path string, outputPath string) (bool, gocv.Mat) {
	// Check if heif-convert tool is available
	_, err := exec.LookPath("heif-convert")
	if err != nil {
		return false, gocv.NewMat()
	}

	// Try to convert using heif-convert
	cmd := exec.Command("heif-convert", path, outputPath)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err = cmd.Run()
	if err != nil {
		logging.LogWarning("heif-convert failed: %v, stderr: %s", err, stderr.String())
		return false, gocv.NewMat()
	}

	// Check if we got a valid image
	img := gocv.IMRead(outputPath, gocv.IMReadGrayScale)
	if !img.Empty() {
		return true, img
	}

	return false, gocv.NewMat()
}

// isValidJpeg checks if a file contains a valid JPEG image
func isValidJpeg(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	// Check for JPEG signature
	signature := make([]byte, 3)
	if _, err := io.ReadFull(file, signature); err != nil {
		return false
	}

	return signature[0] == 0xFF && signature[1] == 0xD8 && signature[2] == 0xFF
}

// fixJpeg attempts to repair a corrupted JPEG
func fixJpeg(path string) error {
	// Check if jpegtran is available
	_, err := exec.LookPath("jpegtran")
	if err != nil {
		return err
	}

	tempFile := path + ".fixed"

	cmd := exec.Command("jpegtran", "-copy", "none", "-outfile", tempFile, path)
	err = cmd.Run()

	if err != nil {
		return err
	}

	// Replace original with fixed version
	return os.Rename(tempFile, path)
}
