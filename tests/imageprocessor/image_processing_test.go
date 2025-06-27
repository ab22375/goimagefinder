package imageprocessor_test

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"imagefinder/imageprocessor"
)

// Helper function to create a test image
func createTestImage(width, height int, colors ...color.Color) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	if len(colors) == 0 {
		// Default: gradient from black to white
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				gray := uint8((x + y) * 255 / (width + height))
				img.Set(x, y, color.RGBA{gray, gray, gray, 255})
			}
		}
	} else {
		// Use provided colors in quadrants
		halfW, halfH := width/2, height/2
		for i, c := range colors {
			if i >= 4 {
				break
			}
			x0 := (i % 2) * halfW
			y0 := (i / 2) * halfH
			rect := image.Rect(x0, y0, x0+halfW, y0+halfH)
			draw.Draw(img, rect, &image.Uniform{c}, image.Point{}, draw.Src)
		}
	}
	
	return img
}

// Helper function to save test image
func saveTestImage(t *testing.T, img image.Image, path string) {
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create test image file: %v", err)
	}
	defer file.Close()
	
	err = jpeg.Encode(file, img, &jpeg.Options{Quality: 90})
	if err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}
}

func TestLoadImage(t *testing.T) {
	tempDir := t.TempDir()
	testImagePath := filepath.Join(tempDir, "test.jpg")
	
	// Create and save test image
	testImg := createTestImage(100, 100)
	saveTestImage(t, testImg, testImagePath)
	
	// Test loading
	mat, err := imageprocessor.LoadImage(testImagePath)
	if err != nil {
		t.Fatalf("Failed to load image: %v", err)
	}
	defer mat.Close()
	
	if mat.Empty() {
		t.Error("Loaded image is empty")
	}
	
	// Test loading non-existent file
	_, err = imageprocessor.LoadImage(filepath.Join(tempDir, "nonexistent.jpg"))
	if err == nil {
		t.Error("Expected error when loading non-existent file")
	}
}

func TestComputeAverageHash(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create two similar images
	img1 := createTestImage(100, 100, color.Black, color.White, color.Black, color.White)
	img2 := createTestImage(100, 100, color.Black, color.White, color.Black, color.Gray{128})
	
	path1 := filepath.Join(tempDir, "similar1.jpg")
	path2 := filepath.Join(tempDir, "similar2.jpg")
	
	saveTestImage(t, img1, path1)
	saveTestImage(t, img2, path2)
	
	// Load and compute hashes
	mat1, _ := imageprocessor.LoadImage(path1)
	mat2, _ := imageprocessor.LoadImage(path2)
	defer mat1.Close()
	defer mat2.Close()
	
	hash1, err := imageprocessor.ComputeAverageHash(mat1)
	if err != nil {
		t.Fatalf("Failed to compute average hash 1: %v", err)
	}
	
	hash2, err := imageprocessor.ComputeAverageHash(mat2)
	if err != nil {
		t.Fatalf("Failed to compute average hash 2: %v", err)
	}
	
	// Hashes should be similar but not identical
	if hash1 == "" || hash2 == "" {
		t.Error("Hash should not be empty")
	}
	
	if hash1 == hash2 {
		t.Log("Hashes are identical (might happen for very similar images)")
	}
	
	// Test with very different image
	img3 := createTestImage(100, 100, color.White, color.Black, color.White, color.Black)
	path3 := filepath.Join(tempDir, "different.jpg")
	saveTestImage(t, img3, path3)
	
	mat3, _ := imageprocessor.LoadImage(path3)
	defer mat3.Close()
	
	hash3, err := imageprocessor.ComputeAverageHash(mat3)
	if err != nil {
		t.Fatalf("Failed to compute average hash 3: %v", err)
	}
	
	// This should be very different from hash1
	if hash1 == hash3 {
		t.Error("Very different images should have different hashes")
	}
}

