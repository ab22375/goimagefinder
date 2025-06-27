package utils

import (
	"database/sql"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"imagefinder/database"
	"imagefinder/types"
)

// TestImageBuilder helps create test images with various properties
type TestImageBuilder struct {
	width  int
	height int
	format string
}

// NewTestImageBuilder creates a new test image builder
func NewTestImageBuilder() *TestImageBuilder {
	return &TestImageBuilder{
		width:  100,
		height: 100,
		format: "jpg",
	}
}

// WithSize sets the image dimensions
func (b *TestImageBuilder) WithSize(width, height int) *TestImageBuilder {
	b.width = width
	b.height = height
	return b
}

// WithFormat sets the image format
func (b *TestImageBuilder) WithFormat(format string) *TestImageBuilder {
	b.format = format
	return b
}

// CreateSolidColor creates a solid color image
func (b *TestImageBuilder) CreateSolidColor(c color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, b.width, b.height))
	draw.Draw(img, img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
	return img
}

// CreateGradient creates a gradient image
func (b *TestImageBuilder) CreateGradient(horizontal bool) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, b.width, b.height))
	
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			var gray uint8
			if horizontal {
				gray = uint8(x * 255 / b.width)
			} else {
				gray = uint8(y * 255 / b.height)
			}
			img.Set(x, y, color.RGBA{gray, gray, gray, 255})
		}
	}
	
	return img
}

// CreateCheckerboard creates a checkerboard pattern
func (b *TestImageBuilder) CreateCheckerboard(squareSize int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, b.width, b.height))
	
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			if ((x/squareSize)+(y/squareSize))%2 == 0 {
				img.Set(x, y, color.Black)
			} else {
				img.Set(x, y, color.White)
			}
		}
	}
	
	return img
}

// CreateNoise creates a noise pattern
func (b *TestImageBuilder) CreateNoise() image.Image {
	img := image.NewRGBA(image.Rect(0, 0, b.width, b.height))
	
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			// Simple pseudo-random pattern
			r := uint8((x * y * 7) % 256)
			g := uint8((x * y * 13) % 256)
			b := uint8((x * y * 23) % 256)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	
	return img
}

// SaveImage saves an image to disk
func (b *TestImageBuilder) SaveImage(t *testing.T, img image.Image, path string) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	defer file.Close()
	
	switch b.format {
	case "png":
		err = png.Encode(file, img)
	case "jpg", "jpeg":
		err = jpeg.Encode(file, img, &jpeg.Options{Quality: 90})
	default:
		t.Fatalf("Unsupported format: %s", b.format)
	}
	
	if err != nil {
		t.Fatalf("Failed to encode image: %v", err)
	}
}

// TestDatabaseHelper provides utilities for database testing
type TestDatabaseHelper struct {
	db     *sql.DB
	dbPath string
}

// NewTestDatabase creates a new test database
func NewTestDatabase(t *testing.T) *TestDatabaseHelper {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}
	
	return &TestDatabaseHelper{
		db:     db,
		dbPath: dbPath,
	}
}

// GetDB returns the database connection
func (h *TestDatabaseHelper) GetDB() *sql.DB {
	return h.db
}

// GetPath returns the database file path
func (h *TestDatabaseHelper) GetPath() string {
	return h.dbPath
}

// Close closes the database connection
func (h *TestDatabaseHelper) Close() {
	if h.db != nil {
		h.db.Close()
	}
}

// InsertTestImage inserts a test image record
func (h *TestDatabaseHelper) InsertTestImage(t *testing.T, path string, avgHash string, percHash string) {
	imageInfo := types.ImageInfo{
		Path:           path,
		SourcePrefix:   "test",
		Format:         "jpg",
		Width:          100,
		Height:         100,
		CreatedAt:      time.Now().Format(time.RFC3339),
		ModifiedAt:     time.Now().Format(time.RFC3339),
		Size:           1000,
		AverageHash:    avgHash,
		PerceptualHash: percHash,
		IsRawFormat:    false,
	}
	
	if err := database.StoreImageInfo(h.db, imageInfo, false); err != nil {
		t.Fatalf("Failed to insert test image: %v", err)
	}
}

// GetImageCount returns the number of images in the database
func (h *TestDatabaseHelper) GetImageCount(t *testing.T) int {
	var count int
	err := h.db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count images: %v", err)
	}
	return count
}

