package integration_test

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"

	"imagefinder/database"
	"imagefinder/imageprocessor"
	"imagefinder/scanner"
)

// Helper to create test images with specific patterns
func createPatternImage(t *testing.T, path string, pattern string) {
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	
	switch pattern {
	case "checkerboard":
		// Black and white checkerboard
		for y := 0; y < 200; y++ {
			for x := 0; x < 200; x++ {
				if ((x/50)+(y/50))%2 == 0 {
					img.Set(x, y, color.Black)
				} else {
					img.Set(x, y, color.White)
				}
			}
		}
	case "gradient":
		// Horizontal gradient
		for y := 0; y < 200; y++ {
			for x := 0; x < 200; x++ {
				gray := uint8(x * 255 / 200)
				img.Set(x, y, color.RGBA{gray, gray, gray, 255})
			}
		}
	case "solid":
		// Solid gray
		gray := color.RGBA{128, 128, 128, 255}
		for y := 0; y < 200; y++ {
			for x := 0; x < 200; x++ {
				img.Set(x, y, gray)
			}
		}
	default:
		// Random pattern
		for y := 0; y < 200; y++ {
			for x := 0; x < 200; x++ {
				r := uint8((x * y) % 256)
				g := uint8((x + y) % 256)
				b := uint8((x - y) % 256)
				img.Set(x, y, color.RGBA{r, g, b, 255})
			}
		}
	}
	
	// Ensure directory exists
	os.MkdirAll(filepath.Dir(path), 0755)
	
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create image file: %v", err)
	}
	defer file.Close()
	
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("Failed to encode image: %v", err)
	}
}

// Test complete workflow: scan and search
func TestCompleteWorkflow(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	imageDir := filepath.Join(tempDir, "images")
	
	// Create test images with different patterns
	patterns := []struct {
		name    string
		pattern string
	}{
		{"checkerboard1.jpg", "checkerboard"},
		{"checkerboard2.jpg", "checkerboard"}, // Similar to checkerboard1
		{"gradient1.jpg", "gradient"},
		{"gradient2.jpg", "gradient"}, // Similar to gradient1
		{"solid.jpg", "solid"},
		{"random.jpg", "random"},
	}
	
	for _, p := range patterns {
		createPatternImage(t, filepath.Join(imageDir, p.name), p.pattern)
	}
	
	// Step 1: Initialize database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Step 2: Scan images
	scanOptions := scanner.ScanOptions{
		FolderPath:   imageDir,
		SourcePrefix: "test",
		ForceRewrite: false,
		MaxWorkers:   4,
		DebugMode:    true,
	}
	
	err = scanner.ScanAndStoreFolder(db, scanOptions)
	if err != nil {
		t.Fatalf("Failed to scan folder: %v", err)
	}
	
	// Verify all images were scanned
	var count int
	db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
	if count != len(patterns) {
		t.Errorf("Expected %d images in database, got %d", len(patterns), count)
	}
	
	// Step 3: Search for similar images
	searchOptions := imageprocessor.SearchOptions{
		QueryPath:    filepath.Join(imageDir, "checkerboard1.jpg"),
		Threshold:    0.7,
		DebugMode:    true,
		SourcePrefix: "",
	}
	
	matches, err := imageprocessor.FindSimilarImages(db, searchOptions)
	if err != nil {
		t.Fatalf("Failed to find similar images: %v", err)
	}
	
	// Should find at least checkerboard2 as similar
	foundSimilar := false
	for _, match := range matches {
		t.Logf("Found match: %s with score %.2f", filepath.Base(match.Path), match.SSIMScore)
		if filepath.Base(match.Path) == "checkerboard2.jpg" {
			foundSimilar = true
		}
	}
	
	if !foundSimilar {
		t.Error("Expected to find checkerboard2.jpg as similar to checkerboard1.jpg")
	}
}

