package signalhandler

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

// SetupHandler configures signal handling for safer interaction with C libraries
func SetupHandler() {
	// Create a channel to receive OS signals
	sigChan := make(chan os.Signal, 1)

	// Register for specific signals
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Handle signals in a separate goroutine
	go func() {
		sig := <-sigChan
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			// Clean shutdown
			os.Exit(0)
		}
	}()
}

// GetOptimalProcs returns the optimal number of worker goroutines for the system
func GetOptimalProcs() int {
	// Get the number of CPUs available
	numCPU := runtime.NumCPU()

	// For image processing with CGo, using too many goroutines can cause issues
	maxProcs := (numCPU * 3) / 4
	if maxProcs < 1 {
		maxProcs = 1
	}

	return maxProcs
}

// GetMaxProcs returns the optimal number of worker goroutines for the system
func GetMaxProcs() int {
	// Get the number of CPUs available
	numCPU := runtime.NumCPU()

	// For image processing with CGo, using too many goroutines can cause issues
	maxProcs := (numCPU * 3) / 4
	if maxProcs < 1 {
		maxProcs = 1
	}

	return maxProcs
}
