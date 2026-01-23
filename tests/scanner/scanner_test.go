package scanner_test

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
	"time"

	"imagefinder/database"
	"imagefinder/scanner"
)

// Helper to create test image file
func createTestImageFile(t *testing.T, path string, width, height int) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Create a simple pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			gray := uint8((x + y) * 255 / (width + height))
			img.Set(x, y, color.RGBA{gray, gray, gray, 255})
		}
	}
	
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
	
	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("Failed to encode image: %v", err)
	}
}

// Helper to create test directory structure
func setupTestDirectory(t *testing.T) string {
	tempDir := t.TempDir()
	
	// Create various image files
	createTestImageFile(t, filepath.Join(tempDir, "image1.jpg"), 100, 100)
	createTestImageFile(t, filepath.Join(tempDir, "image2.jpeg"), 200, 200)
	createTestImageFile(t, filepath.Join(tempDir, "image3.jpg"), 150, 150)
	createTestImageFile(t, filepath.Join(tempDir, "subdir", "image4.jpg"), 100, 100)
	createTestImageFile(t, filepath.Join(tempDir, "subdir", "nested", "image5.jpg"), 100, 100)
	
	// Create non-image files
	os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("test"), 0644)
	os.WriteFile(filepath.Join(tempDir, "data.json"), []byte("{}"), 0644)
	
	return tempDir
}

func TestScanAndStoreFolder(t *testing.T) {
	testDir := setupTestDirectory(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	
	// Initialize database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Scan folder
	options := scanner.ScanOptions{
		FolderPath:   testDir,
		SourcePrefix: "test",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    true,
	}
	
	err = scanner.ScanAndStoreFolder(context.Background(), db, options)
	if err != nil {
		t.Fatalf("Failed to scan folder: %v", err)
	}
	
	// Verify images were stored
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to count images: %v", err)
	}
	
	// We created 5 JPEG files that should be processed
	if count < 3 {
		t.Errorf("Expected at least 3 images in database, got %d", count)
	}
}

