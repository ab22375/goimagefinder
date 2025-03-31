package imageprocessor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"imagefinder/logging"

	"gocv.io/x/gocv"
)

// CR3Parser implements a lightweight parser for CR3 files in pure Go
type CR3Parser struct {
	TempDir string
}

// NewCR3Parser creates a new CR3 parser
func NewCR3Parser() *CR3Parser {
	tempDir := os.TempDir()
	return &CR3Parser{
		TempDir: tempDir,
	}
}

// CanLoad checks if this parser can handle the file
func (p *CR3Parser) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".cr3" && fileExists(path)
}

// LoadImage extracts and loads the preview image from a CR3 file
func (p *CR3Parser) LoadImage(path string) (gocv.Mat, error) {
	logging.LogInfo("Loading CR3 image with native parser: %s", path)

	// Create a unique temporary filename for the extracted preview
	tempFilename := filepath.Join(p.TempDir, fmt.Sprintf("cr3_native_%d.jpg",
		binary.BigEndian.Uint64([]byte(fmt.Sprintf("%d", os.Getpid())))))
	defer os.Remove(tempFilename)

	// Extract preview JPEG
	if err := p.extractPreviewJPEG(path, tempFilename); err != nil {
		return gocv.NewMat(), fmt.Errorf("failed to extract preview: %v", err)
	}

	// Load the extracted preview
	img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, fmt.Errorf("extracted preview could not be loaded")
	}

	return img, nil
}

// extractPreviewJPEG extracts the embedded JPEG preview from a CR3 file
func (p *CR3Parser) extractPreviewJPEG(path, outputPath string) error {
	// Open the CR3 file
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Check if it's a valid CR3 file by reading the FTYP box
	box, err := readISOBoxHeader(file)
	if err != nil {
		return fmt.Errorf("failed to read box header: %v", err)
	}

	if box.Type != "ftyp" {
		return fmt.Errorf("not a valid CR3 file (first box is not ftyp)")
	}

	// Skip to the end of FTYP box
	file.Seek(int64(box.Size-8), io.SeekCurrent)

	// Parse the file structure to find the preview image
	preview, err := p.findPreviewInCR3(file)
	if err != nil {
		return err
	}

	// Write the preview to the output file
	return os.WriteFile(outputPath, preview, 0644)
}

// findPreviewInCR3 parses the CR3 file structure to find the preview JPEG
func (p *CR3Parser) findPreviewInCR3(file io.ReadSeeker) ([]byte, error) {
	// CR3 files typically have a moov box that contains metadata
	// The preview image is often found in a CRAW box or in a uuid box
	// This is a simplified implementation that searches for JPEG signatures

	// Reset to beginning of the file
	file.Seek(0, io.SeekStart)

	// First, try to find JPEG by uuid box parsing
	preview, err := p.findPreviewByBoxParsing(file)
	if err == nil && len(preview) > 0 {
		return preview, nil
	}

	// If box parsing failed, try signature search approach
	logging.LogInfo("Box parsing failed, trying signature search")
	return p.findPreviewBySignature(file)
}

// findPreviewByBoxParsing tries to find preview by properly parsing ISOBMFF boxes
func (p *CR3Parser) findPreviewByBoxParsing(file io.ReadSeeker) ([]byte, error) {
	// Reset to beginning of the file
	file.Seek(0, io.SeekStart)

	// Skip the ftyp box we already checked
	box, _ := readISOBoxHeader(file)
	file.Seek(int64(box.Size-8), io.SeekCurrent)

	// Keep track of our position in the file
	var currentPos int64 = int64(box.Size)

	// Now look through the remaining boxes
	for {
		box, err := readISOBoxHeader(file)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Record the start position of this box
		boxStartPos := currentPos
		currentPos += 8 // Add the size of the box header (8 bytes)

		// Check for uuid box which might contain previews
		if box.Type == "uuid" {
			// Read the extended type (16 bytes UUID)
			uuid := make([]byte, 16)
			if _, err := io.ReadFull(file, uuid); err != nil {
				return nil, err
			}

			currentPos += 16 // Add the UUID size

			// Canon CR3 preview is often in a specific UUID box
			// This is a simplified check - in reality, you'd want to check specific UUIDs
			// Look for JPEG signature inside this box
			boxDataSize := box.Size - 8 - 16 // Total size minus header and UUID

			// Buffer for reading box data in chunks
			bufSize := 8192
			if int(boxDataSize) < bufSize {
				bufSize = int(boxDataSize)
			}

			buf := make([]byte, bufSize)
			var remainingSize uint64 = uint64(boxDataSize)

			for remainingSize > 0 {
				readSize := uint64(bufSize)
				if remainingSize < uint64(bufSize) {
					readSize = remainingSize
				}

				n, err := file.Read(buf[:readSize])
				if err != nil {
					break
				}

				// Look for JPEG signature
				if jpegPos := bytes.Index(buf[:n], []byte{0xFF, 0xD8, 0xFF}); jpegPos >= 0 {
					// Found JPEG signature, now extract the whole JPEG
					jpegStartPos := boxStartPos + int64(8) + int64(16) + int64(jpegPos)

					// Seek to the start of the JPEG
					file.Seek(jpegStartPos, io.SeekStart)

					return extractJPEG(file)
				}

				remainingSize -= uint64(n)
			}

			// Move to the end of this box if we didn't find anything
			file.Seek(boxStartPos+int64(box.Size), io.SeekStart)
			currentPos = boxStartPos + int64(box.Size)

		} else if box.Type == "CRAW" || box.Type == "JPEG" {
			// These box types might directly contain image data
			// Skip the box header (already read)

			// Look for JPEG signature at the start of this box
			jpegSig := make([]byte, 3)
			if _, err := io.ReadFull(file, jpegSig); err == nil {
				if jpegSig[0] == 0xFF && jpegSig[1] == 0xD8 && jpegSig[2] == 0xFF {
					// Found JPEG header, seek back to start of the header
					file.Seek(-3, io.SeekCurrent)
					return extractJPEG(file)
				}
			}

			// If we didn't find a JPEG header at the start, move to the end of this box
			file.Seek(boxStartPos+int64(box.Size), io.SeekStart)
			currentPos = boxStartPos + int64(box.Size)
		} else {
			// For other box types, just skip to the end
			file.Seek(boxStartPos+int64(box.Size), io.SeekStart)
			currentPos = boxStartPos + int64(box.Size)
		}
	}

	return nil, fmt.Errorf("no preview found in CR3 box structure")
}

