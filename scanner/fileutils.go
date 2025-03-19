package scanner

import (
	"path/filepath"
	"strings"
)

// IsImageFile checks if a file extension belongs to an image file
func IsImageFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg", ".png", ".gif", ".bmp", ".webp":
		return true
	case ".cr2", ".nef", ".arw", ".orf", ".rw2", ".pef", ".dng", ".raw", ".raf", ".cr3", ".nrw", ".srf":
		return true
	case ".tif", ".tiff":
		return true
	default:
		return false
	}
}

// IsRawFormat checks if a file is in RAW format
func IsRawFormat(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	rawFormats := []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf", ".orf", ".rw2", ".pef", ".raw"}
	for _, format := range rawFormats {
		if ext == format {
			return true
		}
	}
	return false
}

// IsTiffFormat checks if a file is in TIF format
func IsTiffFormat(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".tif" || ext == ".tiff"
}

// GetFileFormat returns the lowercase file extension without the dot
func GetFileFormat(path string) string {
	return strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
}

// SupportedRawFormats returns a list of supported RAW formats
func SupportedRawFormats() []string {
	return []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf", ".orf", ".rw2", ".pef", ".raw"}
}