func TestComputePerceptualHash(t *testing.T) {
	tempDir := t.TempDir()
	
	// Create test image
	img := createTestImage(100, 100)
	path := filepath.Join(tempDir, "test.jpg")
	saveTestImage(t, img, path)
	
	// Load and compute hash
	mat, _ := imageprocessor.LoadImage(path)
	defer mat.Close()
	
	hash, err := imageprocessor.ComputePerceptualHash(mat)
	if err != nil {
		t.Fatalf("Failed to compute perceptual hash: %v", err)
	}
	
	if hash == "" {
		t.Error("Perceptual hash should not be empty")
	}
	
	// Hash should be 16 characters (64-bit hex)
	if len(hash) != 16 {
		t.Errorf("Expected hash length 16, got %d", len(hash))
	}
}

func TestIsImageFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"image.jpg", true},
		{"photo.JPEG", true},
		{"pic.png", true},
		{"raw.cr2", true},
		{"raw.CR3", true},
		{"document.pdf", false},
		{"text.txt", false},
		{"", false},
	}
	
	for _, tt := range tests {
		result := imageprocessor.IsImageFile(tt.path)
		if result != tt.expected {
			t.Errorf("IsImageFile(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

func TestIsRawFormat(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"photo.cr2", true},
		{"photo.CR3", true},
		{"photo.nef", true},
		{"photo.NEF", true},
		{"photo.arw", true},
		{"photo.jpg", false},
		{"photo.png", false},
		{"photo.tiff", false},
	}
	
	for _, tt := range tests {
		result := imageprocessor.IsRawFormat(tt.path)
		if result != tt.expected {
			t.Errorf("IsRawFormat(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

func TestIsTiffFormat(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"photo.tiff", true},
		{"photo.TIFF", true},
		{"photo.tif", true},
		{"photo.TIF", true},
		{"photo.jpg", false},
		{"photo.cr2", false},
	}
	
	for _, tt := range tests {
		result := imageprocessor.IsTiffFormat(tt.path)
		if result != tt.expected {
			t.Errorf("IsTiffFormat(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}

func TestFindSimilarImages(t *testing.T) {
	// This test verifies the search functionality structure
	// In a real test, you would:
	// 1. Create a test database with known images
	// 2. Create a query image
	// 3. Search for similar images
	// 4. Verify the results
	
	t.Log("FindSimilarImages test placeholder - requires database setup")
}

func TestImageLoaderRegistry(t *testing.T) {
	registry := imageprocessor.NewImageLoaderRegistry()
	
	// Test that registry is created
	if registry == nil {
		t.Fatal("Failed to create image loader registry")
	}
	
	// Test that standard formats have loaders
	formats := []string{".jpg", ".jpeg", ".png", ".cr2", ".cr3", ".nef"}
	
	for _, format := range formats {
		loader := registry.GetLoader(format)
		if loader == nil {
			t.Errorf("No loader registered for format %s", format)
		}
	}
}

func TestHashComparison(t *testing.T) {
	// Test the hash comparison logic
	tempDir := t.TempDir()
	
	// Create identical images
	img := createTestImage(100, 100, color.Black, color.White, color.Black, color.White)
	path1 := filepath.Join(tempDir, "identical1.jpg")
	path2 := filepath.Join(tempDir, "identical2.jpg")
	
	saveTestImage(t, img, path1)
	saveTestImage(t, img, path2)
	
	// Load and compute hashes
	mat1, _ := imageprocessor.LoadImage(path1)
	mat2, _ := imageprocessor.LoadImage(path2)
	defer mat1.Close()
	defer mat2.Close()
	
	hash1, _ := imageprocessor.ComputeAverageHash(mat1)
	hash2, _ := imageprocessor.ComputeAverageHash(mat2)
	
	// Identical images should have identical hashes
	if hash1 != hash2 {
		t.Errorf("Identical images have different hashes: %s vs %s", hash1, hash2)
	}
}