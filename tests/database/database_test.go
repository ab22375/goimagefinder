package database_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"imagefinder/database"
	"imagefinder/types"
)

func TestInitDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Check if file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Error("Database file was not created")
	}

	// Check if tables exist
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='images'").Scan(&tableName)
	if err != nil {
		t.Error("Images table was not created")
	}
}

func TestOpenDatabase(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// First create database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	db.Close()

	// Now try to open it
	db, err = database.OpenDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Note: SQLite creates the database file when opened, so we can't test for
	// non-existent database error. Instead, let's test opening works.
	nonExistentPath := filepath.Join(tempDir, "new.db")
	db2, err := database.OpenDatabase(nonExistentPath)
	if err != nil {
		t.Errorf("Failed to open new database: %v", err)
	}
	if db2 != nil {
		db2.Close()
		os.Remove(nonExistentPath)
	}
}

func TestStoreImageInfo(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	imageInfo := types.ImageInfo{
		Path:           "/test/image.jpg",
		SourcePrefix:   "test",
		Format:         "jpg",
		Width:          1920,
		Height:         1080,
		CreatedAt:      time.Now().Format(time.RFC3339),
		ModifiedAt:     time.Now().Format(time.RFC3339),
		Size:           1024000,
		AverageHash:    "abcdef1234567890",
		PerceptualHash: "1234567890abcdef",
		IsRawFormat:    false,
	}

	err = database.StoreImageInfo(db, imageInfo, false)
	if err != nil {
		t.Fatalf("Failed to store image info: %v", err)
	}

	// Verify insertion
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM images WHERE path = ?", imageInfo.Path).Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query inserted image: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 image, got %d", count)
	}
}

func TestStoreDuplicateImage(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	imageInfo := types.ImageInfo{
		Path:           "/test/image.jpg",
		SourcePrefix:   "test",
		Format:         "jpg",
		Width:          1920,
		Height:         1080,
		CreatedAt:      time.Now().Format(time.RFC3339),
		ModifiedAt:     time.Now().Format(time.RFC3339),
		Size:           1024000,
		AverageHash:    "abcdef1234567890",
		PerceptualHash: "1234567890abcdef",
		IsRawFormat:    false,
	}

	// First insertion
	err = database.StoreImageInfo(db, imageInfo, false)
	if err != nil {
		t.Fatalf("Failed to store image info: %v", err)
	}

	// Try to insert same image again without force - should be ignored
	err = database.StoreImageInfo(db, imageInfo, false)
	if err != nil {
		t.Errorf("Unexpected error when inserting duplicate (should be ignored): %v", err)
	}

	// Count should still be 1
	var count int
	db.QueryRow("SELECT COUNT(*) FROM images WHERE path = ?", imageInfo.Path).Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 image after duplicate insert, got %d", count)
	}
}

func TestStoreImageWithForceRewrite(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	imageInfo := types.ImageInfo{
		Path:           "/test/image.jpg",
		SourcePrefix:   "test",
		Format:         "jpg",
		Width:          1920,
		Height:         1080,
		CreatedAt:      time.Now().Format(time.RFC3339),
		ModifiedAt:     time.Now().Format(time.RFC3339),
		Size:           1024000,
		AverageHash:    "abcdef1234567890",
		PerceptualHash: "1234567890abcdef",
		IsRawFormat:    false,
	}

	// First insertion
	err = database.StoreImageInfo(db, imageInfo, false)
	if err != nil {
		t.Fatalf("Failed to store image info: %v", err)
	}

	// Update with force
	imageInfo.Width = 3840
	imageInfo.Height = 2160
	imageInfo.AverageHash = "fedcba0987654321"
	
	err = database.StoreImageInfo(db, imageInfo, true)
	if err != nil {
		t.Fatalf("Failed to update with force: %v", err)
	}

	// Verify update
	var width, height int
	var hash string
	err = db.QueryRow("SELECT width, height, average_hash FROM images WHERE path = ?", imageInfo.Path).
		Scan(&width, &height, &hash)
	if err != nil {
		t.Fatalf("Failed to query updated image: %v", err)
	}

	if width != 3840 || height != 2160 {
		t.Errorf("Dimensions not updated: got %dx%d, expected 3840x2160", width, height)
	}
	if hash != "fedcba0987654321" {
		t.Errorf("Hash not updated: got %s, expected fedcba0987654321", hash)
	}
}

