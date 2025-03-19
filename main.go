package main

import (
	"database/sql"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"imagefinder/database"
	"imagefinder/imageprocessor"
	"imagefinder/logging"
	"imagefinder/scanner"
	"imagefinder/signalhandler"
	"imagefinder/utils"
)

func main() {
	// Set up proper signal handling
	signalhandler.SetupHandler()

	// Set the optimal number of CPUs to use
	runtime.GOMAXPROCS(signalhandler.GetOptimalProcs()) // <-- Change this function call

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
		utils.PrintUsage()
		os.Exit(1)
	}

	switch command {
	case "scan":
		handleScanCommand(args, dbPath, debugMode)
	case "search":
		handleSearchCommand(args, dbPath, debugMode)
	default:
		fmt.Printf("Unknown command: %s\n", command)
		utils.PrintUsage()
		os.Exit(1)
	}
}

func handleScanCommand(args map[string]string, dbPath string, debugMode bool) {
	// Setup proper signal handling
	signalhandler.SetupHandler()

	// Set optimal GOMAXPROCS
	runtime.GOMAXPROCS(signalhandler.GetOptimalProcs())

	// Get folder path with validation
	folderPath, hasFolder := args["folder"]
	if !hasFolder {
		fmt.Println("Error: Missing folder path (use --folder=PATH)")
		os.Exit(1)
	}

	// Verify folder path exists and is accessible
	folderInfo, err := os.Stat(folderPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Fatalf("Folder path does not exist: %s", folderPath)
		} else {
			log.Fatalf("Cannot access folder path: %s (%v)", folderPath, err)
		}
	}
	if !folderInfo.IsDir() {
		log.Fatalf("Path is not a directory: %s", folderPath)
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

	fmt.Printf("Starting image indexing...\n")
	fmt.Printf("Total image files to process: %d (including %d RAW files and %d TIF files)\n",
		totalImages, rawCount, tifCount)
	fmt.Printf("Force rewrite mode: %v\n", forceRewrite)
	fmt.Printf("Source prefix: %s\n", sourcePrefix)
	fmt.Printf("Debug mode: %s\n", map[bool]string{true: "enabled", false: "disabled"}[debugMode])

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
		err := scanner.ScanAndStoreFolder(db, scanOptions)
		if err != nil {
			errChan <- err
		} else {
			doneChan <- true
		}
	}()

	// Wait for completion or error
	select {
	case err := <-errChan:
		log.Fatalf("Error scanning folder: %v", err)
	case <-doneChan:
		// Print execution time
		duration := time.Since(startTime)
		fmt.Printf("\nScan completed successfully!\n")
		fmt.Printf("Total execution time: %v\n", duration)
		fmt.Printf("Database: %s\n", dbPath)

		// Print summary statistics if available
		stats, err := database.GetScanStats(db, sourcePrefix)
		if err == nil && stats != nil {
			fmt.Printf("\nSummary:\n")
			fmt.Printf("- Total images processed: %d\n", stats.TotalImages)
			fmt.Printf("- Total errors: %d\n", stats.ErrorCount)
			fmt.Printf("- Unique image hashes: %d\n", stats.UniqueHashes)
		}
	}
}
func handleSearchCommand(args map[string]string, dbPath string, debugMode bool) {
	// Get query image path
	queryPath, hasQuery := args["image"]
	if !hasQuery {
		fmt.Println("Error: Missing query image path (use --image=PATH)")
		os.Exit(1)
	}

	// Set custom threshold if provided
	threshold := 0.8 // Default threshold
	if thresholdStr, ok := args["threshold"]; ok {
		parsedThreshold, err := utils.ParseThreshold(thresholdStr)
		if err != nil {
			fmt.Printf("Warning: %v\n", err)
		} else {
			threshold = parsedThreshold
		}
	}

	// Get source prefix for filtering
	var sourcePrefix string
	if prefix, ok := args["prefix"]; ok {
		sourcePrefix = prefix
	}

	// Verify paths exist
	if _, err := os.Stat(queryPath); os.IsNotExist(err) {
		log.Fatalf("Query image does not exist: %s", queryPath)
	}

	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		log.Fatalf("Database does not exist: %s. Run scan command first.", dbPath)
	}

	startTime := time.Now()

	// Open database
	db, err := database.OpenDatabase(dbPath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}
	defer db.Close()

	fmt.Println("Searching for similar images...")
	if sourcePrefix != "" {
		fmt.Printf("Filtering by source prefix: %s\n", sourcePrefix)
	}

	// Find similar images
	searchOptions := imageprocessor.SearchOptions{
		QueryPath:    queryPath,
		Threshold:    threshold,
		SourcePrefix: sourcePrefix,
		DebugMode:    debugMode,
	}

	matches, err := imageprocessor.FindSimilarImages(db, searchOptions)
	if err != nil {
		log.Fatalf("Error finding similar images: %v", err)
	}

	// Print top matches
	fmt.Println("\nTop Matches:")
	limit := 5 // Show top 5 matches

	if len(matches) == 0 {
		fmt.Println("No matches found.")
	} else {
		for i := 0; i < limit && i < len(matches); i++ {
			fmt.Printf("%d. Image: %s\n", i+1, matches[i].Path)
			if matches[i].SourcePrefix != "" {
				fmt.Printf("   Source: %s\n", matches[i].SourcePrefix)
			}
			fmt.Printf("   SSIM Score: %.4f\n", matches[i].SSIMScore)
		}
	}

	// Print execution time
	duration := time.Since(startTime)
	fmt.Printf("\nTotal search time: %v\n", duration)
}