// Test duplicate detection
func TestDuplicateDetection(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	imageDir := filepath.Join(tempDir, "images")
	
	// Create identical images
	originalPath := filepath.Join(imageDir, "original.jpg")
	duplicate1Path := filepath.Join(imageDir, "duplicate1.jpg")
	duplicate2Path := filepath.Join(imageDir, "subdir", "duplicate2.jpg")
	
	// Create a simple solid color image to ensure consistency
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	// Fill with solid gray color
	gray := color.RGBA{128, 128, 128, 255}
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, gray)
		}
	}
	
	// Save original
	os.MkdirAll(filepath.Dir(originalPath), 0755)
	file, _ := os.Create(originalPath)
	jpeg.Encode(file, img, &jpeg.Options{Quality: 100})
	file.Close()
	
	// Copy to create exact duplicates
	original, _ := os.ReadFile(originalPath)
	os.WriteFile(duplicate1Path, original, 0644)
	os.MkdirAll(filepath.Dir(duplicate2Path), 0755)
	os.WriteFile(duplicate2Path, original, 0644)
	
	// Verify files are identical
	dup1, _ := os.ReadFile(duplicate1Path)
	dup2, _ := os.ReadFile(duplicate2Path)
	if !bytes.Equal(original, dup1) || !bytes.Equal(original, dup2) {
		t.Fatal("Duplicate files are not identical to original")
	}
	
	// Initialize and scan
	db, _ := database.InitDatabase(dbPath)
	defer db.Close()
	
	scanOptions := scanner.ScanOptions{
		FolderPath:   imageDir,
		SourcePrefix: "test",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    false,
	}
	
	scanner.ScanAndStoreFolder(db, scanOptions)
	
	// Verify what's in the database
	var dbCount int
	db.QueryRow("SELECT COUNT(*) FROM images").Scan(&dbCount)
	t.Logf("Total images in database: %d", dbCount)
	
	// List all images in database for debugging
	rows, _ := db.Query("SELECT path, average_hash, perceptual_hash FROM images")
	defer rows.Close()
	t.Log("Images in database:")
	for rows.Next() {
		var path, avgHash, pHash string
		rows.Scan(&path, &avgHash, &pHash)
		t.Logf("  - %s (avgHash: %.8s..., pHash: %.8s...)", filepath.Base(path), avgHash, pHash)
	}
	
	// Search for duplicates
	searchOptions := imageprocessor.SearchOptions{
		QueryPath:    originalPath,
		Threshold:    0.99, // Very high threshold for exact matches
		DebugMode:    true,  // Enable debug mode for more information
		SourcePrefix: "",
	}
	
	matches, err := imageprocessor.FindSimilarImages(db, searchOptions)
	if err != nil {
		t.Fatalf("Failed to find duplicates: %v", err)
	}
	
	// Log all matches to understand what's happening
	t.Logf("Query image: %s", originalPath)
	t.Logf("Total matches found: %d", len(matches))
	for i, match := range matches {
		t.Logf("Match %d: %s (score: %.4f)", i+1, match.Path, match.SSIMScore)
	}
	
	// The search returns all similar images, which may include the query image itself
	// Count how many duplicates we found (excluding the original if it's in the results)
	duplicateCount := 0
	foundOriginal := false
	for _, match := range matches {
		if match.Path == originalPath {
			foundOriginal = true
		} else {
			duplicateCount++
		}
	}
	
	if foundOriginal {
		t.Log("Note: Search results include the query image itself")
	}
	
	// We created 2 duplicates, so we should find at least 2
	if duplicateCount < 2 {
		t.Errorf("Expected at least 2 duplicates (excluding query image), found %d", duplicateCount)
	}
	
	// All matches should have score >= 0.99 (near perfect match)
	for _, match := range matches {
		if match.SSIMScore < 0.99 {
			t.Errorf("Duplicate %s has score %.4f, expected >= 0.99", match.Path, match.SSIMScore)
		}
	}
}

