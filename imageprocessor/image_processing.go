package imageprocessor

import (
	"database/sql"
	"encoding/hex"
	"fmt"
	"image"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"imagefinder/database"
	"imagefinder/logging"

	"gocv.io/x/gocv"
)

// SearchOptions defines the options for searching
type SearchOptions struct {
	QueryPath    string
	Threshold    float64
	SourcePrefix string
	DebugMode    bool
}

// ImageMatch represents a matching image with similarity score
type ImageMatch struct {
	Path         string
	SourcePrefix string
	SSIMScore    float64
}

// LoadImage loads an image using the appropriate loader based on file type
func LoadImage(path string) (gocv.Mat, error) {
	// Get a loader registry
	registry := NewImageLoaderRegistry()

	// Get file extension
	ext := strings.ToLower(filepath.Ext(path))

	// Try to get a specialized loader
	loader := registry.GetLoader(ext)

	// Check if the loader exists and can load this file
	if loader != nil && loader.CanLoad(path) {
		return loader.LoadImage(path)
	}

	// Fallback to standard loading method
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, newImageLoadError("failed to load image", path)
	}

	return img, nil
}

// FindSimilarImages finds similar images in the database based on perceptual and average hash comparisons
// with special handling for different image formats
func FindSimilarImages(db *sql.DB, options SearchOptions) ([]ImageMatch, error) {
	logging.LogInfo("Searching for similar images to %s with threshold %f", options.QueryPath, options.Threshold)

	// Get base filename for potential filename matching
	queryBaseName := filepath.Base(options.QueryPath)
	queryBaseName = strings.TrimSuffix(queryBaseName, filepath.Ext(queryBaseName))

	// Determine if query image is a RAW format
	queryIsRaw := isRawFormat(options.QueryPath)
	queryIsTiff := isTifFormat(options.QueryPath)

	// Load query image with appropriate loader based on format
	var queryImg gocv.Mat
	var err error

	if queryIsRaw {
		// Use RAW-specific loader for RAW files
		logging.LogInfo("Query is a RAW file, using specialized RAW loader")
		rawLoader := NewRawImageLoader()
		queryImg, err = rawLoader.LoadImage(options.QueryPath)
	} else if queryIsTiff {
		// Use TIFF-specific loader for TIFF files
		logging.LogInfo("Query is a TIFF file, using specialized TIFF loader")
		tiffLoader := NewTiffImageLoader()
		queryImg, err = tiffLoader.LoadImage(options.QueryPath)
	} else {
		// Standard loading for other formats
		queryImg, err = LoadImage(options.QueryPath)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to load query image: %v", err)
	}
	defer queryImg.Close()

	// Apply consistent preprocessing for hashing
	processedImg := preprocessImageForHashing(queryImg)
	defer processedImg.Close()

	// Compute hashes for query image
	avgHash, err := ComputeAverageHash(processedImg)
	if err != nil {
		return nil, fmt.Errorf("failed to compute average hash: %v", err)
	}

	pHash, err := ComputePerceptualHash(processedImg)
	if err != nil {
		return nil, fmt.Errorf("failed to compute perceptual hash: %v", err)
	}

	logging.LogInfo("Query image hashes: avgHash=%s, pHash=%s", avgHash, pHash)

	// Query the database for potential matches
	rows, err := database.QueryPotentialMatches(db, options.SourcePrefix)
	if err != nil {
		return nil, fmt.Errorf("database query failed: %v", err)
	}
	defer rows.Close()

	// Process the potential matches
	var matches []ImageMatch
	for rows.Next() {
		var path, sourcePrefix, dbAvgHash, dbPHash string
		if err := rows.Scan(&path, &sourcePrefix, &dbAvgHash, &dbPHash); err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Compute hash similarity scores
		avgHashSimilarity := calculateHashSimilarity(avgHash, dbAvgHash)
		pHashSimilarity := calculateHashSimilarity(pHash, dbPHash)

		// Calculate weighted average of the two similarity scores
		// pHash is generally more reliable, so we weight it higher
		const pHashWeight = 0.7
		const avgHashWeight = 0.3
		similarityScore := (pHashSimilarity * pHashWeight) + (avgHashSimilarity * avgHashWeight)

		// Get base filename from path
		dbBaseName := filepath.Base(path)
		dbBaseName = strings.TrimSuffix(dbBaseName, filepath.Ext(dbBaseName))

		// Check filename similarity to boost score for likely matches
		filenameBoost := calculateFilenameSimiliarity(queryBaseName, dbBaseName)
		similarityScore += filenameBoost

		// If the similarity score is above the threshold, add to matches
		if similarityScore >= options.Threshold {
			match := ImageMatch{
				Path:         path,
				SourcePrefix: sourcePrefix,
				SSIMScore:    similarityScore,
			}
			matches = append(matches, match)

			if options.DebugMode {
				logging.DebugLog("Match found: %s (score: %.4f, avgHash: %.4f, pHash: %.4f, filenameBoost: %.4f)",
					path, similarityScore, avgHashSimilarity, pHashSimilarity, filenameBoost)
			}
		} else if options.DebugMode && (avgHashSimilarity > 0.5 || pHashSimilarity > 0.5) {
			// Log near-misses for debugging
			logging.DebugLog("Near miss: %s (score: %.4f, avgHash: %.4f, pHash: %.4f, filenameBoost: %.4f)",
				path, similarityScore, avgHashSimilarity, pHashSimilarity, filenameBoost)
		}
	}

	// Check for any errors during iteration
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating through rows: %v", err)
	}

	// Sort matches by similarity score (highest first)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].SSIMScore > matches[j].SSIMScore
	})

	// If debug mode is enabled, log the number of matches
	logging.LogInfo("Found %d matches above threshold %.2f", len(matches), options.Threshold)

	return matches, nil
}