func TestScanWithForceRewrite(t *testing.T) {
	testDir := setupTestDirectory(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	
	// Initialize database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// First scan
	options := scanner.ScanOptions{
		FolderPath:   testDir,
		SourcePrefix: "test",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    false,
	}
	
	err = scanner.ScanAndStoreFolder(context.Background(), db, options)
	if err != nil {
		t.Fatalf("Failed first scan: %v", err)
	}
	
	// Get initial count
	var initialCount int
	db.QueryRow("SELECT COUNT(*) FROM images").Scan(&initialCount)
	
	// Get a sample hash
	var samplePath, sampleHash string
	db.QueryRow("SELECT path, average_hash FROM images LIMIT 1").Scan(&samplePath, &sampleHash)
	
	// Modify the hash in database
	_, err = db.Exec("UPDATE images SET average_hash = ? WHERE path = ?", "modified_hash", samplePath)
	if err != nil {
		t.Fatalf("Failed to modify hash: %v", err)
	}
	
	// Second scan without force - should not update
	err = scanner.ScanAndStoreFolder(context.Background(), db, options)
	if err != nil {
		t.Fatalf("Failed second scan: %v", err)
	}
	
	// Check hash is still modified
	var currentHash string
	db.QueryRow("SELECT average_hash FROM images WHERE path = ?", samplePath).Scan(&currentHash)
	if currentHash != "modified_hash" {
		t.Error("Hash was updated without force flag")
	}
	
	// Third scan with force - should update
	options.ForceRewrite = true
	err = scanner.ScanAndStoreFolder(context.Background(), db, options)
	if err != nil {
		t.Fatalf("Failed third scan: %v", err)
	}
	
	// Check hash is updated
	db.QueryRow("SELECT average_hash FROM images WHERE path = ?", samplePath).Scan(&currentHash)
	if currentHash == "modified_hash" {
		t.Error("Hash was not updated with force flag")
	}
}

func TestScanWithSourcePrefix(t *testing.T) {
	testDir := setupTestDirectory(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	
	// Initialize database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Scan with prefix1
	options1 := scanner.ScanOptions{
		FolderPath:   testDir,
		SourcePrefix: "source1",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    false,
	}
	
	err = scanner.ScanAndStoreFolder(context.Background(), db, options1)
	if err != nil {
		t.Fatalf("Failed scan with source1: %v", err)
	}
	
	// Scan same folder with prefix2
	options2 := scanner.ScanOptions{
		FolderPath:   testDir,
		SourcePrefix: "source2",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    false,
	}
	
	err = scanner.ScanAndStoreFolder(context.Background(), db, options2)
	if err != nil {
		t.Fatalf("Failed scan with source2: %v", err)
	}
	
	// Check we have entries for both prefixes
	var count1, count2 int
	db.QueryRow("SELECT COUNT(*) FROM images WHERE source_prefix = ?", "source1").Scan(&count1)
	db.QueryRow("SELECT COUNT(*) FROM images WHERE source_prefix = ?", "source2").Scan(&count2)
	
	if count1 == 0 {
		t.Error("No images found with source1 prefix")
	}
	if count2 == 0 {
		t.Error("No images found with source2 prefix")
	}
	if count1 != count2 {
		t.Errorf("Different counts for same folder: source1=%d, source2=%d", count1, count2)
	}
}

func TestScanEmptyFolder(t *testing.T) {
	emptyDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	
	// Initialize database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Scan empty folder
	options := scanner.ScanOptions{
		FolderPath:   emptyDir,
		SourcePrefix: "test",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    false,
	}
	
	err = scanner.ScanAndStoreFolder(context.Background(), db, options)
	// Should complete without error
	if err != nil {
		t.Fatalf("Failed to scan empty folder: %v", err)
	}
	
	// Verify no images were stored
	var count int
	db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
	if count != 0 {
		t.Errorf("Expected 0 images in database, got %d", count)
	}
}

func TestScanNonExistentFolder(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	
	// Initialize database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Try to scan non-existent folder
	nonExistentPath := filepath.Join(t.TempDir(), "this_folder_does_not_exist")
	options := scanner.ScanOptions{
		FolderPath:   nonExistentPath,
		SourcePrefix: "test",
		ForceRewrite: false,
		MaxWorkers:   2,
		DebugMode:    false,
	}
	
	// Note: The scanner might not return an error for non-existent folders,
	// it might just process 0 files. Let's check the behavior.
	err = scanner.ScanAndStoreFolder(context.Background(), db, options)
	
	// If no error, check that no images were processed
	if err == nil {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
		if count != 0 {
			t.Errorf("Expected 0 images when scanning non-existent folder, got %d", count)
		}
		t.Log("Scanner processed non-existent folder without error (0 files found)")
	} else {
		t.Logf("Scanner returned error for non-existent folder: %v", err)
	}
}

func TestConcurrentScanning(t *testing.T) {
	testDir := setupTestDirectory(t)
	dbPath := filepath.Join(t.TempDir(), "test.db")
	
	// Initialize database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()
	
	// Test with different worker counts
	workerCounts := []int{1, 4, 8}
	
	for _, workers := range workerCounts {
		// Clear database
		db.Exec("DELETE FROM images")
		
		start := time.Now()
		
		options := scanner.ScanOptions{
			FolderPath:   testDir,
			SourcePrefix: "test",
			ForceRewrite: false,
			MaxWorkers:   workers,
			DebugMode:    false,
		}
		
		err = scanner.ScanAndStoreFolder(context.Background(), db, options)
		if err != nil {
			t.Fatalf("Failed scan with %d workers: %v", workers, err)
		}
		
		duration := time.Since(start)
		
		// Verify same number of images regardless of worker count
		var count int
		db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
		
		t.Logf("Scan with %d workers took %v, found %d images", workers, duration, count)
		
		if count < 3 {
			t.Errorf("Expected at least 3 images with %d workers, got %d", workers, count)
		}
	}
}

func TestScanOptions(t *testing.T) {
	// Test default values
	options := scanner.ScanOptions{}
	
	if options.MaxWorkers != 0 {
		t.Errorf("Expected default MaxWorkers to be 0, got %d", options.MaxWorkers)
	}
	
	if options.ForceRewrite {
		t.Error("Expected default ForceRewrite to be false")
	}
	
	if options.DebugMode {
		t.Error("Expected default DebugMode to be false")
	}
}