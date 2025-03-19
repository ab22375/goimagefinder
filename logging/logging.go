package logging

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"
)

var (
	debugLogger *log.Logger
	logFile     *os.File
	mu          sync.Mutex
	isSetup     bool
)

// SetupLogger initializes the debug logger with the specified log file
func SetupLogger(logFilePath string) error {
	mu.Lock()
	defer mu.Unlock()

	// Check if logger is already set up
	if isSetup {
		return nil
	}

	// Open log file
	var err error
	logFile, err = os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	// Create logger with timestamp prefix
	debugLogger = log.New(logFile, "", log.LstdFlags)

	// Log startup information
	debugLogger.Printf("--- ImageFinder Debug Log Started at %s ---\n", time.Now().Format(time.RFC3339))

	isSetup = true
	return nil
}

// CloseLogger closes the log file
func CloseLogger() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		debugLogger.Printf("--- ImageFinder Debug Log Closed at %s ---\n", time.Now().Format(time.RFC3339))
		logFile.Close()
		logFile = nil
		isSetup = false
	}
}

// LogInfo logs an information message (added to match usage in format_loader_implementations.go)
func LogInfo(format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()

	if debugLogger != nil {
		debugLogger.Printf("INFO: "+format, args...)
	} else {
		// Fallback to standard output if logger is not set up
		log.Printf("INFO: "+format, args...)
	}
}

// DebugLog logs a message if debug mode is enabled
func DebugLog(format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()

	if debugLogger != nil {
		debugLogger.Printf(format, args...)
	}
}

// LogError logs an error message
func LogError(format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()

	if debugLogger != nil {
		debugLogger.Printf("ERROR: "+format, args...)
	}
}

// LogWarning logs a warning message
func LogWarning(format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()

	if debugLogger != nil {
		debugLogger.Printf("WARNING: "+format, args...)
	}
}

// LogImageProcessed logs when an image is processed
func LogImageProcessed(path string, success bool, errMsg string) {
	mu.Lock()
	defer mu.Unlock()

	if debugLogger != nil {
		if success {
			debugLogger.Printf("PROCESSED: %s", path)
		} else {
			debugLogger.Printf("FAILED: %s - Error: %s", path, errMsg)
		}
	}
}
