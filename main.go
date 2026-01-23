package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"imagefinder/database"
	"imagefinder/imageprocessor"
	"imagefinder/logging"
	"imagefinder/output"
	"imagefinder/scanner"
	"imagefinder/signalhandler"
	"imagefinder/utils"
)

func main() {
	// Set up graceful shutdown with context
	ctx, cancel := signalhandler.SetupWithContext()
	defer cancel()

	// Set the optimal number of CPUs to use
	runtime.GOMAXPROCS(signalhandler.GetOptimalProcs())

	// Parse command line arguments into a map
	args := utils.ParseArguments()

	// Get the command (scan or search)
	command, hasCommand := args["command"]

	// Set default database path
	dbPath := utils.GetDefaultDatabasePath()
	if customDB, ok := args["database"]; ok && customDB != "" {
		dbPath = customDB
	} else if customDB, ok := args["db"]; ok && customDB != "" {
		// Allow --db as an alias for --database
		dbPath = customDB
	}

	// Setup debug logging if enabled
	debugMode := false
	if _, ok := args["debug"]; ok {
		debugMode = true
		logPath := "imagefinder.log"
		if customLogPath, ok := args["logfile"]; ok && customLogPath != "" {
			logPath = customLogPath
		}
		if err := logging.SetupLogger(logPath); err != nil {
			fmt.Printf("Warning: Failed to setup logging: %v\n", err)
		} else {
			fmt.Printf("Debug mode enabled. Logging to: %s\n", logPath)
		}
	}

	// Check JSON output mode
	jsonMode := false
	if _, ok := args["json"]; ok {
		jsonMode = true
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
		if jsonMode {
			output.PrintError("Missing required arguments", 1)
		} else {
			utils.PrintUsage()
		}
		os.Exit(1)
	}

	switch command {
	case "scan":
		handleScanCommand(ctx, args, dbPath, debugMode, jsonMode)
	case "search":
		handleSearchCommand(ctx, args, dbPath, debugMode, jsonMode)
	case "info":
		handleInfoCommand(args, dbPath, jsonMode)
	default:
		if jsonMode {
			output.PrintError(fmt.Sprintf("Unknown command: %s", command), 1)
		} else {
			fmt.Printf("Unknown command: %s\n", command)
			utils.PrintUsage()
		}
		os.Exit(1)
	}
}

