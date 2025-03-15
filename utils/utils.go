package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ParseArguments converts command-line arguments into a map of flags and values
func ParseArguments() map[string]string {
	args := make(map[string]string)

	// First, identify the command (scan/search)
	command := ""
	commandIndex := -1
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "scan" || os.Args[i] == "search" {
			command = os.Args[i]
			commandIndex = i
			break
		}
	}

	if command != "" {
		args["command"] = command
	}

	// Process all arguments, skipping the command
	for i := 1; i < len(os.Args); i++ {
		if i == commandIndex {
			continue
		}

		arg := os.Args[i]

		// Handle flags with equals sign (--key=value)
		if strings.HasPrefix(arg, "--") && strings.Contains(arg, "=") {
			parts := strings.SplitN(arg, "=", 2)
			flagName := strings.TrimPrefix(parts[0], "--")
			args[flagName] = parts[1]
			continue
		}

		// Handle flags without equals sign (--key value)
		if strings.HasPrefix(arg, "--") {
			flagName := strings.TrimPrefix(arg, "--")

			// Check if this is a boolean flag (no value)
			if i+1 >= len(os.Args) || strings.HasPrefix(os.Args[i+1], "--") {
				args[flagName] = "true"
			} else {
				// The next argument is the value
				args[flagName] = os.Args[i+1]
				i++ // Skip the value in the next iteration
			}
		}
	}

	return args
}

// GetDefaultDatabasePath returns the default path for the database file
func GetDefaultDatabasePath() string {
	// Get the executable path
	exePath, err := os.Executable()
	if err != nil {
		// Fallback to current directory if executable path can't be determined
		return "images.db"
	}

	// Get the directory containing the executable
	exeDir := filepath.Dir(exePath)

	// Return the default database path in the same directory
	return filepath.Join(exeDir, "images.db")
}

// PrintUsage outputs the command-line usage instructions
func PrintUsage() {
	fmt.Printf("Usage:\n")
	fmt.Printf("  %s scan --folder=PATH [--database=PATH] [--prefix=NAME] [--force] [--debug] [--logfile=PATH]\n", os.Args[0])
	fmt.Printf("  %s search --image=PATH [--database=PATH] [--threshold=VALUE] [--prefix=NAME] [--debug] [--logfile=PATH]\n", os.Args[0])
	fmt.Printf("\nParameters:\n")
	fmt.Printf("  --folder      : Path to folder containing images to scan\n")
	fmt.Printf("  --image       : Path to query image for search\n")
	fmt.Printf("  --database    : Path to database file (default: %s)\n", GetDefaultDatabasePath())
	fmt.Printf("  --prefix      : Source prefix for scanning/filtering results\n")
	fmt.Printf("  --force       : Force rewrite existing entries during scan\n")
	fmt.Printf("  --threshold   : Similarity threshold for search (0.0-1.0, default: 0.8)\n")
	fmt.Printf("  --debug       : Enable debug mode (logs detailed information)\n")
	fmt.Printf("  --logfile     : Specify custom log file path (default: imagefinder.log)\n")
	fmt.Printf("\nExamples:\n")
	fmt.Printf("  %s scan --folder=/path/to/images --prefix=ExternalDrive1 --debug\n", os.Args[0])
	fmt.Printf("  %s search --image=/path/to/query.jpg --threshold=0.85\n", os.Args[0])
}

// ParseThreshold parses and validates the threshold value from string
func ParseThreshold(thresholdStr string) (float64, error) {
	parsedThreshold, err := strconv.ParseFloat(thresholdStr, 64)
	if err != nil || parsedThreshold < 0 || parsedThreshold > 1 {
		return 0.8, fmt.Errorf("Invalid threshold value '%s', using default (0.8)", thresholdStr)
	}
	return parsedThreshold, nil
}