func TestCheckImageExists(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Check non-existent image
	exists, _, err := database.CheckImageExists(db, "/test/nonexistent.jpg", "test")
	if err != nil {
		t.Fatalf("Failed to check image existence: %v", err)
	}
	if exists {
		t.Error("Non-existent image reported as existing")
	}

	// Insert an image
	imageInfo := types.ImageInfo{
		Path:           "/test/image.jpg",
		SourcePrefix:   "test",
		Format:         "jpg",
		Width:          1920,
		Height:         1080,
		CreatedAt:      time.Now().Format(time.RFC3339),
		ModifiedAt:     time.Now().Format(time.RFC3339),
		Size:           1024000,
		AverageHash:    "abcdef1234567890",
		PerceptualHash: "1234567890abcdef",
		IsRawFormat:    false,
	}

	database.StoreImageInfo(db, imageInfo, false)

	// Check existing image
	exists, modTime, err := database.CheckImageExists(db, imageInfo.Path, imageInfo.SourcePrefix)
	if err != nil {
		t.Fatalf("Failed to check image existence: %v", err)
	}
	if !exists {
		t.Error("Existing image not found")
	}
	if modTime != imageInfo.ModifiedAt {
		t.Errorf("Modified time mismatch: got %s, expected %s", modTime, imageInfo.ModifiedAt)
	}
}

func TestQueryPotentialMatches(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Insert test images with different source prefixes
	images := []types.ImageInfo{
		{
			Path:           "/test/image1.jpg",
			SourcePrefix:   "camera1",
			Format:         "jpg",
			Width:          1920,
			Height:         1080,
			ModifiedAt:     time.Now().Format(time.RFC3339),
			Size:           1024000,
			AverageHash:    "hash1",
			PerceptualHash: "phash1",
		},
		{
			Path:           "/test/image2.jpg",
			SourcePrefix:   "camera2",
			Format:         "jpg",
			Width:          1920,
			Height:         1080,
			ModifiedAt:     time.Now().Format(time.RFC3339),
			Size:           1024000,
			AverageHash:    "hash2",
			PerceptualHash: "phash2",
		},
	}

	for _, img := range images {
		database.StoreImageInfo(db, img, false)
	}

	// Query all matches
	rows, err := database.QueryPotentialMatches(db, nil)
	if err != nil {
		t.Fatalf("Failed to query all matches: %v", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		count++
	}
	if count != 2 {
		t.Errorf("Expected 2 matches without filter, got %d", count)
	}

	// Query with source prefix filter
	rows, err = database.QueryPotentialMatches(db, []string{"camera1"})
	if err != nil {
		t.Fatalf("Failed to query filtered matches: %v", err)
	}
	defer rows.Close()

	count = 0
	for rows.Next() {
		var path, prefix, avgHash, percHash string
		rows.Scan(&path, &prefix, &avgHash, &percHash)
		if prefix != "camera1" {
			t.Errorf("Got wrong source prefix: %s", prefix)
		}
		count++
	}
	if count != 1 {
		t.Errorf("Expected 1 match with filter, got %d", count)
	}
}

func TestGetScanStats(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Get stats for empty database
	stats, err := database.GetScanStats(db, nil)
	if err != nil {
		t.Fatalf("Failed to get scan stats: %v", err)
	}
	if stats.TotalImages != 0 {
		t.Errorf("Expected 0 images in empty database, got %d", stats.TotalImages)
	}

	// Insert some images
	for i := 0; i < 5; i++ {
		imageInfo := types.ImageInfo{
			Path:           filepath.Join("/test", fmt.Sprintf("image%d.jpg", i)),
			SourcePrefix:   "test",
			Format:         "jpg",
			Width:          1920,
			Height:         1080,
			ModifiedAt:     time.Now().Format(time.RFC3339),
			Size:           1024000,
			AverageHash:    fmt.Sprintf("hash%d", i%3), // Create some duplicates
			PerceptualHash: fmt.Sprintf("phash%d", i),
		}
		database.StoreImageInfo(db, imageInfo, false)
	}

	// Get stats
	stats, err = database.GetScanStats(db, []string{"test"})
	if err != nil {
		t.Fatalf("Failed to get scan stats: %v", err)
	}
	if stats.TotalImages != 5 {
		t.Errorf("Expected 5 total images, got %d", stats.TotalImages)
	}
	if stats.UniqueHashes != 3 {
		t.Errorf("Expected 3 unique hashes, got %d", stats.UniqueHashes)
	}
}

func TestDatabaseIndexes(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Check indexes exist
	indexes := []string{"idx_path", "idx_average_hash", "idx_perceptual_hash"}
	
	for _, indexName := range indexes {
		var name string
		err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='index' AND name=?", indexName).Scan(&name)
		if err != nil {
			t.Errorf("Index %s was not created", indexName)
		}
	}
}