package imageprocessor

import (
	"database/sql"
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

// ComputeAverageHash calculates a simple average hash for the image
func ComputeAverageHash(img gocv.Mat) (string, error) {
	if img.Empty() {
		return "", fmt.Errorf("cannot compute hash for empty image")
	}

	// Resize to 8x8
	resized := gocv.NewMat()
	defer resized.Close()

	gocv.Resize(img, &resized, image.Point{X: 8, Y: 8}, 0, 0, gocv.InterpolationLinear)

	// Convert to grayscale if not already
	gray := gocv.NewMat()
	defer gray.Close()

	if img.Channels() != 1 {
		gocv.CvtColor(resized, &gray, gocv.ColorBGRToGray)
	} else {
		resized.CopyTo(&gray)
	}

	// Calculate mean pixel value manually
	var sum uint64
	var count int

	for y := 0; y < gray.Rows(); y++ {
		for x := 0; x < gray.Cols(); x++ {
			pixel := gray.GetUCharAt(y, x)
			sum += uint64(pixel)
			count++
		}
	}

	// Calculate average
	var threshold float64
	if count > 0 {
		threshold = float64(sum) / float64(count)
	}

	// Compute hash
	hash := ""
	for y := 0; y < gray.Rows(); y++ {
		for x := 0; x < gray.Cols(); x++ {
			pixel := gray.GetUCharAt(y, x)
			if float64(pixel) >= threshold {
				hash += "1"
			} else {
				hash += "0"
			}
		}
	}

	return hash, nil
}

// ComputePerceptualHash computes a DCT-based perceptual hash for the image
func ComputePerceptualHash(img gocv.Mat) (string, error) {
	if img.Empty() {
		return "", fmt.Errorf("cannot compute hash for empty image")
	}

	// Resize to 32x32
	resized := gocv.NewMat()
	defer resized.Close()

	gocv.Resize(img, &resized, image.Point{X: 32, Y: 32}, 0, 0, gocv.InterpolationLinear)

	// Convert to grayscale if not already
	gray := gocv.NewMat()
	defer gray.Close()

	if img.Channels() != 1 {
		gocv.CvtColor(resized, &gray, gocv.ColorBGRToGray)
	} else {
		resized.CopyTo(&gray)
	}

	// Convert to float for DCT
	floatImg := gocv.NewMat()
	defer floatImg.Close()
	gray.ConvertTo(&floatImg, gocv.MatTypeCV32F)

	// Apply DCT
	dct := gocv.NewMat()
	defer dct.Close()
	gocv.DCT(floatImg, &dct, 0)

	// Extract 8x8 low frequency components
	lowFreq := dct.Region(image.Rect(0, 0, 8, 8))
	defer lowFreq.Close()

	// Calculate median value
	values := make([]float32, 64)
	idx := 0
	for y := 0; y < lowFreq.Rows(); y++ {
		for x := 0; x < lowFreq.Cols(); x++ {
			values[idx] = lowFreq.GetFloatAt(y, x)
			idx++
		}
	}

	// Simple median calculation
	median := calculateMedian(values)

	// Compute hash
	hash := ""
	for y := 0; y < lowFreq.Rows(); y++ {
		for x := 0; x < lowFreq.Cols(); x++ {
			val := lowFreq.GetFloatAt(y, x)
			if val >= median {
				hash += "1"
			} else {
				hash += "0"
			}
		}
	}

	return hash, nil
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
	// If lengths don't match, we can't compare them properly
	if len(hash1) != len(hash2) {
		return 0.0
	}

	// Calculate Hamming distance (number of differing bits)
	distance := 0
	for i := 0; i < len(hash1); i++ {
		if hash1[i] != hash2[i] {
			distance++
		}
	}

	// Normalize the result to 0.0-1.0 range
	// 0.0 means maximum distance (all bits different)
	// 1.0 means identical hashes (no bits different)
	return 1.0 - float64(distance)/float64(len(hash1))
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

// Utility function to calculate the median of a float array
func calculateMedian(values []float32) float32 {
	// Make a copy to avoid modifying the original slice
	valuesCopy := make([]float32, len(values))
	copy(valuesCopy, values)

	// Sort the values
	for i := 0; i < len(valuesCopy); i++ {
		for j := i + 1; j < len(valuesCopy); j++ {
			if valuesCopy[i] > valuesCopy[j] {
				valuesCopy[i], valuesCopy[j] = valuesCopy[j], valuesCopy[i]
			}
		}
	}

	// Calculate median
	length := len(valuesCopy)
	if length%2 == 0 {
		return (valuesCopy[length/2-1] + valuesCopy[length/2]) / 2
	}

	return valuesCopy[length/2]
}

// Helper function to create standardized image load errors
func newImageLoadError(message, path string) error {
	return fmt.Errorf("%s: %s", message, path)
}
