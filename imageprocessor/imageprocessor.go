package imageprocessor

import (
	"bytes"
	"database/sql"
	"fmt"
	"image"
	"os"
	"os/exec"
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
type RawImageLoader struct {
	TempDir string
}

// NewRawImageLoader creates a new RawImageLoader with a temp directory
func NewRawImageLoader() *RawImageLoader {
	// Create a temp directory for raw image processing if needed
	tempDir := os.TempDir()
	return &RawImageLoader{
		TempDir: tempDir,
	}
}

func (l *RawImageLoader) CanLoad(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	// Explicitly include all requested formats: DNG, RAF, ARW, NEF, CR2, CR3
	rawFormats := []string{".dng", ".raf", ".arw", ".nef", ".cr2", ".cr3", ".nrw", ".srf"}
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
	// Create a unique temporary filename for the converted image
	tempFilename := filepath.Join(l.TempDir, fmt.Sprintf("raw_conv_%d.tiff", time.Now().UnixNano()))
	defer os.Remove(tempFilename) // Clean up temp file when done

	// Check if it's a CR3 file specifically
	if strings.ToLower(filepath.Ext(path)) == ".cr3" {
		if success, img := l.tryCR3(path, tempFilename); success {
			return img, nil
		}
	}

	// First try with dcraw
	if success, img := l.tryDcraw(path, tempFilename); success {
		return img, nil
	}

	// If dcraw fails, try libraw fallback
	if success, img := l.tryLibRaw(path, tempFilename); success {
		return img, nil
	}

	// If all else fails, attempt direct load (unlikely to work for most RAW formats)
	img := gocv.IMRead(path, gocv.IMReadGrayScale)
	if img.Empty() {
		return img, fmt.Errorf("failed to load RAW image: %s (all conversion methods failed)", path)
	}

	return img, nil
}

func (l *RawImageLoader) tryDcraw(path string, tempFilename string) (bool, gocv.Mat) {
	// Convert RAW to TIFF using dcraw
	// -T = output TIFF
	// -c = output to stdout (we redirect to file)
	// -w = use camera white balance
	// -q 3 = use high-quality interpolation
	cmd := exec.Command("dcraw", "-T", "-c", "-w", "-q", "3", path)

	// Create the output file
	outFile, err := os.Create(tempFilename)
	if err != nil {
		logging.LogWarning("Failed to create temp file for dcraw conversion: %v", err)
		return false, gocv.NewMat()
	}
	defer outFile.Close()

	// Set stdout to our file
	cmd.Stdout = outFile

	// Capture stderr for error reporting
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	// Run the command
	err = cmd.Run()
	if err != nil {
		logging.LogWarning("dcraw conversion failed: %v, stderr: %s", err, stderr.String())
		return false, gocv.NewMat()
	}

	// Load the converted TIFF
	img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
	if img.Empty() {
		return false, gocv.NewMat()
	}

	return true, img
}

func (l *RawImageLoader) tryLibRaw(path string, tempFilename string) (bool, gocv.Mat) {
	// Try with rawtherapee-cli as an alternative for RAW conversion
	// Example: rawtherapee-cli -o /tmp/output.jpg -c /path/to/raw/file.CR2
	cmd := exec.Command("rawtherapee-cli", "-o", tempFilename, "-c", path)

	// Capture stderr for error reporting
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		logging.LogWarning("rawtherapee conversion failed: %v, stderr: %s", err, stderr.String())
		return false, gocv.NewMat()
	}

	img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
	if img.Empty() {
		return false, gocv.NewMat()
	}

	return true, img
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
			NewRawImageLoader(), // Use constructor for the enhanced version
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

// FindSimilarImages finds similar images to the query image with enhanced RAW/JPG matching
func FindSimilarImages(db *sql.DB, options SearchOptions) ([]types.ImageMatch, error) {
	if options.DebugMode {
		logging.DebugLog("Starting image search for: %s", options.QueryPath)
		logging.DebugLog("Threshold: %.2f, Source Prefix: %s", options.Threshold, options.SourcePrefix)
	}

	// Check if query is a RAW file
	isRawQuery := isRawFormat(options.QueryPath)
	if isRawQuery && options.DebugMode {
		logging.DebugLog("Query image is a RAW format file, using special processing")
	}

	// If the query is a JPG, check if there might be RAW versions to match against
	isJpgQuery := isJpgFormat(options.QueryPath)

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

	// Create a more complex query based on the query type
	var rows *sql.Rows
	if isJpgQuery {
		// For JPG queries, boost the chance of finding related RAW files by using a more aggressive query
		// This query includes RAW files with similar filenames
		baseFilename := getBaseFilename(options.QueryPath)

		query := `SELECT path, source_prefix, average_hash, perceptual_hash, format FROM images 
			WHERE (source_prefix = ? OR ? = '') AND 
			      (path LIKE ? OR 1=1)`

		searchPattern := "%" + baseFilename + "%"

		if options.DebugMode {
			logging.DebugLog("Using filename pattern search for JPG query: %s", searchPattern)
		}

		rows, err = db.Query(query, options.SourcePrefix, options.SourcePrefix, searchPattern)
	} else {
		// Standard query for other file types
		rows, err = db.Query(`SELECT path, source_prefix, average_hash, perceptual_hash, format FROM images 
			WHERE source_prefix = ? OR ? = ''`, options.SourcePrefix, options.SourcePrefix)
	}

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
	var rawProcessed int
	startTime := time.Now()

	for rows.Next() {
		var path, sourcePrefix, dbAvgHash, dbPHash, format string
		err := rows.Scan(&path, &sourcePrefix, &dbAvgHash, &dbPHash, &format)
		if err != nil {
			if options.DebugMode {
				logging.LogError("Error scanning row: %v", err)
			}
			continue
		}

		processed++

		// Check if the candidate is a RAW file
		isRawCandidate := isRawFormat(path)
		if isRawCandidate {
			rawProcessed++
		}

		// Calculate hamming distance
		avgHashDistance := CalculateHammingDistance(avgHash, dbAvgHash)
		pHashDistance := CalculateHammingDistance(pHash, dbPHash)

		// Determine thresholds based on file types
		var avgThreshold, pHashThreshold int

		// Use very generous thresholds for RAW-JPG comparisons
		if (isRawQuery && isJpgFormat(path)) || (isJpgQuery && isRawCandidate) {
			avgThreshold = 20   // Much more lenient
			pHashThreshold = 25 // Much more lenient

			if options.DebugMode {
				logging.DebugLog("Using very lenient thresholds for RAW-JPG comparison between %s and %s",
					options.QueryPath, path)
			}
		} else if isRawQuery || isRawCandidate {
			// Somewhat lenient for other RAW-involved comparisons
			avgThreshold = 15
			pHashThreshold = 18
		} else {
			// Standard thresholds for normal image comparisons
			avgThreshold = 10
			pHashThreshold = 12
		}

		// Special handling for filename-based matching
		if isJpgQuery && isRawCandidate {
			// Check if the filenames suggest they're related
			if areFilenamesRelated(options.QueryPath, path) {
				if options.DebugMode {
					logging.DebugLog("Filename relationship detected between %s and %s, forcing comparison",
						options.QueryPath, path)
				}

				// For related filenames, force SSIM comparison regardless of hash distance
				avgThreshold = 64   // Essentially bypassing the check (8x8 hash has max 64 bits)
				pHashThreshold = 64 // Essentially bypassing the check
			}
		}

		// If hash distance is within threshold, compute SSIM for more accurate comparison
		if avgHashDistance <= avgThreshold || pHashDistance <= pHashThreshold {
			if options.DebugMode {
				if isRawCandidate || isRawQuery {
					logging.DebugLog("RAW image potential match found: %s (avgHashDist: %d/%d, pHashDist: %d/%d)",
						path, avgHashDistance, avgThreshold, pHashDistance, pHashThreshold)
				} else {
					logging.DebugLog("Potential match found: %s (avgHashDist: %d/%d, pHashDist: %d/%d)",
						path, avgHashDistance, avgThreshold, pHashDistance, pHashThreshold)
				}
			}

			wg.Add(1)
			semaphore <- struct{}{}

			go func(p, prefix, fmt string) {
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

				// Adjust threshold for RAW-JPG comparisons
				localThreshold := options.Threshold

				// Use a more lenient SSIM threshold for RAW-JPG comparisons
				if (isRawQuery && isJpgFormat(p)) || (isJpgQuery && isRawFormat(p)) {
					// Lower the threshold by 20% for RAW-JPG comparisons
					localThreshold = options.Threshold * 0.8

					if options.DebugMode {
						logging.DebugLog("Using reduced SSIM threshold of %.2f for RAW-JPG comparison with %s",
							localThreshold, p)
					}
				}

				ssimScore := ComputeSSIM(queryImg, candidateImg)

				// If SSIM score is above threshold, add to matches
				if ssimScore >= localThreshold {
					if options.DebugMode {
						logging.DebugLog("Match confirmed: %s (SSIM: %.4f >= %.4f)",
							p, ssimScore, localThreshold)
					}

					match := types.ImageMatch{
						Path:         p,
						SourcePrefix: prefix,
						SSIMScore:    ssimScore,
					}

					mutex.Lock()
					matches = append(matches, match)
					mutex.Unlock()
				} else if options.DebugMode && (isRawQuery || isRawFormat(p)) {
					logging.DebugLog("RAW image match rejected: %s (SSIM: %.4f < %.4f)",
						p, ssimScore, localThreshold)
				}
			}(path, sourcePrefix, format)
		}

		// Log progress every 100 images in debug mode
		if options.DebugMode && processed%100 == 0 {
			elapsed := time.Since(startTime)
			logging.DebugLog("Search progress: %d images processed (%d RAW) in %v",
				processed, rawProcessed, elapsed)
		}
	}

	wg.Wait()

	if options.DebugMode {
		logging.DebugLog("Search completed. Total images processed: %d (%d RAW), Matches found: %d",
			processed, rawProcessed, len(matches))
	}

	// Sort matches by SSIM score (higher is better)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].SSIMScore > matches[j].SSIMScore
	})

	return matches, nil
}

