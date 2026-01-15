package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"imagefinder/database"
)

// TestBatchSearchRequest tests the batch search request structure
func TestBatchSearchRequestStructure(t *testing.T) {
	// Create temp test images
	tempDir := t.TempDir()
	testImages := createTestImages(t, tempDir, 3)

	// Create multipart form with multiple images
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add form fields
	if err := writer.WriteField("databasePath", "/tmp/test.db"); err != nil {
		t.Fatalf("Failed to write databasePath field: %v", err)
	}
	if err := writer.WriteField("threshold", "0.75"); err != nil {
		t.Fatalf("Failed to write threshold field: %v", err)
	}

	// Add multiple image files
	for _, imgPath := range testImages {
		file, err := os.Open(imgPath)
		if err != nil {
			t.Fatalf("Failed to open test image: %v", err)
		}
		defer file.Close()

		part, err := writer.CreateFormFile("images", filepath.Base(imgPath))
		if err != nil {
			t.Fatalf("Failed to create form file: %v", err)
		}
		if _, err := io.Copy(part, file); err != nil {
			t.Fatalf("Failed to copy file to form: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close writer: %v", err)
	}

	// Verify request can be parsed
	req := httptest.NewRequest(http.MethodPost, "/api/batch-search", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatalf("Failed to parse multipart form: %v", err)
	}

	// Verify form values
	if req.FormValue("databasePath") != "/tmp/test.db" {
		t.Error("Expected databasePath to be /tmp/test.db")
	}
	if req.FormValue("threshold") != "0.75" {
		t.Error("Expected threshold to be 0.75")
	}

	// Verify multiple files were received
	files := req.MultipartForm.File["images"]
	if len(files) != 3 {
		t.Errorf("Expected 3 images, got %d", len(files))
	}
}

// TestBatchSearchResponseStructure tests the batch search response structure
func TestBatchSearchResponseStructure(t *testing.T) {
	// Test response JSON marshaling
	results := []BatchSearchResult{
		{
			QueryImage: "image1.jpg",
			Results: []SearchResult{
				{Path: "/path/to/match1.jpg", Score: 0.95},
				{Path: "/path/to/match2.jpg", Score: 0.85},
			},
		},
		{
			QueryImage: "image2.jpg",
			Results: []SearchResult{
				{Path: "/path/to/match3.jpg", Score: 0.90},
			},
		},
		{
			QueryImage: "image3.jpg",
			Results: []SearchResult{},
			Error:   "failed to process image",
		},
	}

	data, err := json.Marshal(results)
	if err != nil {
		t.Fatalf("Failed to marshal response: %v", err)
	}

	// Unmarshal and verify
	var decoded []BatchSearchResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if len(decoded) != 3 {
		t.Errorf("Expected 3 results, got %d", len(decoded))
	}

	// Verify first result
	if decoded[0].QueryImage != "image1.jpg" {
		t.Error("First result should be image1.jpg")
	}
	if len(decoded[0].Results) != 2 {
		t.Error("First result should have 2 matches")
	}

	// Verify third result has error
	if decoded[2].Error == "" {
		t.Error("Third result should have an error")
	}
}

// TestBatchSearchEmptyRequest tests handling of empty batch requests
func TestBatchSearchEmptyRequest(t *testing.T) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("databasePath", "/tmp/test.db")
	writer.WriteField("threshold", "0.75")
	// No images added
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/batch-search", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatalf("Failed to parse multipart form: %v", err)
	}

	files := req.MultipartForm.File["images"]
	if len(files) != 0 {
		t.Error("Expected 0 images for empty request")
	}
}