// ClearImages removes all images from the database
func (h *TestDatabaseHelper) ClearImages(t *testing.T) {
	_, err := h.db.Exec("DELETE FROM images")
	if err != nil {
		t.Fatalf("Failed to clear images: %v", err)
	}
}

// TestFileSystem provides utilities for file system testing
type TestFileSystem struct {
	rootDir string
}

// NewTestFileSystem creates a new test file system
func NewTestFileSystem(t *testing.T) *TestFileSystem {
	return &TestFileSystem{
		rootDir: t.TempDir(),
	}
}

// GetRoot returns the root directory
func (fs *TestFileSystem) GetRoot() string {
	return fs.rootDir
}

// CreateDirectory creates a directory
func (fs *TestFileSystem) CreateDirectory(t *testing.T, relPath string) string {
	fullPath := filepath.Join(fs.rootDir, relPath)
	if err := os.MkdirAll(fullPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	return fullPath
}

// CreateFile creates an empty file
func (fs *TestFileSystem) CreateFile(t *testing.T, relPath string, content []byte) string {
	fullPath := filepath.Join(fs.rootDir, relPath)
	dir := filepath.Dir(fullPath)
	
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("Failed to create parent directory: %v", err)
	}
	
	if err := os.WriteFile(fullPath, content, 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	
	return fullPath
}

// CreateImageGallery creates a test image gallery
func (fs *TestFileSystem) CreateImageGallery(t *testing.T) map[string]string {
	builder := NewTestImageBuilder()
	gallery := make(map[string]string)
	
	// Create different categories
	categories := []struct {
		dir     string
		pattern string
		count   int
	}{
		{"landscapes", "gradient", 3},
		{"portraits", "solid", 3},
		{"abstract", "checkerboard", 3},
		{"misc", "noise", 2},
	}
	
	for _, cat := range categories {
		dir := fs.CreateDirectory(t, cat.dir)
		
		for i := 0; i < cat.count; i++ {
			filename := fmt.Sprintf("image_%d.jpg", i+1)
			path := filepath.Join(dir, filename)
			
			var img image.Image
			switch cat.pattern {
			case "gradient":
				img = builder.CreateGradient(i%2 == 0)
			case "solid":
				colors := []color.Color{color.Black, color.White, color.RGBA{128, 128, 128, 255}}
				img = builder.CreateSolidColor(colors[i%len(colors)])
			case "checkerboard":
				img = builder.CreateCheckerboard(10 * (i + 1))
			case "noise":
				img = builder.CreateNoise()
			}
			
			builder.SaveImage(t, img, path)
			gallery[cat.dir+"/"+filename] = path
		}
	}
	
	return gallery
}

// AssertFileExists checks if a file exists
func AssertFileExists(t *testing.T, path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("File does not exist: %s", path)
	}
}

// AssertFileNotExists checks if a file does not exist
func AssertFileNotExists(t *testing.T, path string) {
	if _, err := os.Stat(path); err == nil {
		t.Errorf("File should not exist: %s", path)
	}
}

// AssertImagesEqual checks if two images have the same dimensions
func AssertImagesEqual(t *testing.T, img1, img2 image.Image) {
	bounds1 := img1.Bounds()
	bounds2 := img2.Bounds()
	
	if bounds1.Dx() != bounds2.Dx() || bounds1.Dy() != bounds2.Dy() {
		t.Errorf("Image dimensions differ: %dx%d vs %dx%d", 
			bounds1.Dx(), bounds1.Dy(), bounds2.Dx(), bounds2.Dy())
	}
}

// CompareHashes compares two hash strings and returns the number of different bits
func CompareHashes(hash1, hash2 string) int {
	if len(hash1) != len(hash2) {
		return -1
	}
	
	differences := 0
	for i := 0; i < len(hash1); i++ {
		if hash1[i] != hash2[i] {
			// Count bit differences in hex characters
			val1 := hexToInt(hash1[i])
			val2 := hexToInt(hash2[i])
			xor := val1 ^ val2
			
			// Count bits in XOR result
			for xor > 0 {
				if xor&1 == 1 {
					differences++
				}
				xor >>= 1
			}
		}
	}
	
	return differences
}

func hexToInt(c byte) int {
	if c >= '0' && c <= '9' {
		return int(c - '0')
	}
	if c >= 'a' && c <= 'f' {
		return int(c - 'a' + 10)
	}
	if c >= 'A' && c <= 'F' {
		return int(c - 'A' + 10)
	}
	return 0
}