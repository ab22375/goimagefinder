package imageprocessor

import (
	"database/sql"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"imagefinder/logging"
	"imagefinder/types"

	"gocv.io/x/gocv"
)

// SearchOptions defines the options for image searching
type SearchOptions struct {
	QueryPath    string
	Threshold    float64
	SourcePrefix string
	DebugMode    bool
}

// ImageLoader interface for loading different image formats
type ImageLoader interface {
	CanLoad(path string) bool
	LoadImage(path string) (gocv.Mat, error)
}

// DefaultImageLoader handles common formats supported by gocv directly
type DefaultImageLoader struct{}

func (l *DefaultImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	// Check extension and make sure file exists and is readable
	if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".tiff" || ext == ".tif" {
		_, err := os.Stat(path)
		return err == nil
	}
	return false
}

func (l *DefaultImageLoader) LoadImage(path string) (gocv.Mat, error) {
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, fmt.Errorf("failed to load image with default loader: %s", path)
	}
	return img, nil
}

// RawImageLoader handles RAW camera formats
type RawImageLoader struct{}

func (l *RawImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	rawFormats := []string{".nef", ".nrw", ".arw", ".raf", ".srf", ".cr2", ".cr3", ".dng"}
	for _, format := range rawFormats {
		if ext == format {
			// Check if file exists and is readable
			_, err := os.Stat(path)
			return err == nil
		}
	}
	return false
}

func (l *RawImageLoader) LoadImage(path string) (gocv.Mat, error) {
	// This is a simplified implementation
	// In a real application, you would use libraries like libraw or dcraw
	// to decode raw files to a temporary TIFF/JPEG, then load that with gocv

	// Example pseudocode:
	// 1. Use external tool to convert RAW to temporary TIFF
	// 2. Load the TIFF with gocv
	// 3. Delete the temporary file

	// For demonstration, we'll attempt to load directly
	// This will likely fail for most RAW formats without proper implementation
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, fmt.Errorf("failed to load RAW image: %s (proper RAW implementation needed)", path)
	}
	return img, nil
}

// HeicImageLoader handles HEIC/HEIF formats
type HeicImageLoader struct{}

func (l *HeicImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".heic" || ext == ".heif" {
		// Check if file exists and is readable
		_, err := os.Stat(path)
		return err == nil
	}
	return false
}

func (l *HeicImageLoader) LoadImage(path string) (gocv.Mat, error) {
	// Similar to RAW files, HEIC typically needs conversion
	// You would use tools like libheif or external converters

	// For demonstration purposes
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, fmt.Errorf("failed to load HEIC image: %s (proper HEIC implementation needed)", path)
	}
	return img, nil
}

// ImageLoaderRegistry manages available image loaders
type ImageLoaderRegistry struct {
	loaders []ImageLoader
}

// NewImageLoaderRegistry creates a registry with default loaders
func NewImageLoaderRegistry() *ImageLoaderRegistry {
	return &ImageLoaderRegistry{
		loaders: []ImageLoader{
			&DefaultImageLoader{},
			&RawImageLoader{},
			&HeicImageLoader{},
		},
	}
}

// RegisterLoader adds a custom loader to the registry
func (r *ImageLoaderRegistry) RegisterLoader(loader ImageLoader) {
	r.loaders = append(r.loaders, loader)
}

// GetLoaders returns the slice of registered loaders
func (r *ImageLoaderRegistry) GetLoaders() []ImageLoader {
	return r.loaders
}

// CanLoadFile checks if any registered loader can handle the given file
func (r *ImageLoaderRegistry) CanLoadFile(path string) bool {
	for _, loader := range r.loaders {
		if loader.CanLoad(path) {
			return true
		}
	}
	return false
}

// LoadImage tries to load an image using the appropriate loader
func (r *ImageLoaderRegistry) LoadImage(path string) (gocv.Mat, error) {
	for _, loader := range r.loaders {
		if loader.CanLoad(path) {
			return loader.LoadImage(path)
		}
	}
	return gocv.NewMat(), fmt.Errorf("no suitable loader found for image: %s", path)
}

// LoadImage loads an image in grayscale with error handling
func LoadImage(path string) (gocv.Mat, error) {
	registry := NewImageLoaderRegistry()
	return registry.LoadImage(path)
}

