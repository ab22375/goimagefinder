package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"imagefinder/database"
	"imagefinder/imageprocessor"
	"imagefinder/logging"
	"imagefinder/scanner"
	"imagefinder/utils"
)

func main() {
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
	// Get folder path
	folderPath, hasFolder := args["folder"]
	if !hasFolder {
		fmt.Println("Error: Missing folder path (use --folder=PATH)")
		os.Exit(1)
	}

	// Get source prefix
	sourcePrefix := ""
	if prefix, ok := args["prefix"]; ok {
		sourcePrefix = prefix
	}

	// Get force rewrite flag
	forceRewrite := false
	if _, ok := args["force"]; ok {
		forceRewrite = true
	}

	// Verify folder path exists
	if _, err := os.Stat(folderPath); os.IsNotExist(err) {
		log.Fatalf("Folder path does not exist: %s", folderPath)
	}

	startTime := time.Now()

	// Initialize database
	db, err := database.InitDatabase(dbPath)
	if err != nil {
		log.Fatalf("Error initializing database: %v", err)
	}
	defer db.Close()

	// Scan folder and store image information
	scanOptions := scanner.ScanOptions{
		FolderPath:   folderPath,
		SourcePrefix: sourcePrefix,
		ForceRewrite: forceRewrite,
		DebugMode:    debugMode,
		DbPath:       dbPath,
	}

	err = scanner.ScanAndStoreFolder(db, scanOptions)
	if err != nil {
		log.Fatalf("Error scanning folder: %v", err)
	}

	// Print execution time
	duration := time.Since(startTime)
	fmt.Printf("\nTotal execution time: %v\n", duration)
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