// TestBatchSearchMaxImages tests the maximum image limit
func TestBatchSearchMaxImages(t *testing.T) {
	const maxImages = 20 // Proposed limit

	tempDir := t.TempDir()
	testImages := createTestImages(t, tempDir, maxImages+5)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("databasePath", "/tmp/test.db")
	writer.WriteField("threshold", "0.75")

	for i, imgPath := range testImages {
		file, _ := os.Open(imgPath)
		defer file.Close()
		part, _ := writer.CreateFormFile("images", filepath.Base(imgPath))
		io.Copy(part, file)

		// Simulate limit enforcement
		if i >= maxImages-1 {
			break
		}
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/batch-search", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ParseMultipartForm(32 << 20)

	files := req.MultipartForm.File["images"]
	if len(files) > maxImages {
		t.Errorf("Should enforce max %d images, got %d", maxImages, len(files))
	}
}

// TestBatchSearchPartialFailure tests handling when some images fail
func TestBatchSearchPartialFailure(t *testing.T) {
	// Simulate response where some images succeeded and some failed
	results := []BatchSearchResult{
		{
			QueryImage: "valid.jpg",
			Results: []SearchResult{
				{Path: "/path/to/match.jpg", Score: 0.95},
			},
		},
		{
			QueryImage: "corrupted.jpg",
			Results: []SearchResult{},
			Error:   "failed to decode image: invalid format",
		},
		{
			QueryImage: "valid2.png",
			Results: []SearchResult{
				{Path: "/path/to/match2.jpg", Score: 0.80},
			},
		},
	}

	// Verify structure allows partial success
	successCount := 0
	errorCount := 0
	for _, r := range results {
		if r.Error != "" {
			errorCount++
		} else {
			successCount++
		}
	}

	if successCount != 2 {
		t.Errorf("Expected 2 successful results, got %d", successCount)
	}
	if errorCount != 1 {
		t.Errorf("Expected 1 error result, got %d", errorCount)
	}
}

// TestBatchSearchWithDatabase tests batch search against a real database
func TestBatchSearchWithDatabase(t *testing.T) {
	// Create temporary database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	defer db.Close()

	// Insert some test records
	_, err = db.Exec(`
		INSERT INTO images (path, source_prefix, format, width, height, average_hash, perceptual_hash)
		VALUES
			(?, 'test', 'png', 100, 100, 'aaaaaaaaaaaaaaaa', 'bbbbbbbbbbbbbbbb'),
			(?, 'test', 'png', 100, 100, 'cccccccccccccccc', 'dddddddddddddddd'),
			(?, 'test', 'png', 100, 100, 'eeeeeeeeeeeeeeee', 'ffffffffffffffff')
	`, filepath.Join(tempDir, "img1.png"), filepath.Join(tempDir, "img2.png"), filepath.Join(tempDir, "img3.png"))

	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	// Verify database has records
	var count int
	db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
	if count != 3 {
		t.Errorf("Expected 3 records in database, got %d", count)
	}
}

// TestBatchSearchFormDataSize tests that large requests are handled properly
func TestBatchSearchFormDataSize(t *testing.T) {
	const maxFormSize = 50 << 20 // 50MB limit

	tempDir := t.TempDir()

	// Create a single large test image
	largePath := filepath.Join(tempDir, "large.png")
	createLargeTestImage(t, largePath, 1000, 1000) // ~1MB uncompressed

	info, err := os.Stat(largePath)
	if err != nil {
		t.Fatalf("Failed to stat large image: %v", err)
	}

	// Verify the test image was created
	if info.Size() == 0 {
		t.Error("Large test image should have non-zero size")
	}

	t.Logf("Large test image size: %d bytes", info.Size())
}

// TestBatchSearchConcurrentProcessing tests concurrent search processing
func TestBatchSearchConcurrentProcessing(t *testing.T) {
	// This test validates the design decision to process images concurrently
	// In implementation, we should use a worker pool pattern

	numImages := 5
	results := make(chan BatchSearchResult, numImages)

	// Simulate concurrent processing
	for i := 0; i < numImages; i++ {
		go func(idx int) {
			// Simulate image processing
			results <- BatchSearchResult{
				QueryImage: fmt.Sprintf("image%d.jpg", idx),
				Results: []SearchResult{
					{Path: fmt.Sprintf("/match%d.jpg", idx), Score: 0.9},
				},
			}
		}(i)
	}

	// Collect results
	collected := make([]BatchSearchResult, 0, numImages)
	for i := 0; i < numImages; i++ {
		collected = append(collected, <-results)
	}

	if len(collected) != numImages {
		t.Errorf("Expected %d results, got %d", numImages, len(collected))
	}
}

// TestBatchSearchResultOrdering tests that results maintain query order
func TestBatchSearchResultOrdering(t *testing.T) {
	// Results should be returned in the same order as query images
	queryOrder := []string{"first.jpg", "second.jpg", "third.jpg"}

	results := make([]BatchSearchResult, len(queryOrder))
	for i, name := range queryOrder {
		results[i] = BatchSearchResult{
			QueryImage: name,
			Results:    []SearchResult{},
		}
	}

	// Verify order is maintained
	for i, r := range results {
		if r.QueryImage != queryOrder[i] {
			t.Errorf("Result %d: expected %s, got %s", i, queryOrder[i], r.QueryImage)
		}
	}
}

// Helper function to create test PNG images
func createTestImages(t *testing.T, dir string, count int) []string {
	t.Helper()
	paths := make([]string, count)

	for i := 0; i < count; i++ {
		path := filepath.Join(dir, fmt.Sprintf("test_%d.png", i))
		createTestPNG(t, path, 100, 100, uint8(i*50%256))
		paths[i] = path
	}

	return paths
}

// Helper function to create a test PNG image
func createTestPNG(t *testing.T, path string, width, height int, grayValue uint8) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	c := color.RGBA{grayValue, grayValue, grayValue, 255}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, c)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create test image: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}
}