// ComputeAverageHash computes average hash for image indexing
func ComputeAverageHash(img gocv.Mat) (string, error) {
	// Resize to 8x8
	resized := gocv.NewMat()
	defer resized.Close()

	gocv.Resize(img, &resized, image.Point{X: 8, Y: 8}, 0, 0, gocv.InterpolationArea)

	// Convert to grayscale if not already
	gray := gocv.NewMat()
	defer gray.Close()

	if img.Channels() > 1 {
		gocv.CvtColor(resized, &gray, gocv.ColorBGRToGray)
	} else {
		resized.CopyTo(&gray)
	}

	// Calculate average pixel value manually
	var sum float64
	totalPixels := gray.Rows() * gray.Cols()

	for i := 0; i < gray.Rows(); i++ {
		for j := 0; j < gray.Cols(); j++ {
			sum += float64(gray.GetUCharAt(i, j))
		}
	}

	threshold := sum / float64(totalPixels)

	// Compute the hash
	var hash strings.Builder
	for i := 0; i < 8; i++ {
		for j := 0; j < 8; j++ {
			pixel := gray.GetUCharAt(i, j)
			if float64(pixel) >= threshold {
				hash.WriteString("1")
			} else {
				hash.WriteString("0")
			}
		}
	}

	return hash.String(), nil
}

// ComputePerceptualHash computes perceptual hash (pHash) for better matching
func ComputePerceptualHash(img gocv.Mat) (string, error) {
	// Resize to 32x32
	resized := gocv.NewMat()
	defer resized.Close()

	gocv.Resize(img, &resized, image.Point{X: 32, Y: 32}, 0, 0, gocv.InterpolationArea)

	// Convert to grayscale if not already
	gray := gocv.NewMat()
	defer gray.Close()

	if img.Channels() > 1 {
		gocv.CvtColor(resized, &gray, gocv.ColorBGRToGray)
	} else {
		resized.CopyTo(&gray)
	}

	// Since DCT isn't available, let's implement a simplified alternative hash
	// We'll use a variation of the average hash with more regions

	// Create a simplified hash based on brightness patterns
	regions := 8 // 8x8 regions
	var regionValues []float32

	// Calculate average brightness in each region
	regionHeight := gray.Rows() / regions
	regionWidth := gray.Cols() / regions

	for i := 0; i < regions; i++ {
		for j := 0; j < regions; j++ {
			// Calculate region boundaries
			startY := i * regionHeight
			endY := (i + 1) * regionHeight
			startX := j * regionWidth
			endX := (j + 1) * regionWidth

			// Calculate average for region
			var sum float32
			var count int
			for y := startY; y < endY; y++ {
				for x := startX; x < endX; x++ {
					sum += float32(gray.GetUCharAt(y, x))
					count++
				}
			}

			avg := sum / float32(count)
			regionValues = append(regionValues, avg)
		}
	}

	// Calculate median
	sort.Slice(regionValues, func(i, j int) bool {
		return regionValues[i] < regionValues[j]
	})
	median := regionValues[len(regionValues)/2]

	// Create hash based on whether each value is above median
	var hash strings.Builder
	for _, val := range regionValues {
		if val > median {
			hash.WriteString("1")
		} else {
			hash.WriteString("0")
		}
	}

	return hash.String(), nil
}

// ComputeSSIM computes a simplified and more robust SSIM implementation
func ComputeSSIM(img1, img2 gocv.Mat) float64 {
	// Check for valid matrices
	if img1.Empty() || img2.Empty() || img1.Rows() == 0 || img1.Cols() == 0 ||
		img2.Rows() == 0 || img2.Cols() == 0 {
		return 0.0
	}

	// Convert to 8-bit grayscale if needed
	img1Gray := gocv.NewMat()
	img2Gray := gocv.NewMat()
	defer img1Gray.Close()
	defer img2Gray.Close()

	if img1.Type() != gocv.MatTypeCV8U {
		img1.ConvertTo(&img1Gray, gocv.MatTypeCV8U)
	} else {
		img1.CopyTo(&img1Gray)
	}

	if img2.Type() != gocv.MatTypeCV8U {
		img2.ConvertTo(&img2Gray, gocv.MatTypeCV8U)
	} else {
		img2.CopyTo(&img2Gray)
	}

	// Ensure images are same size
	resized := gocv.NewMat()
	defer resized.Close()
	gocv.Resize(img2Gray, &resized, image.Point{X: img1Gray.Cols(), Y: img1Gray.Rows()}, 0, 0, gocv.InterpolationLinear)

	// Calculate simple mean difference
	diff := gocv.NewMat()
	defer diff.Close()

	gocv.AbsDiff(img1Gray, resized, &diff)

	mean := gocv.NewMat()
	stdDev := gocv.NewMat()
	defer mean.Close()
	defer stdDev.Close()

	if diff.Empty() || diff.Rows() == 0 || diff.Cols() == 0 {
		return 0.0
	}

	gocv.MeanStdDev(diff, &mean, &stdDev)

	if mean.Empty() || mean.Rows() == 0 || mean.Cols() == 0 {
		return 0.0
	}

	// Calculate similarity score (1 - normalized difference)
	meanDiff := mean.GetDoubleAt(0, 0)
	if meanDiff > 255.0 {
		return 0.0
	}

	// Return similarity score (1 = identical, 0 = completely different)
	return 1.0 - (meanDiff / 255.0)
}

