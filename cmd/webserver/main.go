package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"imagefinder/database"
	"imagefinder/imageprocessor"
	"imagefinder/scanner"
)

//go:embed templates/*
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

type Server struct {
	tmpl *template.Template
}

type ScanRequest struct {
	DatabasePath string `json:"databasePath"`
	FolderPath   string `json:"folderPath"`
}

type SearchRequest struct {
	DatabasePath string  `json:"databasePath"`
	ImagePath    string  `json:"imagePath"`
	Threshold    float64 `json:"threshold"`
}

type SearchResult struct {
	Path      string  `json:"path"`
	Score     float64 `json:"score"`
	Thumbnail string  `json:"thumbnail"`
}

type ScanProgress struct {
	Current int    `json:"current"`
	Total   int    `json:"total"`
	Message string `json:"message"`
}

func NewServer() (*Server, error) {
	tmpl, err := template.ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return &Server{
		tmpl: tmpl,
	}, nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if err := s.tmpl.ExecuteTemplate(w, "index.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Expand paths
	dbPath := expandPath(req.DatabasePath)
	folderPath := expandPath(req.FolderPath)

	log.Printf("Starting scan - DB: %s, Folder: %s", dbPath, folderPath)

	// Open or create database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		// Try opening existing database
		db, err = database.OpenDatabase(dbPath)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
			return
		}
	}
	defer db.Close()

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	// Start scanning in background
	doneCh := make(chan error, 1)
	progressCh := make(chan string, 100)

	go func() {
		options := scanner.ScanOptions{
			FolderPath:   folderPath,
			SourcePrefix: "",
			ForceRewrite: false,
			MaxWorkers:   8,
			DebugMode:    true,
		}
		
		// Count files first
		fileCount := 0
		filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				ext := strings.ToLower(filepath.Ext(path))
				if isImageFile(ext) {
					fileCount++
				}
			}
			return nil
		})
		
		progressCh <- fmt.Sprintf("{\"total\": %d, \"message\": \"Found %d images\"}", fileCount, fileCount)
		
		err := scanner.ScanAndStoreFolder(db, options)
		if err != nil {
			log.Printf("Scan error: %v", err)
		}
		doneCh <- err
	}()

	// Send periodic progress updates
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	processedCount := 0

	for {
		select {
		case msg := <-progressCh:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
			
		case <-ticker.C:
			// Query database for current count
			var count int
			db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
			if count > processedCount {
				processedCount = count
				fmt.Fprintf(w, "data: {\"current\": %d, \"message\": \"Processing...\"}\n\n", processedCount)
				flusher.Flush()
			}

		case err := <-doneCh:
			if err != nil {
				log.Printf("Scan completed with error: %v", err)
				fmt.Fprintf(w, "data: {\"error\": \"%s\"}\n\n", err.Error())
			} else {
				// Get final stats
				var finalCount int
				db.QueryRow("SELECT COUNT(*) FROM images").Scan(&finalCount)
				log.Printf("Scan completed successfully. Total images: %d", finalCount)
				fmt.Fprintf(w, "data: {\"complete\": true, \"total\": %d}\n\n", finalCount)
			}
			flusher.Flush()
			return
		}
	}
}

func isImageFile(ext string) bool {
	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif", ".webp", 
		".cr2", ".cr3", ".nef", ".arw", ".dng", ".orf", ".rw2", ".raf", ".srw"}
	for _, imgExt := range imageExts {
		if ext == imgExt {
			return true
		}
	}
	return false
}

func (s *Server) handleUploadAndSearch(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form
	err := r.ParseMultipartForm(10 << 20) // 10MB max
	if err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// Get form values
	dbPath := expandPath(r.FormValue("databasePath"))
	threshold, _ := strconv.ParseFloat(r.FormValue("threshold"), 64)
	if threshold == 0 {
		threshold = 0.75
	}

	// Get uploaded file
	file, header, err := r.FormFile("image")
	if err != nil {
		http.Error(w, "Failed to get uploaded file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create temp file
	tempFile, err := os.CreateTemp("", "search-*."+filepath.Ext(header.Filename))
	if err != nil {
		http.Error(w, "Failed to create temp file", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Copy uploaded file to temp file
	_, err = io.Copy(tempFile, file)
	if err != nil {
		http.Error(w, "Failed to save uploaded file", http.StatusInternalServerError)
		return
	}

	// Open database
	db, err := database.OpenDatabase(dbPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Search for similar images
	searchOptions := imageprocessor.SearchOptions{
		QueryPath:    tempFile.Name(),
		Threshold:    threshold,
		DebugMode:    false,
		SourcePrefix: "",
	}
	
	matches, err := imageprocessor.FindSimilarImages(db, searchOptions)
	if err != nil {
		http.Error(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	results := make([]SearchResult, len(matches))
	for i, match := range matches {
		results[i] = SearchResult{
			Path:  match.Path,
			Score: match.SSIMScore,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Expand paths
	dbPath := expandPath(req.DatabasePath)
	imagePath := expandPath(req.ImagePath)

	// Open database
	db, err := database.OpenDatabase(dbPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
		return
	}
	defer db.Close()

	// Search for similar images
	searchOptions := imageprocessor.SearchOptions{
		QueryPath:    imagePath,
		Threshold:    req.Threshold,
		DebugMode:    false,
		SourcePrefix: "",
	}
	
	matches, err := imageprocessor.FindSimilarImages(db, searchOptions)
	if err != nil {
		http.Error(w, fmt.Sprintf("Search failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert to response format
	results := make([]SearchResult, len(matches))
	for i, match := range matches {
		results[i] = SearchResult{
			Path:  match.Path,
			Score: match.SSIMScore,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleFile(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "Missing path parameter", http.StatusBadRequest)
		return
	}

	// Security check - ensure path exists and is an image
	if _, err := os.Stat(filePath); err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	// Check if it's an image
	ext := filepath.Ext(filePath)
	contentType := ""
	switch ext {
	case ".jpg", ".jpeg":
		contentType = "image/jpeg"
	case ".png":
		contentType = "image/png"
	case ".gif":
		contentType = "image/gif"
	case ".webp":
		contentType = "image/webp"
	default:
		http.Error(w, "Not an image file", http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", contentType)
	http.ServeFile(w, r, filePath)
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}

func main() {
	port := 8012
	if len(os.Args) > 1 {
		if p, err := strconv.Atoi(os.Args[1]); err == nil {
			port = p
		}
	}

	server, err := NewServer()
	if err != nil {
		log.Fatal(err)
	}

	// Routes
	http.HandleFunc("/", server.handleIndex)
	http.HandleFunc("/api/scan", server.handleScan)
	http.HandleFunc("/api/search", server.handleSearch)
	http.HandleFunc("/api/upload-search", server.handleUploadAndSearch)
	http.HandleFunc("/api/file", server.handleFile)
	
	// Static files
	http.Handle("/static/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting GoImageFinder web server on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}