// findPreviewBySignature uses a simple signature-based approach to find embedded JPEGs
func (p *CR3Parser) findPreviewBySignature(file io.ReadSeeker) ([]byte, error) {
	// Reset to beginning of the file
	file.Seek(0, io.SeekStart)

	// Buffer for reading file in chunks
	bufSize := 8192
	buf := make([]byte, bufSize)

	// Keep track of position in file
	var pos int64 = 0
	var largestJPEG []byte
	var largestJPEGSize int

	for {
		n, err := file.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		// Look for JPEG signature
		jpegPos := bytes.Index(buf[:n], []byte{0xFF, 0xD8, 0xFF})
		if jpegPos >= 0 {
			// Found a JPEG signature
			jpegStartPos := pos + int64(jpegPos)

			// Seek to start of the JPEG
			file.Seek(jpegStartPos, io.SeekStart)

			// Extract the JPEG
			jpegData, err := extractJPEG(file)
			if err == nil {
				// Keep track of the largest JPEG we find
				if len(jpegData) > largestJPEGSize {
					largestJPEG = jpegData
					largestJPEGSize = len(jpegData)
				}

				// Continue searching from the end of this JPEG
				file.Seek(jpegStartPos+int64(len(jpegData)), io.SeekStart)
				pos = jpegStartPos + int64(len(jpegData))
			} else {
				// If extraction failed, move past this signature
				file.Seek(jpegStartPos+3, io.SeekStart)
				pos = jpegStartPos + 3
			}
		} else {
			// No JPEG found in this chunk, move to next chunk
			// Backtrack slightly to catch signatures that might span chunk boundaries
			backtrack := 2
			if n <= backtrack {
				backtrack = 0
			}

			pos += int64(n - backtrack)
			file.Seek(pos, io.SeekStart)
		}
	}

	if largestJPEG != nil {
		return largestJPEG, nil
	}

	return nil, fmt.Errorf("no JPEG preview found in CR3 file")
}

// extractJPEG extracts a complete JPEG from the current position in the stream
func extractJPEG(r io.Reader) ([]byte, error) {
	// JPEG starts with FF D8 and ends with FF D9
	var jpeg bytes.Buffer

	// Write the JPEG header
	jpeg.Write([]byte{0xFF, 0xD8})

	// Read and process the JPEG data
	buf := make([]byte, 4096)
	var lastByte byte

	for {
		n, err := r.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		if n == 0 {
			break
		}

		// Look for JPEG end marker (FF D9)
		for i := 0; i < n; i++ {
			if lastByte == 0xFF && buf[i] == 0xD9 {
				// Found end of JPEG
				jpeg.WriteByte(buf[i])
				return jpeg.Bytes(), nil
			}

			jpeg.WriteByte(buf[i])
			lastByte = buf[i]
		}

		// If JPEG is getting too large, it might not be a valid JPEG
		// Set a reasonable limit
		if jpeg.Len() > 20*1024*1024 { // 20MB limit
			return nil, fmt.Errorf("JPEG extraction exceeded size limit")
		}
	}

	// If we get here, we reached EOF without finding JPEG end marker
	// Check if the data we have looks reasonably like a JPEG
	if jpeg.Len() > 1000 {
		// Append an end marker to make it a valid JPEG
		jpeg.Write([]byte{0xFF, 0xD9})
		return jpeg.Bytes(), nil
	}

	return nil, fmt.Errorf("incomplete JPEG data")
}

// readISOBoxHeader reads an ISO base media file format box header
func readISOBoxHeader(r io.Reader) (*ISOBox, error) {
	var size uint32
	var boxType [4]byte

	if err := binary.Read(r, binary.BigEndian, &size); err != nil {
		return nil, err
	}

	if _, err := io.ReadFull(r, boxType[:]); err != nil {
		return nil, err
	}

	box := &ISOBox{
		Size: size,
		Type: string(boxType[:]),
	}

	// Handle special size cases
	if box.Size == 1 {
		// Extended size (64-bit)
		var extendedSize uint64
		if err := binary.Read(r, binary.BigEndian, &extendedSize); err != nil {
			return nil, err
		}
		box.ExtendedSize = extendedSize
	}

	return box, nil
}

// ISOBox represents an ISO base media file format box
type ISOBox struct {
	Size         uint32
	Type         string
	ExtendedSize uint64
}