// CalculateHammingDistance calculates the number of differing bits between two hash strings
func CalculateHammingDistance(hash1, hash2 string) int {
	var distance int
	minLen := len(hash1)
	if len(hash2) < minLen {
		minLen = len(hash2)
	}

	for i := 0; i < minLen; i++ {
		if hash1[i] != hash2[i] {
			distance++
		}
	}

	return distance
}

// FindSimilarImages finds similar images to the query image
func FindSimilarImages(db *sql.DB, options SearchOptions) ([]types.ImageMatch, error) {
	if options.DebugMode {
		logging.DebugLog("Starting image search for: %s", options.QueryPath)
		logging.DebugLog("Threshold: %.2f, Source Prefix: %s", options.Threshold, options.SourcePrefix)
	}

	queryImg, err := LoadImage(options.QueryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load query image: %v", err)
	}
	defer queryImg.Close()

	// Compute hashes for query image
	avgHash, err := ComputeAverageHash(queryImg)
	if err != nil {
		return nil, fmt.Errorf("cannot compute average hash: %v", err)
	}

	pHash, err := ComputePerceptualHash(queryImg)
	if err != nil {
		return nil, fmt.Errorf("cannot compute perceptual hash: %v", err)
	}

	if options.DebugMode {
		logging.DebugLog("Query image hashes - avgHash: %s, pHash: %s", avgHash, pHash)
	}

	// Query database for potential matches
	rows, err := db.Query(`SELECT path, source_prefix, average_hash, perceptual_hash FROM images 
		WHERE source_prefix = ? OR ? = ''`, options.SourcePrefix, options.SourcePrefix)
	if err != nil {
		return nil, fmt.Errorf("database query error: %v", err)
	}
	defer rows.Close()

	var matches []types.ImageMatch
	var wg sync.WaitGroup
	var mutex sync.Mutex
	semaphore := make(chan struct{}, 8)

	// Process count for logging
	var processed int
	startTime := time.Now()

	for rows.Next() {
		var path, sourcePrefix, dbAvgHash, dbPHash string
		err := rows.Scan(&path, &sourcePrefix, &dbAvgHash, &dbPHash)
		if err != nil {
			if options.DebugMode {
				logging.LogError("Error scanning row: %v", err)
			}
			continue
		}

		processed++

		// Calculate hamming distance (number of different bits)
		avgHashDistance := CalculateHammingDistance(avgHash, dbAvgHash)
		pHashDistance := CalculateHammingDistance(pHash, dbPHash)

		// If hash distance is within threshold, compute SSIM for more accurate comparison
		if avgHashDistance <= 10 || pHashDistance <= 12 { // Adjustable thresholds
			if options.DebugMode {
				logging.DebugLog("Potential match found: %s (avgHashDist: %d, pHashDist: %d)",
					path, avgHashDistance, pHashDistance)
			}

			wg.Add(1)
			semaphore <- struct{}{}

			go func(p, prefix string) {
				defer wg.Done()
				defer func() { <-semaphore }()

				// Check if file still exists
				_, err := os.Stat(p)
				if err != nil {
					if options.DebugMode && !os.IsNotExist(err) {
						logging.LogWarning("Error checking file %s: %v", p, err)
					}
					return
				}

				// Load candidate image and compute SSIM
				candidateImg, err := LoadImage(p)
				if err != nil {
					if options.DebugMode {
						logging.LogWarning("Failed to load candidate image %s: %v", p, err)
					}
					return
				}
				defer candidateImg.Close()

				ssimScore := ComputeSSIM(queryImg, candidateImg)

				// If SSIM score is above threshold, add to matches
				if ssimScore >= options.Threshold {
					if options.DebugMode {
						logging.DebugLog("Match confirmed: %s (SSIM: %.4f)", p, ssimScore)
					}

					match := types.ImageMatch{
						Path:         p,
						SourcePrefix: prefix,
						SSIMScore:    ssimScore,
					}

					mutex.Lock()
					matches = append(matches, match)
					mutex.Unlock()
				}
			}(path, sourcePrefix)
		}

		// Log progress every 100 images in debug mode
		if options.DebugMode && processed%100 == 0 {
			elapsed := time.Since(startTime)
			logging.DebugLog("Search progress: %d images processed in %v", processed, elapsed)
		}
	}

	wg.Wait()

	if options.DebugMode {
		logging.DebugLog("Search completed. Total images processed: %d, Matches found: %d",
			processed, len(matches))
	}

	// Sort matches by SSIM score (higher is better)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].SSIMScore > matches[j].SSIMScore
	})

	return matches, nil
}
