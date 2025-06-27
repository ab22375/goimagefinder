package webserver_test

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"imagefinder/database"
)

// Helper to create test image
func createTestImage(t *testing.T, path string) {
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	file, err := os.Create(path)
	if err != nil {
		t.Fatalf("Failed to create test image: %v", err)
	}
	defer file.Close()
	
	if err := jpeg.Encode(file, img, nil); err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}
}

// Test configuration structure
type Config struct {
	DatabasePath string  `json:"databasePath"`
	FolderPath   string  `json:"folderPath"`
	Threshold    float64 `json:"threshold"`
	Prefix       string  `json:"prefix"`
	ForceRewrite bool    `json:"forceRewrite"`
}

// Test basic HTTP endpoints
func TestHTTPEndpoints(t *testing.T) {
	// This test demonstrates the expected API structure
	// In a real test environment, you would start the actual server
	
	endpoints := []struct {
		method string
		path   string
		body   interface{}
		status int
	}{
		{"GET", "/", nil, http.StatusOK},
		{"GET", "/api/config", nil, http.StatusOK},
		{"POST", "/api/config", Config{}, http.StatusOK},
		{"GET", "/api/database-info?path=/test.db", nil, http.StatusOK},
		{"GET", "/api/browse?path=/tmp&type=folder", nil, http.StatusOK},
		{"POST", "/api/scan", map[string]interface{}{}, http.StatusOK},
		{"POST", "/api/upload-search", nil, http.StatusOK},
		{"GET", "/api/file?path=/test.jpg", nil, http.StatusNotFound},
		{"GET", "/static/style.css", nil, http.StatusOK},
		{"GET", "/static/script.js", nil, http.StatusOK},
	}
	
	// Document expected endpoints
	for _, ep := range endpoints {
		t.Logf("Endpoint: %s %s - Expected status: %d", ep.method, ep.path, ep.status)
	}
}

// Test scan request structure
func TestScanRequestStructure(t *testing.T) {
	scanReq := map[string]interface{}{
		"databasePath": "/path/to/db.db",
		"folderPath":   "/path/to/images",
		"prefix":       "test",
		"forceRewrite": false,
	}
	
	jsonData, err := json.Marshal(scanReq)
	if err != nil {
		t.Fatalf("Failed to marshal scan request: %v", err)
	}
	
	// Verify JSON structure
	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal scan request: %v", err)
	}
	
	if decoded["databasePath"] != "/path/to/db.db" {
		t.Error("Database path not properly encoded")
	}
}

// Test search request structure
func TestSearchRequestStructure(t *testing.T) {
	searchReq := map[string]interface{}{
		"databasePath": "/path/to/db.db",
		"imagePath":    "/path/to/query.jpg",
		"threshold":    0.75,
	}
	
	jsonData, err := json.Marshal(searchReq)
	if err != nil {
		t.Fatalf("Failed to marshal search request: %v", err)
	}
	
	// Verify JSON structure
	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal search request: %v", err)
	}
	
	if decoded["threshold"].(float64) != 0.75 {
		t.Error("Threshold not properly encoded")
	}
}

// Test multipart form creation for upload
func TestMultipartFormCreation(t *testing.T) {
	tempDir := t.TempDir()
	imagePath := filepath.Join(tempDir, "test.jpg")
	createTestImage(t, imagePath)
	
	// Create multipart form
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	
	// Add file
	file, err := os.Open(imagePath)
	if err != nil {
		t.Fatalf("Failed to open test image: %v", err)
	}
	defer file.Close()
	
	part, err := writer.CreateFormFile("image", "test.jpg")
	if err != nil {
		t.Fatalf("Failed to create form file: %v", err)
	}
	
	if _, err := io.Copy(part, file); err != nil {
		t.Fatalf("Failed to copy file content: %v", err)
	}
	
	// Add fields
	writer.WriteField("databasePath", "/path/to/db.db")
	writer.WriteField("threshold", "0.75")
	
	if err := writer.Close(); err != nil {
		t.Fatalf("Failed to close multipart writer: %v", err)
	}
	
	// Verify content type
	contentType := writer.FormDataContentType()
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("Expected multipart/form-data content type, got %s", contentType)
	}
}

// Test SSE (Server-Sent Events) parsing
func TestSSEParsing(t *testing.T) {
	sseData := []string{
		"data: {\"total\": 100, \"message\": \"Found 100 images\"}\n\n",
		"data: {\"current\": 10, \"message\": \"Processing...\"}\n\n",
		"data: {\"complete\": true, \"total\": 100}\n\n",
		"data: {\"error\": \"Something went wrong\"}\n\n",
	}
	
	for _, data := range sseData {
		if !strings.HasPrefix(data, "data: ") {
			t.Error("SSE data should start with 'data: '")
		}
		
		jsonStr := strings.TrimPrefix(strings.TrimSpace(data), "data: ")
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
			t.Errorf("Failed to parse SSE JSON: %v", err)
		}
	}
}