func handleScanCommand(ctx context.Context, args map[string]string, dbPath string, debugMode bool, jsonMode bool) {
	// Set optimal GOMAXPROCS
	runtime.GOMAXPROCS(signalhandler.GetOptimalProcs())

	// Check for progress streaming mode
	progressMode := false
	if _, ok := args["progress"]; ok {
		progressMode = true
	}

	// Get folder path with validation
	folderPath, hasFolder := args["folder"]
	if !hasFolder {
		if jsonMode {
			output.PrintError("Missing folder path (use --folder=PATH)", 1)
		} else {
			fmt.Println("Error: Missing folder path (use --folder=PATH)")
		}
		os.Exit(1)
	}

	// Verify folder path exists and is accessible
	folderInfo, err := os.Stat(folderPath)
	if err != nil {
		errMsg := fmt.Sprintf("Folder path does not exist: %s", folderPath)
		if !os.IsNotExist(err) {
			errMsg = fmt.Sprintf("Cannot access folder path: %s (%v)", folderPath, err)
		}
		if jsonMode {
			output.PrintError(errMsg, 1)
			os.Exit(1)
		}
		log.Fatalf("%s", errMsg)
	}
	if !folderInfo.IsDir() {
		errMsg := fmt.Sprintf("Path is not a directory: %s", folderPath)
		if jsonMode {
			output.PrintError(errMsg, 1)
			os.Exit(1)
		}
		log.Fatalf("%s", errMsg)
	}

	// Get source prefix (with empty default)
	sourcePrefix := ""
	if prefix, ok := args["prefix"]; ok {
		sourcePrefix = prefix
	}

	// Get force rewrite flag
	forceRewrite := false
	if _, ok := args["force"]; ok {
		forceRewrite = true
	}

	// Get log file path if provided
	logPath := ""
	if path, ok := args["logfile"]; ok {
		logPath = path
		// Set up file-based logging if logfile is specified
		logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Failed to open log file: %v", err)
		}
		defer logFile.Close()

		// Use MultiWriter to write logs to both stdout and file
		if debugMode {
			log.SetOutput(io.MultiWriter(os.Stdout, logFile))
		} else {
			log.SetOutput(logFile)
		}
		log.Printf("Debug mode enabled. Logging to: %s", logPath)
	}

	startTime := time.Now()

	// Initialize database with retry logic
	var db *sql.DB
	const maxRetries = 3
	for i := 0; i < maxRetries; i++ {
		db, err = database.InitDatabase(dbPath)
		if err == nil {
			break
		}

		if i < maxRetries-1 {
			log.Printf("Error initializing database (attempt %d/%d): %v - retrying...",
				i+1, maxRetries, err)
			time.Sleep(time.Second * time.Duration(i+1))
		} else {
			log.Fatalf("Error initializing database after %d attempts: %v", maxRetries, err)
		}
	}
	defer db.Close()

	// Count total image files for progress tracking
	var totalImages int
	var rawCount, tifCount int
	err = filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files that can't be accessed
		}
		if !info.IsDir() {
			ext := strings.ToLower(filepath.Ext(path))
			if scanner.IsImageFile(ext) {
				totalImages++
				if scanner.IsRawFormat(ext) {
					rawCount++
				} else if scanner.IsTiffFormat(ext) {
					tifCount++
				}
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Warning: Could not count all files: %v", err)
	}

	if !jsonMode {
		fmt.Printf("Starting image indexing...\n")
		fmt.Printf("Total image files to process: %d (including %d RAW files and %d TIF files)\n",
			totalImages, rawCount, tifCount)
		fmt.Printf("Force rewrite mode: %v\n", forceRewrite)
		fmt.Printf("Source prefix: %s\n", sourcePrefix)
		fmt.Printf("Debug mode: %s\n", map[bool]string{true: "enabled", false: "disabled"}[debugMode])
	} else if progressMode {
		// Output initial progress event
		output.PrintJSONLine(output.ScanProgressEvent{
			Type:      "start",
			Processed: 0,
			Total:     totalImages,
			Status:    "starting",
		})
	}

	// Create scan options with all parameters
	scanOptions := scanner.ScanOptions{
		FolderPath:   folderPath,
		SourcePrefix: sourcePrefix,
		ForceRewrite: forceRewrite,
		DebugMode:    debugMode,
		DbPath:       dbPath,
		LogPath:      logPath,
		TotalImages:  totalImages,
		MaxWorkers:   signalhandler.GetOptimalProcs(),
	}

	// Run scanner with graceful shutdown handling
	errChan := make(chan error, 1)
	doneChan := make(chan bool, 1)

	go func() {
		err := scanner.ScanAndStoreFolder(ctx, db, scanOptions)
		if err != nil {
			errChan <- err
		} else {
			doneChan <- true
		}
	}()

	// Wait for completion, error, or cancellation
	select {
	case <-ctx.Done():
		// Graceful shutdown requested
		if jsonMode {
			output.PrintError("Scan interrupted by user", 130)
		} else {
			fmt.Println("\nScan interrupted. Waiting for in-progress operations to complete...")
		}
		// Wait briefly for cleanup
		select {
		case <-errChan:
		case <-doneChan:
		case <-time.After(5 * time.Second):
		}
		os.Exit(130) // Standard exit code for SIGINT
	case err := <-errChan:
		if jsonMode {
			output.PrintError(fmt.Sprintf("Error scanning folder: %v", err), 1)
			os.Exit(1)
		}
		log.Fatalf("Error scanning folder: %v", err)
	case <-doneChan:
		// Get statistics
		duration := time.Since(startTime)
		var prefixes []string
		if sourcePrefix != "" {
			prefixes = []string{sourcePrefix}
		}
		stats, statsErr := database.GetScanStats(db, prefixes)

		if jsonMode {
			// Output JSON result
			result := output.ScanCompleteResult{
				Type:         "complete",
				Success:      true,
				Processed:    totalImages,
				Total:        totalImages,
				DatabasePath: dbPath,
				DurationSecs: duration.Seconds(),
			}
			if statsErr == nil && stats != nil {
				result.NewImages = stats.TotalImages
				result.Errors = stats.ErrorCount
			}
			output.PrintJSON(result)
		} else {
			// Print human-readable output
			fmt.Printf("\nScan completed successfully!\n")
			fmt.Printf("Total execution time: %v\n", duration)
			fmt.Printf("Database: %s\n", dbPath)

			if statsErr == nil && stats != nil {
				fmt.Printf("\nSummary:\n")
				fmt.Printf("- Total images processed: %d\n", stats.TotalImages)
				fmt.Printf("- Total errors: %d\n", stats.ErrorCount)
				fmt.Printf("- Unique image hashes: %d\n", stats.UniqueHashes)
			}
		}
	}
}
func handleSearchCommand(ctx context.Context, args map[string]string, dbPath string, debugMode bool, jsonMode bool) {
	// Check for early cancellation
	if ctx.Err() != nil {
		if jsonMode {
			output.PrintError("Operation cancelled", 130)
		}
		os.Exit(130)
	}
	// Get query image path
	queryPath, hasQuery := args["image"]
	if !hasQuery {
		if jsonMode {
			output.PrintError("Missing query image path (use --image=PATH)", 1)
		} else {
			fmt.Println("Error: Missing query image path (use --image=PATH)")
		}
		os.Exit(1)
	}

	// Set custom threshold if provided
	threshold := 0.8 // Default threshold
	if thresholdStr, ok := args["threshold"]; ok {
		parsedThreshold, err := utils.ParseThreshold(thresholdStr)
		if err != nil {
			if !jsonMode {
				fmt.Printf("Warning: %v\n", err)
			}
		} else {
			threshold = parsedThreshold
		}
	}

	// Set result limit
	limit := 50 // Default limit
	if limitStr, ok := args["limit"]; ok {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
		}
	}

	// Get source prefixes for filtering (comma-separated)
	var sourcePrefixes []string
	if prefix, ok := args["prefix"]; ok && prefix != "" {
		// Split by comma and trim whitespace
		parts := strings.Split(prefix, ",")
		for _, p := range parts {
			trimmed := strings.TrimSpace(p)
			if trimmed != "" {
				sourcePrefixes = append(sourcePrefixes, trimmed)
			}
		}
	}

	// Verify paths exist
	if _, err := os.Stat(queryPath); os.IsNotExist(err) {
		errMsg := fmt.Sprintf("Query image does not exist: %s", queryPath)
		if jsonMode {
			output.PrintError(errMsg, 1)
			os.Exit(1)
		}
		log.Fatalf("%s", errMsg)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		errMsg := fmt.Sprintf("Database does not exist: %s. Run scan command first.", dbPath)
		if jsonMode {
			output.PrintError(errMsg, 1)
			os.Exit(1)
		}
		log.Fatalf("%s", errMsg)
	}

	startTime := time.Now()

	// Open database
	db, err := database.OpenDatabase(dbPath)
	if err != nil {
		errMsg := fmt.Sprintf("Error opening database: %v", err)
		if jsonMode {
			output.PrintError(errMsg, 1)
			os.Exit(1)
		}
		log.Fatalf("%s", errMsg)
	}
	defer db.Close()

	if !jsonMode {
		fmt.Println("Searching for similar images...")
		if len(sourcePrefixes) > 0 {
			fmt.Printf("Filtering by source prefixes: %s\n", strings.Join(sourcePrefixes, ", "))
		}
	}

	// Find similar images
	searchOptions := imageprocessor.SearchOptions{
		QueryPath:      queryPath,
		Threshold:      threshold,
		SourcePrefixes: sourcePrefixes,
		DebugMode:      debugMode,
	}

	matches, err := imageprocessor.FindSimilarImages(db, searchOptions)
	if err != nil {
		errMsg := fmt.Sprintf("Error finding similar images: %v", err)
		if jsonMode {
			output.PrintError(errMsg, 1)
			os.Exit(1)
		}
		log.Fatalf("%s", errMsg)
	}

	duration := time.Since(startTime)

	// Apply limit
	if len(matches) > limit {
		matches = matches[:limit]
	}

	if jsonMode {
		// Output JSON result
		jsonMatches := make([]output.SearchMatch, len(matches))
		for i, m := range matches {
			jsonMatches[i] = output.SearchMatch{
				Path:         m.Path,
				Score:        m.SSIMScore,
				SourcePrefix: m.SourcePrefix,
			}
		}
		result := output.SearchResult{
			Success:      true,
			Query:        queryPath,
			Matches:      jsonMatches,
			Total:        len(jsonMatches),
			Threshold:    threshold,
			DurationSecs: duration.Seconds(),
		}
		output.PrintJSON(result)
	} else {
		// Print human-readable output
		fmt.Println("\nTop Matches:")

		if len(matches) == 0 {
			fmt.Println("No matches found.")
		} else {
			displayLimit := limit
			if displayLimit > len(matches) {
				displayLimit = len(matches)
			}
			for i := 0; i < displayLimit; i++ {
				fmt.Printf("%d. Image: %s\n", i+1, matches[i].Path)
				if matches[i].SourcePrefix != "" {
					fmt.Printf("   Source: %s\n", matches[i].SourcePrefix)
				}
				fmt.Printf("   SSIM Score: %.4f\n", matches[i].SSIMScore)
			}
		}

		// Print execution time
		fmt.Printf("\nTotal search time: %v\n", duration)
	}
}

