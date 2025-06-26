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
	tmpl       *template.Template
	config     *Config
	configPath string
}

type ScanRequest struct {
	DatabasePath string `json:"databasePath"`
	FolderPath   string `json:"folderPath"`
	Prefix       string `json:"prefix"`
	ForceRewrite bool   `json:"forceRewrite"`
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
	// Define template functions
	funcMap := template.FuncMap{
		"mul": func(a, b float64) float64 {
			return a * b
		},
	}
	
	tmpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	configPath := GetConfigPath()
	config, err := LoadConfig(configPath)
	if err != nil {
		log.Printf("Failed to load config: %v, using defaults", err)
		config = DefaultConfig()
	}

	return &Server{
		tmpl:       tmpl,
		config:     config,
		configPath: configPath,
	}, nil
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if err := s.tmpl.ExecuteTemplate(w, "index.html", s.config); err != nil {
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
			SourcePrefix: req.Prefix,
			ForceRewrite: req.ForceRewrite,
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
	thumbnail := r.URL.Query().Get("thumbnail") == "true"
	
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
	ext := strings.ToLower(filepath.Ext(filePath))
	
	// For thumbnails or raw formats, generate a JPEG thumbnail
	if thumbnail || isRawFormat(ext) {
		// Load image and generate thumbnail
		img, err := imageprocessor.LoadImage(filePath)
		if err != nil {
			// If we can't load the image, return a placeholder
			http.Error(w, "Failed to generate thumbnail", http.StatusInternalServerError)
			return
		}
		
		// Return JPEG thumbnail
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "max-age=3600")
		
		// Write thumbnail directly to response
		if err := imageprocessor.WriteJPEGThumbnail(w, img, 200); err != nil {
			http.Error(w, "Failed to write thumbnail", http.StatusInternalServerError)
		}
		return
	}
	
	// For regular image formats, serve the file directly
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
	w.Header().Set("Cache-Control", "max-age=3600")
	http.ServeFile(w, r, filePath)
}

func isRawFormat(ext string) bool {
	rawExts := []string{".cr2", ".cr3", ".nef", ".arw", ".dng", ".orf", ".rw2", ".raf", ".srw"}
	for _, rawExt := range rawExts {
		if ext == rawExt {
			return true
		}
	}
	return false
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

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(s.config)
		
	case http.MethodPost:
		var newConfig Config
		if err := json.NewDecoder(r.Body).Decode(&newConfig); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		
		// Update server config
		s.config = &newConfig
		
		// Save to file
		if err := SaveConfig(s.configPath, s.config); err != nil {
			log.Printf("Failed to save config: %v", err)
		}
		
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "saved"})
		
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleDatabaseInfo(w http.ResponseWriter, r *http.Request) {
	dbPath := r.URL.Query().Get("path")
	if dbPath == "" {
		http.Error(w, "Missing database path", http.StatusBadRequest)
		return
	}
	
	dbPath = expandPath(dbPath)
	
	// Check if database exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"exists": false,
			"count":  0,
		})
		return
	}
	
	// Open database
	db, err := database.OpenDatabase(dbPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to open database: %v", err), http.StatusInternalServerError)
		return
	}
	defer db.Close()
	
	// Get record count
	var count int
	err = db.QueryRow("SELECT COUNT(*) FROM images").Scan(&count)
	if err != nil {
		count = 0
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"exists": true,
		"count":  count,
	})
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	browseType := r.URL.Query().Get("type") // "file" or "folder"
	
	if path == "" {
		// Start from home directory if no path provided
		homeDir, _ := os.UserHomeDir()
		path = homeDir
	}
	
	path = expandPath(path)
	
	// Ensure the path exists
	info, err := os.Stat(path)
	if err != nil {
		// If path doesn't exist, try parent directory
		path = filepath.Dir(path)
		info, err = os.Stat(path)
		if err != nil {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
	}
	
	// If it's a file and we're browsing for folders, use parent directory
	if !info.IsDir() && browseType == "folder" {
		path = filepath.Dir(path)
	}
	
	// List directory contents
	entries, err := os.ReadDir(path)
	if err != nil {
		http.Error(w, "Failed to read directory", http.StatusInternalServerError)
		return
	}
	
	type FileEntry struct {
		Name     string `json:"name"`
		Path     string `json:"path"`
		IsDir    bool   `json:"isDir"`
		Size     int64  `json:"size"`
		Modified string `json:"modified"`
	}
	
	var result struct {
		CurrentPath string       `json:"currentPath"`
		ParentPath  string       `json:"parentPath"`
		Entries     []FileEntry  `json:"entries"`
	}
	
	result.CurrentPath = path
	result.ParentPath = filepath.Dir(path)
	result.Entries = make([]FileEntry, 0)
	
	// Add entries
	for _, entry := range entries {
		// Skip hidden files/folders (starting with .)
		if strings.HasPrefix(entry.Name(), ".") && entry.Name() != ".." {
			continue
		}
		
		info, err := entry.Info()
		if err != nil {
			continue
		}
		
		// For file browsing, only show directories and .db files
		if browseType == "file" && !entry.IsDir() {
			if !strings.HasSuffix(strings.ToLower(entry.Name()), ".db") {
				continue
			}
		}
		
		fe := FileEntry{
			Name:     entry.Name(),
			Path:     filepath.Join(path, entry.Name()),
			IsDir:    entry.IsDir(),
			Size:     info.Size(),
			Modified: info.ModTime().Format("2006-01-02 15:04"),
		}
		
		result.Entries = append(result.Entries, fe)
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
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
	http.HandleFunc("/api/config", server.handleConfig)
	http.HandleFunc("/api/database-info", server.handleDatabaseInfo)
	http.HandleFunc("/api/browse", server.handleBrowse)
	
	// Static files
	http.Handle("/static/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf(":%d", port)
	log.Printf("Starting GoImageFinder web server on http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}