package imageprocessor

import (
	"path/filepath"
	"strings"
)

// FormatType represents a known image format type
type FormatType string

// Known image format constants
const (
	FormatUnknown   FormatType = "unknown"
	FormatJPEG      FormatType = "jpeg"
	FormatPNG       FormatType = "png"
	FormatGIF       FormatType = "gif"
	FormatTIFF      FormatType = "tiff"
	FormatRAW       FormatType = "raw"
	FormatBMP       FormatType = "bmp"
	FormatWEBP      FormatType = "webp"
	FormatHEIC      FormatType = "heic"
	FormatCR2       FormatType = "cr2"
	FormatCR3       FormatType = "cr3"
	FormatNEF       FormatType = "nef"
	FormatARW       FormatType = "arw"
	FormatDNG       FormatType = "dng"
	FormatPSD       FormatType = "psd"
)

// Map of extensions to format types
var formatExtensions = map[string]FormatType{
	".jpg":  FormatJPEG,
	".jpeg": FormatJPEG,
	".png":  FormatPNG,
	".gif":  FormatGIF,
	".tif":  FormatTIFF,
	".tiff": FormatTIFF,
	".bmp":  FormatBMP,
	".webp": FormatWEBP,
	".heic": FormatHEIC,
	".psd":  FormatPSD,
	
	// RAW formats
	".raw":  FormatRAW,
	".cr2":  FormatCR2,
	".cr3":  FormatCR3,
	".nef":  FormatNEF,
	".arw":  FormatARW,
	".dng":  FormatDNG,
	".raf":  FormatRAW,
	".nrw":  FormatRAW,
	".srf":  FormatRAW,
}

// IsImageFile checks if a file is a supported image based on extension
func IsImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, supported := formatExtensions[ext]
	return supported
}

// GetFileFormat returns the format type based on file extension
func GetFileFormat(path string) FormatType {
	ext := strings.ToLower(filepath.Ext(path))
	format, exists := formatExtensions[ext]
	if !exists {
		return FormatUnknown
	}
	return format
}

// IsRawFormat checks if a file is in RAW format
func IsRawFormat(path string) bool {
	format := GetFileFormat(path)
	return format == FormatRAW || 
	       format == FormatCR2 || 
	       format == FormatCR3 || 
	       format == FormatNEF || 
	       format == FormatARW || 
	       format == FormatDNG
}

// IsTiffFormat checks if a file is in TIFF format
func IsTiffFormat(path string) bool {
	format := GetFileFormat(path)
	return format == FormatTIFF
}

// GetSupportedExtensions returns all supported image file extensions
func GetSupportedExtensions() []string {
	extensions := make([]string, 0, len(formatExtensions))
	for ext := range formatExtensions {
		extensions = append(extensions, ext)
	}
	return extensions
}

// FormatToExtension returns a canonical file extension for a format
func FormatToExtension(format FormatType) string {
	switch format {
	case FormatJPEG:
		return ".jpg"
	case FormatPNG:
		return ".png"
	case FormatGIF:
		return ".gif"
	case FormatTIFF:
		return ".tiff"
	case FormatBMP:
		return ".bmp"
	case FormatWEBP:
		return ".webp"
	case FormatHEIC:
		return ".heic"
	case FormatPSD:
		return ".psd"
	case FormatCR2:
		return ".cr2"
	case FormatCR3:
		return ".cr3"
	case FormatNEF:
		return ".nef"
	case FormatARW:
		return ".arw"
	case FormatDNG:
		return ".dng"
	default:
		return ""
	}
}