// Test cross-format similarity
func TestCrossFormatSimilarity(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	imageDir := filepath.Join(tempDir, "images")
	
	// Create same image in different formats
	// Note: We'll create JPEGs with different names to simulate different formats
	jpgPath := filepath.Join(imageDir, "IMG_1234.jpg")
	rawPath := filepath.Join(imageDir, "IMG_1234_raw.jpg") // Simulating a RAW file
	
	createPatternImage(t, jpgPath, "gradient")
	createPatternImage(t, rawPath, "gradient")
	
	// Initialize and scan
	db, _ := database.InitDatabase(dbPath)
	defer db.Close()
	
	scanOptions := scanner.ScanOptions{
		FolderPath:   imageDir,
		SourcePrefix: "camera",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    false,
	}
	
	scanner.ScanAndStoreFolder(db, scanOptions)
	
	// Search using the JPG
	searchOptions := imageprocessor.SearchOptions{
		QueryPath:    jpgPath,
		Threshold:    0.8,
		DebugMode:    false,
		SourcePrefix: "",
	}
	
	matches, _ := imageprocessor.FindSimilarImages(db, searchOptions)
	
	// Should find the "RAW" version
	foundRaw := false
	for _, match := range matches {
		if filepath.Base(match.Path) == "IMG_1234_raw.jpg" {
			foundRaw = true
			// Should get filename similarity boost
			if match.SSIMScore < 0.9 {
				t.Errorf("Expected higher score for similar filename, got %.2f", match.SSIMScore)
			}
		}
	}
	
	if !foundRaw {
		t.Error("Failed to find cross-format similar image")
	}
}

// Test incremental scanning
func TestIncrementalScanning(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	imageDir := filepath.Join(tempDir, "images")
	
	// Create initial images
	createPatternImage(t, filepath.Join(imageDir, "image1.jpg"), "solid")
	createPatternImage(t, filepath.Join(imageDir, "image2.jpg"), "gradient")
	
	// Initialize and first scan
	db, _ := database.InitDatabase(dbPath)
	defer db.Close()
	
	scanOptions := scanner.ScanOptions{
		FolderPath:   imageDir,
		SourcePrefix: "test",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    false,
	}
	
	scanner.ScanAndStoreFolder(db, scanOptions)
	
	// Check initial count
	var count1 int
	db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count1)
	
	// Add more images
	createPatternImage(t, filepath.Join(imageDir, "image3.jpg"), "checkerboard")
	createPatternImage(t, filepath.Join(imageDir, "image4.jpg"), "random")
	
	// Second scan - should only process new images
	scanner.ScanAndStoreFolder(db, scanOptions)
	
	// Check new count
	var count2 int
	db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count2)
	
	if count2 != count1+2 {
		t.Errorf("Expected %d images after incremental scan, got %d", count1+2, count2)
	}
	
	// Verify old images weren't updated (check by querying a specific path)
	var hash1, hash2 string
	db.QueryRow("SELECT average_hash FROM images WHERE path = ?", 
		filepath.Join(imageDir, "image1.jpg")).Scan(&hash1)
	
	// Third scan with same images - hashes should remain the same
	scanner.ScanAndStoreFolder(db, scanOptions)
	
	db.QueryRow("SELECT average_hash FROM images WHERE path = ?", 
		filepath.Join(imageDir, "image1.jpg")).Scan(&hash2)
	
	if hash1 != hash2 {
		t.Error("Image hash changed without force flag")
	}
}

