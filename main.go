package main

import (
	"database/sql"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"gocv.io/x/gocv"
)

// ImageInfo holds the image metadata and features
type ImageInfo struct {
	ID             int64  `json:"id"`
	Path           string `json:"path"`
	SourcePrefix   string `json:"source_prefix"`
	Format         string `json:"format"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	CreatedAt      string `json:"created_at"`
	ModifiedAt     string `json:"modified_at"`
	Size           int64  `json:"size"`
	AverageHash    string `json:"average_hash"`
	PerceptualHash string `json:"perceptual_hash"`
}

// ImageMatch holds the similarity scores
type ImageMatch struct {
	Path         string
	SourcePrefix string
	SSIMScore    float64
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

// LoadImage tries to load an image using the appropriate loader
func (r *ImageLoaderRegistry) LoadImage(path string) (gocv.Mat, error) {
	for _, loader := range r.loaders {
		if loader.CanLoad(path) {
			return loader.LoadImage(path)
		}
	}
	return gocv.NewMat(), fmt.Errorf("no suitable loader found for image: %s", path)
}

// Load image in grayscale with error handling
func loadImage(path string) (gocv.Mat, error) {
	registry := NewImageLoaderRegistry()
	return registry.LoadImage(path)
}

// Database operations
func initDatabase(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, err
	}

	// Create table if it doesn't exist
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS images (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT NOT NULL,
		source_prefix TEXT,
		width INTEGER,
		height INTEGER,
		created_at TEXT,
		modified_at TEXT,
		size INTEGER,
		average_hash TEXT,
		perceptual_hash TEXT,
		features BLOB,
		UNIQUE(path, source_prefix)
	);
	CREATE INDEX IF NOT EXISTS idx_path ON images(path);
	CREATE INDEX IF NOT EXISTS idx_average_hash ON images(average_hash);
	CREATE INDEX IF NOT EXISTS idx_perceptual_hash ON images(perceptual_hash);`

	_, err = db.Exec(createTableSQL)
	if err != nil {
		return nil, err
	}

	// Check if format column exists, add it if it doesn't
	var hasFormatColumn bool
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('images') WHERE name='format'").Scan(&hasFormatColumn)
	if err != nil {
		return nil, fmt.Errorf("error checking for format column: %v", err)
	}

	if !hasFormatColumn {
		// Add format column to existing table
		_, err = db.Exec("ALTER TABLE images ADD COLUMN format TEXT;")
		if err != nil {
			return nil, fmt.Errorf("error adding format column: %v", err)
		}
		fmt.Println("Added 'format' column to existing database schema")
	}

	// Check if source_prefix column exists, add it if it doesn't
	var hasSourcePrefixColumn bool
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('images') WHERE name='source_prefix'").Scan(&hasSourcePrefixColumn)
	if err != nil {
		return nil, fmt.Errorf("error checking for source_prefix column: %v", err)
	}

	if !hasSourcePrefixColumn {
		// Add source_prefix column to existing table
		_, err = db.Exec("ALTER TABLE images ADD COLUMN source_prefix TEXT;")
		if err != nil {
			return nil, fmt.Errorf("error adding source_prefix column: %v", err)
		}
		fmt.Println("Added 'source_prefix' column to existing database schema")

		// If we're adding this column to an existing DB, we need to
		// update the uniqueness constraint (can't directly modify in SQLite)
		// In a real app, you'd create a new table and migrate the data
		fmt.Println("Note: To fully update schema, consider rebuilding the database.")
	}

	return db, nil
}