// Helper to check if a file is in JPG format
func isJpgFormat(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".jpg" || ext == ".jpeg"
}

// Extract the base filename without extension and path
func getBaseFilename(path string) string {
	// Get just the filename without the directory
	filename := filepath.Base(path)
	// Remove the extension
	return strings.TrimSuffix(filename, filepath.Ext(filename))
}

// Check if two filenames are likely related (e.g., IMG_1234.NEF and IMG_1234.JPG)
func areFilenamesRelated(path1, path2 string) bool {
	base1 := getBaseFilename(path1)
	base2 := getBaseFilename(path2)

	// Direct match
	if base1 == base2 {
		return true
	}

	// Check for common patterns where JPG exports get renamed
	// 1. Some software adds suffixes like "_edited" or "-edited"
	if strings.HasPrefix(base1, base2) || strings.HasPrefix(base2, base1) {
		return true
	}

	// 2. Check for the same numeric part (cameras often use numeric names)
	digits1 := extractDigits(base1)
	digits2 := extractDigits(base2)

	if digits1 != "" && digits2 != "" && digits1 == digits2 {
		return true
	}

	return false
}

// Extract just the digits from a string
func extractDigits(s string) string {
	var result strings.Builder
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			result.WriteRune(ch)
		}
	}
	return result.String()
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

func (l *RawImageLoader) tryCR3(path string, tempFilename string) (bool, gocv.Mat) {
	// CR3 files often need different handling

	// Try with exiftool to extract preview image (often works for CR3)
	cmd := exec.Command("exiftool", "-b", "-PreviewImage", path)

	outFile, err := os.Create(tempFilename)
	if err != nil {
		logging.LogWarning("Failed to create temp file for CR3 conversion: %v", err)
		return false, gocv.NewMat()
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	err = cmd.Run()

	if err == nil {
		// Check if file has content
		info, err := os.Stat(tempFilename)
		if err == nil && info.Size() > 0 {
			img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
			if !img.Empty() {
				return true, img
			}
		}
	}

	// Alternative approach using newer versions of libraw
	cmd = exec.Command("libraw_unpack", "-O", tempFilename, path)
	err = cmd.Run()
	if err == nil {
		img := gocv.IMRead(tempFilename, gocv.IMReadGrayScale)
		if !img.Empty() {
			return true, img
		}
	}

	return false, gocv.NewMat()
}