// calculateHashSimilarity computes the normalized similarity between two hash strings
// Returns a value between 0.0 (completely different) and 1.0 (identical)
func calculateHashSimilarity(hash1, hash2 string) float64 {
	// Both hashes should already be in hex format, but let's validate
	isValidHex := func(s string) bool {
		for _, c := range s {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return len(s) > 0 && len(s)%2 == 0
	}

	// If either hash is not valid hex, log warning and try binary comparison
	if !isValidHex(hash1) || !isValidHex(hash2) {
		logging.LogWarning("Non-hex hashes being compared: %s vs %s", hash1, hash2)
		// Fall back to binary comparison in this case
		return calculateBinaryHashSimilarity(hash1, hash2)
	}

	// Calculate Hamming distance on hex strings
	distance := 0
	bytes1, err1 := hex.DecodeString(hash1)
	bytes2, err2 := hex.DecodeString(hash2)

	if err1 != nil || err2 != nil {
		logging.LogError("Error decoding hex strings: %v, %v", err1, err2)
		return 0.0
	}

	// Make sure we can compare the byte slices
	if len(bytes1) != len(bytes2) {
		// If lengths are different, pad the shorter one
		if len(bytes1) < len(bytes2) {
			bytes1 = append(bytes1, make([]byte, len(bytes2)-len(bytes1))...)
		} else {
			bytes2 = append(bytes2, make([]byte, len(bytes1)-len(bytes2))...)
		}
	}

	// Compute bit-level Hamming distance
	for i := 0; i < len(bytes1); i++ {
		xor := bytes1[i] ^ bytes2[i]
		// Count set bits in XOR (hamming weight of XOR equals hamming distance)
		for j := 0; j < 8; j++ {
			if (xor & (1 << j)) != 0 {
				distance++
			}
		}
	}

	// Calculate similarity (1.0 = identical, 0.0 = completely different)
	totalBits := len(bytes1) * 8
	return 1.0 - float64(distance)/float64(totalBits)
}

// Fallback for binary hash comparison
func calculateBinaryHashSimilarity(hash1, hash2 string) float64 {
	// Ensure both strings only consist of '0' and '1'
	isBinaryString := func(s string) bool {
		for _, c := range s {
			if c != '0' && c != '1' {
				return false
			}
		}
		return true
	}

	if !isBinaryString(hash1) || !isBinaryString(hash2) {
		logging.LogError("Invalid binary hash formats: %s vs %s", hash1, hash2)
		return 0.0
	}

	// If lengths don't match, we'll extend the shorter one
	if len(hash1) != len(hash2) {
		if len(hash1) < len(hash2) {
			hash1 = hash1 + strings.Repeat("0", len(hash2)-len(hash1))
		} else {
			hash2 = hash2 + strings.Repeat("0", len(hash1)-len(hash2))
		}
	}

	// Calculate hamming distance
	distance := 0
	for i := 0; i < len(hash1); i++ {
		if hash1[i] != hash2[i] {
			distance++
		}
	}

	return 1.0 - float64(distance)/float64(len(hash1))
}

// hexToBinary converts a hexadecimal string to binary string
func hexToBinary(hexStr string) string {
	hexBytes, _ := hex.DecodeString(hexStr)
	var binBuilder strings.Builder

	for _, b := range hexBytes {
		for i := 7; i >= 0; i-- {
			if (b & (1 << i)) != 0 {
				binBuilder.WriteRune('1')
			} else {
				binBuilder.WriteRune('0')
			}
		}
	}

	return binBuilder.String()
}

// calculateFilenameSimiliarity returns a similarity boost based on filename comparison
// Returns a value between 0.0 (no similarity) and 0.15 (highly similar)
func calculateFilenameSimiliarity(filename1, filename2 string) float64 {
	// Convert to lowercase for comparison
	name1 := strings.ToLower(filename1)
	name2 := strings.ToLower(filename2)

	// Check for direct name containment
	if strings.Contains(name1, name2) || strings.Contains(name2, name1) {
		// If one name fully contains the other, high boost
		return 0.15
	}

	// Check for partial match
	// Split names into components
	parts1 := strings.FieldsFunc(name1, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	parts2 := strings.FieldsFunc(name2, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	// Count matching parts
	matches := 0
	for _, p1 := range parts1 {
		if len(p1) < 3 {
			continue // Skip short parts
		}
		for _, p2 := range parts2 {
			if len(p2) < 3 {
				continue // Skip short parts
			}
			if strings.Contains(p1, p2) || strings.Contains(p2, p1) {
				matches++
				break
			}
		}
	}

	// Scale boost based on number of matches
	if matches > 0 {
		return math.Min(0.1, float64(matches)*0.03)
	}

	return 0.0
}

// Helper to check if a file is in RAW format
func isRawFormat(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	rawFormats := []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf"}
	for _, format := range rawFormats {
		if ext == format {
			return true
		}
	}
	return false
}

// Helper to check if a file is in TIF format
func isTifFormat(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".tif" || ext == ".tiff"
}

// preprocessImageForHashing applies consistent preprocessing to ensure hash stability
func preprocessImageForHashing(img gocv.Mat) gocv.Mat {
	// Create a copy of the image to avoid modifying the original
	processed := gocv.NewMat()
	img.CopyTo(&processed)

	// Ensure grayscale
	if img.Channels() != 1 {
		gray := gocv.NewMat()
		gocv.CvtColor(processed, &gray, gocv.ColorBGRToGray)
		processed.Close()
		processed = gray
	}

	// Normalize contrast to improve hash stability
	normalized := gocv.NewMat()
	gocv.Normalize(processed, &normalized, 0, 255, gocv.NormMinMax)
	processed.Close()
	processed = normalized

	// Apply slight Gaussian blur to reduce noise
	blurred := gocv.NewMat()
	gocv.GaussianBlur(processed, &blurred, image.Pt(3, 3), 0, 0, gocv.BorderDefault)
	processed.Close()

	return blurred
}

// End of image processing functions