// Compute average hash for image indexing
func computeAverageHash(img gocv.Mat) (string, error) {
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

// Compute perceptual hash (pHash) for better matching
func computePerceptualHash(img gocv.Mat) (string, error) {
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

	// We've already computed the region values in the code above
	// No need to extract additional values

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

// Simplified and more robust SSIM implementation
func computeSSIM(img1, img2 gocv.Mat) float64 {
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

// Process and store image information in the database
func processAndStoreImage(db *sql.DB, path string, sourcePrefix string, wg *sync.WaitGroup, errChan chan<- error, semaphore chan struct{}, forceRewrite bool) {
	defer wg.Done()
	defer func() { <-semaphore }() // Release semaphore when done

	// Check if image already exists in database
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM images WHERE path = ? AND source_prefix = ?", path, sourcePrefix).Scan(&count)
	if err != nil {
		errChan <- fmt.Errorf("database error for %s: %v", path, err)
		return
	}

	if count > 0 && !forceRewrite {
		// Image already indexed, check if it needs update
		fileInfo, err := os.Stat(path)
		if err != nil {
			errChan <- fmt.Errorf("cannot stat file %s: %v", path, err)
			return
		}

		var storedModTime string
		err = db.QueryRow("SELECT modified_at FROM images WHERE path = ? AND source_prefix = ?", path, sourcePrefix).Scan(&storedModTime)
		if err != nil {
			errChan <- fmt.Errorf("cannot get modified time for %s: %v", path, err)
			return
		}

		// Parse stored time and compare with file modified time
		storedTime, err := time.Parse(time.RFC3339, storedModTime)
		if err != nil {
			errChan <- fmt.Errorf("cannot parse stored time for %s: %v", path, err)
			return
		}

		// If file hasn't been modified, skip processing
		if !fileInfo.ModTime().After(storedTime) {
			return
		}
	}

	// Load and process the image
	img, err := loadImage(path)
	if err != nil {
		errChan <- err
		return
	}
	defer img.Close()

	// Get file info
	fileInfo, err := os.Stat(path)
	if err != nil {
		errChan <- fmt.Errorf("cannot stat file %s: %v", path, err)
		return
	}

	// Compute hashes
	avgHash, err := computeAverageHash(img)
	if err != nil {
		errChan <- fmt.Errorf("cannot compute average hash for %s: %v", path, err)
		return
	}

	pHash, err := computePerceptualHash(img)
	if err != nil {
		errChan <- fmt.Errorf("cannot compute perceptual hash for %s: %v", path, err)
		return
	}

	// Store in database
	now := time.Now().Format(time.RFC3339)
	modTime := fileInfo.ModTime().Format(time.RFC3339)

	// Get file format from extension
	fileFormat := strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")

	// Prepare statement to avoid SQL injection
	var stmt *sql.Stmt
	var insertErr error

	if forceRewrite {
		// Always use INSERT OR REPLACE when force rewrite is enabled
		stmt, insertErr = db.Prepare(`
			INSERT OR REPLACE INTO images (
				path, source_prefix, format, width, height, created_at, modified_at, size, average_hash, perceptual_hash
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
	} else {
		// Use conditional insert or update
		stmt, insertErr = db.Prepare(`
			INSERT OR IGNORE INTO images (
				path, source_prefix, format, width, height, created_at, modified_at, size, average_hash, perceptual_hash
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`)
	}

	if insertErr != nil {
		errChan <- fmt.Errorf("cannot prepare statement for %s: %v", path, insertErr)
		return
	}
	defer stmt.Close()

	_, err = stmt.Exec(
		path,
		sourcePrefix,
		fileFormat,
		img.Cols(),
		img.Rows(),
		now,
		modTime,
		fileInfo.Size(),
		avgHash,
		pHash,
	)

	if err != nil {
		errChan <- fmt.Errorf("cannot insert data for %s: %v", path, err)
		return
	}
}

// Scan folder and store image information in database
func scanAndStoreFolder(db *sql.DB, folderPath string, sourcePrefix string, forceRewrite bool) error {
	var wg sync.WaitGroup

	// Channel for collecting errors
	errorsChan := make(chan error, 100)

	// Semaphore to limit concurrent goroutines
	semaphore := make(chan struct{}, 8)

	// Count total files before starting
	var totalFiles int
	registry := NewImageLoaderRegistry()

	filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if any loader can handle this file
		for _, loader := range registry.loaders {
			if loader.CanLoad(path) {
				totalFiles++
				break
			}
		}
		return nil
	})

	fmt.Printf("Starting image indexing...\nTotal image files to process: %d\n", totalFiles)
	fmt.Printf("Force rewrite mode: %v\n", forceRewrite)
	if sourcePrefix != "" {
		fmt.Printf("Source prefix: %s\n", sourcePrefix)
	}

	// Create a ticker for progress indicator
	ticker := time.NewTicker(500 * time.Millisecond)
	done := make(chan bool)
	processed := 0
	var mu sync.Mutex

	go func() {
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				mu.Lock()
				fmt.Printf("\rProgress: %d/%d", processed, totalFiles)
				mu.Unlock()
			}
		}
	}()

	// Process files
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Check if we have a loader for this file
		registry := NewImageLoaderRegistry()
		canLoad := false
		for _, loader := range registry.loaders {
			if loader.CanLoad(path) {
				canLoad = true
				break
			}
		}
		if !canLoad {
			return nil
		}

		wg.Add(1)
		// Acquire semaphore
		semaphore <- struct{}{}

		go func(p string) {
			processAndStoreImage(db, p, sourcePrefix, &wg, errorsChan, semaphore, forceRewrite)
			mu.Lock()
			processed++
			mu.Unlock()
		}(path)

		return nil
	})

	// Wait for all goroutines to complete
	wg.Wait()
	close(errorsChan)

	// Stop the progress indicator
	ticker.Stop()
	done <- true
	fmt.Println("\nIndexing complete.")

	// Check for errors
	var errorCount int
	for err := range errorsChan {
		log.Printf("Warning: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		fmt.Printf("Encountered %d errors during indexing.\n", errorCount)
	}

	return err
}

// Find similar images in the database
func findSimilarImages(db *sql.DB, queryPath string, threshold float64, sourcePrefix string) ([]ImageMatch, error) {
	queryImg, err := loadImage(queryPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load query image: %v", err)
	}
	defer queryImg.Close()

	// Compute hashes for query image
	avgHash, err := computeAverageHash(queryImg)
	if err != nil {
		return nil, fmt.Errorf("cannot compute average hash: %v", err)
	}

	pHash, err := computePerceptualHash(queryImg)
	if err != nil {
		return nil, fmt.Errorf("cannot compute perceptual hash: %v", err)
	}

	// Prepare SQL query
	var query string
	var args []interface{}

	if sourcePrefix != "" {
		// Filter by source prefix if specified
		query = `SELECT path, source_prefix, average_hash, perceptual_hash FROM images WHERE source_prefix = ?`
		args = []interface{}{sourcePrefix}
	} else {
		// No source prefix filter
		query = `SELECT path, source_prefix, average_hash, perceptual_hash FROM images`
	}

	// Query database for potential matches based on hash similarity
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("database query error: %v", err)
	}
	defer rows.Close()

	var matches []ImageMatch
	var wg sync.WaitGroup
	var mutex sync.Mutex
	semaphore := make(chan struct{}, 8)

	for rows.Next() {
		var path, sourcePrefix, dbAvgHash, dbPHash string
		err := rows.Scan(&path, &sourcePrefix, &dbAvgHash, &dbPHash)
		if err != nil {
			return nil, fmt.Errorf("error scanning row: %v", err)
		}

		// Calculate hamming distance (number of different bits)
		var avgHashDistance, pHashDistance int
		for i := 0; i < len(avgHash) && i < len(dbAvgHash); i++ {
			if avgHash[i] != dbAvgHash[i] {
				avgHashDistance++
			}
		}

		for i := 0; i < len(pHash) && i < len(dbPHash); i++ {
			if pHash[i] != dbPHash[i] {
				pHashDistance++
			}
		}

		// If hash distance is within threshold, compute SSIM for more accurate comparison
		if avgHashDistance <= 10 || pHashDistance <= 12 { // Adjustable thresholds
			wg.Add(1)
			semaphore <- struct{}{}

			go func(p, prefix string) {
				defer wg.Done()
				defer func() { <-semaphore }()

				// Check if file still exists
				_, err := os.Stat(p)
				if err != nil {
					// File doesn't exist, skip SSIM computation
					// In a real app, you might want to note this for the user
					if !os.IsNotExist(err) {
						log.Printf("Warning: Error checking file %s: %v", p, err)
					}
					return
				}

				// Load candidate image and compute SSIM
				candidateImg, err := loadImage(p)
				if err != nil {
					log.Printf("Warning: Failed to load candidate image %s: %v", p, err)
					return
				}
				defer candidateImg.Close()

				ssimScore := computeSSIM(queryImg, candidateImg)

				// If SSIM score is above threshold, add to matches
				if ssimScore >= threshold {
					match := ImageMatch{
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
	}

	wg.Wait()

	// Sort matches by SSIM score (higher is better)
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].SSIMScore > matches[j].SSIMScore
	})

	return matches, nil
}

func parseArguments() map[string]string {
	args := make(map[string]string)

	// First, identify the command (scan/search)
	command := ""
	commandIndex := -1
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "scan" || os.Args[i] == "search" {
			command = os.Args[i]
			commandIndex = i
			break
		}
	}

	if command != "" {
		args["command"] = command
	}

	// Process all arguments, skipping the command
	for i := 1; i < len(os.Args); i++ {
		if i == commandIndex {
			continue
		}

		arg := os.Args[i]

		// Handle flags with equals sign (--key=value)
		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			flagName := strings.TrimPrefix(parts[0], "--")
			args[flagName] = parts[1]
			continue
		}

		// Handle flags without equals sign (--key value)
		if strings.HasPrefix(arg, "--") {
			flagName := strings.TrimPrefix(arg, "--")

			// Check if this is a boolean flag (no value)
			if i+1 >= len(os.Args) || strings.HasPrefix(os.Args[i+1], "--") {
				args[flagName] = "true"
			} else {
				// The next argument is the value
				args[flagName] = os.Args[i+1]
				i++ // Skip the value in the next iteration
			}
		}
	}

	return args
}

func getDefaultDatabasePath() string {
	// Get the executable path
	exePath, err := os.Executable()
	if err != nil {
		// Fallback to current directory if executable path can't be determined
		return "images.db"
	}

	// Get the directory containing the executable
	exeDir := filepath.Dir(exePath)

	// Return the default database path in the same directory
	return filepath.Join(exeDir, "images.db")
}

func main() {
	// Parse command line arguments into a map
	args := parseArguments()

	// Get the command (scan or search)
	command, hasCommand := args["command"]

	// Set default database path
	dbPath := getDefaultDatabasePath()
	if customDB, ok := args["database"]; ok && customDB != "" {
		dbPath = customDB
	} else if customDB, ok := args["db"]; ok && customDB != "" {
		// Allow --db as an alias for --database
		dbPath = customDB
	}

	// Check if required arguments are missing
	showUsage := !hasCommand

	if hasCommand && command == "scan" && args["folder"] == "" {
		showUsage = true
	}

	if hasCommand && command == "search" && args["image"] == "" {
		showUsage = true
	}

	// Show usage if required arguments are missing
	if showUsage {
		fmt.Printf("Usage:\n")
		fmt.Printf("  %s scan --folder=PATH [--database=PATH] [--prefix=NAME] [--force]\n", os.Args[0])
		fmt.Printf("  %s search --image=PATH [--database=PATH] [--threshold=VALUE] [--prefix=NAME]\n", os.Args[0])
		fmt.Printf("\nParameters:\n")
		fmt.Printf("  --folder      : Path to folder containing images to scan\n")
		fmt.Printf("  --image       : Path to query image for search\n")
		fmt.Printf("  --database    : Path to database file (default: %s)\n", getDefaultDatabasePath())
		fmt.Printf("  --prefix      : Source prefix for scanning/filtering results\n")
		fmt.Printf("  --force       : Force rewrite existing entries during scan\n")
		fmt.Printf("  --threshold   : Similarity threshold for search (0.0-1.0, default: 0.8)\n")
		fmt.Printf("\nExamples:\n")
		fmt.Printf("  %s scan --folder=/path/to/images --prefix=ExternalDrive1\n", os.Args[0])
		fmt.Printf("  %s search --image=/path/to/query.jpg --threshold=0.85\n", os.Args[0])
		os.Exit(1)
	}

	switch command {
	case "scan":
		// Get folder path
		folderPath, hasFolder := args["folder"]
		if !hasFolder {
			fmt.Println("Error: Missing folder path (use --folder=PATH)")
			os.Exit(1)
		}

		// Get source prefix
		sourcePrefix := ""
		if prefix, ok := args["prefix"]; ok {
			sourcePrefix = prefix
		}

		// Get force rewrite flag
		forceRewrite := false
		if _, ok := args["force"]; ok {
			forceRewrite = true
		}

		// Verify folder path exists
		if _, err := os.Stat(folderPath); os.IsNotExist(err) {
			log.Fatalf("Folder path does not exist: %s", folderPath)
		}

		startTime := time.Now()

		// Initialize database
		db, err := initDatabase(dbPath)
		if err != nil {
			log.Fatalf("Error initializing database: %v", err)
		}
		defer db.Close()

		// Scan folder and store image information
		err = scanAndStoreFolder(db, folderPath, sourcePrefix, forceRewrite)
		if err != nil {
			log.Fatalf("Error scanning folder: %v", err)
		}

		// Print execution time
		duration := time.Since(startTime)
		fmt.Printf("\nTotal execution time: %v\n", duration)

	case "search":
		// Get query image path
		queryPath, hasQuery := args["image"]
		if !hasQuery {
			fmt.Println("Error: Missing query image path (use --image=PATH)")
			os.Exit(1)
		}

		// Set custom threshold if provided
		threshold := 0.8 // Default threshold
		if thresholdStr, ok := args["threshold"]; ok {
			parsedThreshold, err := strconv.ParseFloat(thresholdStr, 64)
			if err == nil && parsedThreshold >= 0 && parsedThreshold <= 1 {
				threshold = parsedThreshold
			} else {
				fmt.Printf("Warning: Invalid threshold value '%s', using default (0.8)\n", thresholdStr)
			}
		}

		// Get source prefix for filtering
		var sourcePrefix string
		if prefix, ok := args["prefix"]; ok {
			sourcePrefix = prefix
		}

		// Verify paths exist
		if _, err := os.Stat(queryPath); os.IsNotExist(err) {
			log.Fatalf("Query image does not exist: %s", queryPath)
		}

		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			log.Fatalf("Database does not exist: %s. Run scan command first.", dbPath)
		}

		startTime := time.Now()

		// Open database
		db, err := sql.Open("sqlite3", dbPath)
		if err != nil {
			log.Fatalf("Error opening database: %v", err)
		}
		defer db.Close()

		fmt.Println("Searching for similar images...")
		if sourcePrefix != "" {
			fmt.Printf("Filtering by source prefix: %s\n", sourcePrefix)
		}

		// Find similar images
		matches, err := findSimilarImages(db, queryPath, threshold, sourcePrefix)
		if err != nil {
			log.Fatalf("Error finding similar images: %v", err)
		}

		// Print top matches
		fmt.Println("\nTop Matches:")
		limit := 5 // Show top 5 matches

		if len(matches) == 0 {
			fmt.Println("No matches found.")
		} else {
			for i := 0; i < limit && i < len(matches); i++ {
				fmt.Printf("%d. Image: %s\n", i+1, matches[i].Path)
				if matches[i].SourcePrefix != "" {
					fmt.Printf("   Source: %s\n", matches[i].SourcePrefix)
				}
				fmt.Printf("   SSIM Score: %.4f\n", matches[i].SSIMScore)
			}
		}

		// Print execution time
		duration := time.Since(startTime)
		fmt.Printf("\nTotal search time: %v\n", duration)

	default:
		fmt.Printf("Unknown command: %s\n", command)
		fmt.Printf("Usage:\n")
		fmt.Printf("  %s scan --folder=PATH [--database=PATH] [--prefix=NAME] [--force]\n", os.Args[0])
		fmt.Printf("  %s search --image=PATH [--database=PATH] [--threshold=VALUE] [--prefix=NAME]\n", os.Args[0])
		os.Exit(1)
	}
}