func handleInfoCommand(args map[string]string, dbPath string, jsonMode bool) {
	// Check if database exists
	fileInfo, err := os.Stat(dbPath)
	if os.IsNotExist(err) {
		errMsg := fmt.Sprintf("Database does not exist: %s", dbPath)
		if jsonMode {
			output.PrintError(errMsg, 1)
		} else {
			fmt.Printf("Error: %s\n", errMsg)
		}
		os.Exit(1)
	}

	// Open database
	db, err := database.OpenDatabase(dbPath)
	if err != nil {
		errMsg := fmt.Sprintf("Error opening database: %v", err)
		if jsonMode {
			output.PrintError(errMsg, 1)
		} else {
			fmt.Printf("Error: %s\n", errMsg)
		}
		os.Exit(1)
	}
	defer db.Close()

	// Get statistics
	stats, err := database.GetScanStats(db, nil)
	if err != nil {
		errMsg := fmt.Sprintf("Error getting database stats: %v", err)
		if jsonMode {
			output.PrintError(errMsg, 1)
		} else {
			fmt.Printf("Error: %s\n", errMsg)
		}
		os.Exit(1)
	}

	if jsonMode {
		info := output.DatabaseInfo{
			Success:           true,
			DatabasePath:      dbPath,
			TotalImages:       stats.TotalImages,
			UniqueHashes:      stats.UniqueHashes,
			DatabaseSizeBytes: fileInfo.Size(),
			LastModified:      fileInfo.ModTime().Format(time.RFC3339),
		}
		output.PrintJSON(info)
	} else {
		fmt.Printf("Database Information:\n")
		fmt.Printf("  Path: %s\n", dbPath)
		fmt.Printf("  Size: %.2f MB\n", float64(fileInfo.Size())/(1024*1024))
		fmt.Printf("  Last Modified: %s\n", fileInfo.ModTime().Format("2006-01-02 15:04:05"))
		fmt.Printf("\nStatistics:\n")
		fmt.Printf("  Total Images: %d\n", stats.TotalImages)
		fmt.Printf("  Unique Hashes: %d\n", stats.UniqueHashes)
	}
}