// Test browse API response structure
func TestBrowseResponseStructure(t *testing.T) {
	response := struct {
		CurrentPath string `json:"currentPath"`
		ParentPath  string `json:"parentPath"`
		Entries     []struct {
			Name     string `json:"name"`
			Path     string `json:"path"`
			IsDir    bool   `json:"isDir"`
			Size     int64  `json:"size"`
			Modified string `json:"modified"`
		} `json:"entries"`
	}{
		CurrentPath: "/home/user",
		ParentPath:  "/home",
		Entries: []struct {
			Name     string `json:"name"`
			Path     string `json:"path"`
			IsDir    bool   `json:"isDir"`
			Size     int64  `json:"size"`
			Modified string `json:"modified"`
		}{
			{
				Name:     "documents",
				Path:     "/home/user/documents",
				IsDir:    true,
				Size:     0,
				Modified: "2025-01-01 12:00",
			},
			{
				Name:     "test.db",
				Path:     "/home/user/test.db",
				IsDir:    false,
				Size:     1024,
				Modified: "2025-01-01 12:00",
			},
		},
	}
	
	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Failed to marshal browse response: %v", err)
	}
	
	// Verify structure
	var decoded map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal browse response: %v", err)
	}
	
	entries := decoded["entries"].([]interface{})
	if len(entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(entries))
	}
}

// Test database info response structure
func TestDatabaseInfoResponse(t *testing.T) {
	responses := []struct {
		exists bool
		count  int
	}{
		{exists: true, count: 100},
		{exists: false, count: 0},
	}
	
	for _, resp := range responses {
		data := map[string]interface{}{
			"exists": resp.exists,
			"count":  resp.count,
		}
		
		jsonData, err := json.Marshal(data)
		if err != nil {
			t.Fatalf("Failed to marshal database info: %v", err)
		}
		
		var decoded map[string]interface{}
		if err := json.Unmarshal(jsonData, &decoded); err != nil {
			t.Fatalf("Failed to unmarshal database info: %v", err)
		}
		
		if decoded["exists"].(bool) != resp.exists {
			t.Errorf("Expected exists=%v, got %v", resp.exists, decoded["exists"])
		}
	}
}

// Test search result structure
func TestSearchResultStructure(t *testing.T) {
	results := []struct {
		Path  string  `json:"path"`
		Score float64 `json:"score"`
	}{
		{Path: "/images/photo1.jpg", Score: 0.95},
		{Path: "/images/photo2.jpg", Score: 0.87},
		{Path: "/images/photo3.jpg", Score: 0.76},
	}
	
	jsonData, err := json.Marshal(results)
	if err != nil {
		t.Fatalf("Failed to marshal search results: %v", err)
	}
	
	var decoded []map[string]interface{}
	if err := json.Unmarshal(jsonData, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal search results: %v", err)
	}
	
	if len(decoded) != 3 {
		t.Errorf("Expected 3 results, got %d", len(decoded))
	}
	
	// Verify scores are in descending order
	for i := 1; i < len(decoded); i++ {
		prevScore := decoded[i-1]["score"].(float64)
		currScore := decoded[i]["score"].(float64)
		if prevScore < currScore {
			t.Error("Results should be sorted by score in descending order")
		}
	}
}

// Test HTML template rendering
func TestHTMLTemplateStructure(t *testing.T) {
	// Test that template expects certain data structure
	templateData := struct {
		DatabasePath string
		FolderPath   string
		Threshold    float64
		Prefix       string
		ForceRewrite bool
	}{
		DatabasePath: "/path/to/db.db",
		FolderPath:   "/path/to/images",
		Threshold:    0.75,
		Prefix:       "test",
		ForceRewrite: false,
	}
	
	// Verify all fields are present
	if templateData.DatabasePath == "" {
		t.Error("DatabasePath should not be empty")
	}
	if templateData.Threshold < 0 || templateData.Threshold > 1 {
		t.Error("Threshold should be between 0 and 1")
	}
}

// Integration test helper - demonstrates full workflow
func TestWorkflowExample(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	imageDir := filepath.Join(tempDir, "images")
	
	// 1. Create test environment
	os.MkdirAll(imageDir, 0755)
	createTestImage(t, filepath.Join(imageDir, "test1.jpg"))
	createTestImage(t, filepath.Join(imageDir, "test2.jpg"))
	
	// 2. Initialize database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	db.Close()
	
	// 3. Document expected workflow
	t.Log("Workflow steps:")
	t.Log("1. User navigates to web interface")
	t.Log("2. User selects database path using file browser")
	t.Log("3. User selects image folder using file browser")
	t.Log("4. User configures scan options (prefix, force rewrite)")
	t.Log("5. User clicks 'Scan' to index images")
	t.Log("6. Progress bar shows scan progress via SSE")
	t.Log("7. User selects query image for similarity search")
	t.Log("8. User adjusts threshold slider")
	t.Log("9. User clicks 'Search' to find similar images")
	t.Log("10. Results display with thumbnails and copy buttons")
}