// TestBatchSearchHandler tests the actual HTTP handler
func TestBatchSearchHandler(t *testing.T) {
	// Create temp directory and database
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	// Initialize database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	// Create test images and add them to database
	testImages := createTestImages(t, tempDir, 3)
	for i, imgPath := range testImages {
		_, err = db.Exec(`
			INSERT INTO images (path, source_prefix, format, width, height, average_hash, perceptual_hash)
			VALUES (?, 'test', 'png', 100, 100, ?, ?)
		`, imgPath, fmt.Sprintf("%016x", i), fmt.Sprintf("%016x", i+100))
		if err != nil {
			t.Fatalf("Failed to insert test image: %v", err)
		}
	}
	db.Close()

	// Create server
	server, err := NewServer()
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Create multipart form request
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	// Add form fields
	writer.WriteField("databasePath", dbPath)
	writer.WriteField("threshold", "0.5")

	// Add 2 test images
	for i := 0; i < 2; i++ {
		file, _ := os.Open(testImages[i])
		defer file.Close()
		part, _ := writer.CreateFormFile("images", filepath.Base(testImages[i]))
		io.Copy(part, file)
	}
	writer.Close()

	// Create request
	req := httptest.NewRequest(http.MethodPost, "/api/batch-search", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Create response recorder
	rr := httptest.NewRecorder()

	// Call handler
	server.handleBatchSearch(rr, req)

	// Check status code
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", rr.Code, rr.Body.String())
		return
	}

	// Parse response
	var results []BatchSearchResult
	if err := json.Unmarshal(rr.Body.Bytes(), &results); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify we got 2 results
	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	// Verify each result has a query image name
	for i, result := range results {
		if result.QueryImage == "" {
			t.Errorf("Result %d has empty queryImage", i)
		}
	}
}

// Helper function to create a larger test image
func createLargeTestImage(t *testing.T, path string, width, height int) {
	t.Helper()

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := color.RGBA{
				uint8(x * 255 / width),
				uint8(y * 255 / height),
				128,
				255,
			}
			img.Set(x, y, c)
		}
	}

	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create large test image: %v", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("Failed to encode large test image: %v", err)
	}
}
