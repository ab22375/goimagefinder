package signalhandler

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"syscall"
)

// SetupHandler configures signal handling for safer interaction with C libraries
// Deprecated: Use SetupWithContext for graceful shutdown support
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

// SetupWithContext creates a context that is cancelled when SIGINT or SIGTERM is received.
// This enables graceful shutdown of long-running operations.
// Returns a context and cancel function. The cancel function should be deferred.
func SetupWithContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case <-sigChan:
			cancel() // Signal all operations to stop gracefully
		case <-ctx.Done():
			// Context was cancelled elsewhere
		}
		signal.Stop(sigChan)
	}()

	return ctx, cancel
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
