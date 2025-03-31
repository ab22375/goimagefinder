package scanner

import (
	"imagefinder/imageprocessor"
	"strings"
)

// IsImageFile checks if a file extension belongs to an image file
func IsImageFile(path string) bool {
	return imageprocessor.IsImageFile(path)
}

// IsRawFormat checks if a file is in RAW format
func IsRawFormat(path string) bool {
	return imageprocessor.IsRawFormat(path)
}

// IsTiffFormat checks if a file is in TIF format
func IsTiffFormat(path string) bool {
	return imageprocessor.IsTiffFormat(path)
}

// GetFileFormat returns the lowercase file extension without the dot
func GetFileFormat(path string) string {
	format := imageprocessor.GetFileFormat(path)
	return strings.ToLower(string(format))
}

// SupportedRawFormats returns a list of supported RAW formats
func SupportedRawFormats() []string {
	// Get all supported extensions
	allExtensions := imageprocessor.GetSupportedExtensions()
	
	// Filter to include only RAW formats
	var rawFormats []string
	for _, ext := range allExtensions {
		if imageprocessor.IsRawFormat(ext) {
			rawFormats = append(rawFormats, ext)
		}
	}
	
	return rawFormats
}