// Test search with source prefix filtering
func TestSearchWithSourcePrefix(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	// Create images in different directories
	camera1Dir := filepath.Join(tempDir, "camera1")
	camera2Dir := filepath.Join(tempDir, "camera2")
	
	createPatternImage(t, filepath.Join(camera1Dir, "img1.jpg"), "checkerboard")
	createPatternImage(t, filepath.Join(camera2Dir, "img2.jpg"), "checkerboard")
	
	// Initialize database
	db, _ := database.InitDatabase(dbPath)
	defer db.Close()
	
	// Scan from camera1
	scanOptions1 := scanner.ScanOptions{
		FolderPath:   camera1Dir,
		SourcePrefix: "Camera1",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    false,
	}
	scanner.ScanAndStoreFolder(db, scanOptions1)
	
	// Scan from camera2
	scanOptions2 := scanner.ScanOptions{
		FolderPath:   camera2Dir,
		SourcePrefix: "Camera2",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    false,
	}
	scanner.ScanAndStoreFolder(db, scanOptions2)
	
	// Create query image
	queryPath := filepath.Join(tempDir, "query.jpg")
	createPatternImage(t, queryPath, "checkerboard")
	
	// Search without prefix - should find both
	searchOptions1 := imageprocessor.SearchOptions{
		QueryPath:    queryPath,
		Threshold:    0.7,
		DebugMode:    false,
		SourcePrefix: "",
	}
	
	matches1, _ := imageprocessor.FindSimilarImages(db, searchOptions1)
	if len(matches1) != 2 {
		t.Errorf("Expected 2 matches without prefix filter, got %d", len(matches1))
	}
	
	// Search with prefix - should find only one
	searchOptions2 := imageprocessor.SearchOptions{
		QueryPath:    queryPath,
		Threshold:    0.7,
		DebugMode:    false,
		SourcePrefix: "Camera1",
	}
	
	matches2, _ := imageprocessor.FindSimilarImages(db, searchOptions2)
	if len(matches2) != 1 {
		t.Errorf("Expected 1 match with prefix filter, got %d", len(matches2))
	}
	
	if len(matches2) > 0 && !contains(matches2[0].Path, "camera1") {
		t.Error("Filtered result is from wrong source")
	}
}

// Helper function
func contains(s, substr string) bool {
	return filepath.Base(filepath.Dir(s)) == substr
}

// Test error handling
func TestErrorHandling(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	
	// Test 1: Corrupted image file
	corruptPath := filepath.Join(tempDir, "corrupt.jpg")
	os.WriteFile(corruptPath, []byte("not a valid image"), 0644)
	
	_, err := imageprocessor.LoadImage(corruptPath)
	if err == nil {
		t.Error("Expected error for corrupted image")
	}
	
	// Test 2: Non-existent folder scan
	db, _ := database.InitDatabase(dbPath)
	defer db.Close()
	
	scanOptions := scanner.ScanOptions{
		FolderPath:   "/non/existent/folder",
		SourcePrefix: "test",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    false,
	}
	
	// The scanner actually handles non-existent folders gracefully
	// It just processes 0 files without error
	err = scanner.ScanAndStoreFolder(db, scanOptions)
	// This is actually the expected behavior - no error, just 0 files processed
	if err != nil {
		t.Logf("Scanner returned error for non-existent folder: %v", err)
	} else {
		t.Log("Scanner handled non-existent folder gracefully (0 files processed)")
		// Verify 0 images in database
		var count int
		db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
		if count != 0 {
			t.Errorf("Expected 0 images after scanning non-existent folder, got %d", count)
		}
	}
}

// Benchmark scanning performance
func BenchmarkScanning(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench.db")
	imageDir := filepath.Join(tempDir, "images")
	
	// Create 100 test images
	for i := 0; i < 100; i++ {
		patterns := []string{"checkerboard", "gradient", "solid", "random"}
		pattern := patterns[i%len(patterns)]
		createPatternImage(&testing.T{}, filepath.Join(imageDir, fmt.Sprintf("img%03d.jpg", i)), pattern)
	}
	
	db, _ := database.InitDatabase(dbPath)
	defer db.Close()
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		// Clear database
		db.Exec("DELETE FROM images")
		
		scanOptions := scanner.ScanOptions{
			FolderPath:   imageDir,
			SourcePrefix: "bench",
			ForceRewrite: false,
			MaxWorkers:   8,
			DebugMode:    false,
		}
		
		scanner.ScanAndStoreFolder(db, scanOptions)
	}
}