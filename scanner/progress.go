package scanner

import (
	"fmt"
	"time"

	"imagefinder/logging"
)

// NewProgressTracker initializes the progress tracker
func NewProgressTracker(stats FileStats, resultsChan chan ProcessImageResult) *ProgressTracker {
	tracker := &ProgressTracker{
		ticker:     time.NewTicker(500 * time.Millisecond),
		done:       make(chan bool),
		totalFiles: stats.totalFiles,
		rawFiles:   stats.rawFiles,
		tifFiles:   stats.tifFiles,
	}

	// Start progress display goroutine
	go tracker.displayProgress()

	// Start result processor goroutine
	go tracker.processResults(resultsChan)

	return tracker
}

// displayProgress shows the progress periodically
func (p *ProgressTracker) displayProgress() {
	for {
		select {
		case <-p.done:
			return
		case <-p.ticker.C:
			p.mu.Lock()
			if p.errors > 0 {
				fmt.Printf("\rProgress: %d/%d (Errors: %d, RAW: %d/%d, TIF: %d/%d)",
					p.processed, p.totalFiles, p.errors, p.rawProcessed, p.rawFiles, p.tifProcessed, p.tifFiles)
			} else {
				fmt.Printf("\rProgress: %d/%d (RAW: %d/%d, TIF: %d/%d)",
					p.processed, p.totalFiles, p.rawProcessed, p.rawFiles, p.tifProcessed, p.tifFiles)
			}
			p.mu.Unlock()
		}
	}
}

// processResults updates the tracker state based on processing results
func (p *ProgressTracker) processResults(resultsChan chan ProcessImageResult) {
	for result := range resultsChan {
		p.mu.Lock()
		p.processed++

		// Track RAW files
		if result.IsRaw {
			p.rawProcessed++
		}

		// Track TIF files
		if result.IsTif {
			p.tifProcessed++
		}

		if !result.Success {
			p.errors++
			if result.IsRaw {
				p.rawErrors++
			}
			if result.IsTif {
				p.tifErrors++
			}
			// Log the error
			if result.Error != nil {
				logging.LogImageProcessed(result.Path, false, result.Error.Error())
			}
		} else {
			logging.LogImageProcessed(result.Path, true, "")
		}

		p.mu.Unlock()
	}
}

// Stop ends the progress tracking
func (p *ProgressTracker) Stop() {
	p.ticker.Stop()
	p.done <- true
}

// PrintStartupInfo displays information about the scan before starting
func PrintStartupInfo(stats FileStats, options ScanOptions) {
	fmt.Printf("Starting image indexing...\nTotal image files to process: %d (including %d RAW files and %d TIF files)\n",
		stats.totalFiles, stats.rawFiles, stats.tifFiles)
	fmt.Printf("Force rewrite mode: %v\n", options.ForceRewrite)

	if options.SourcePrefix != "" {
		fmt.Printf("Source prefix: %s\n", options.SourcePrefix)
	}

	if options.DebugMode {
		fmt.Printf("Debug mode: enabled\n")
		logging.DebugLog("Found %d image files to process (%d RAW files, %d TIF files)",
			stats.totalFiles, stats.rawFiles, stats.tifFiles)
	}
}

// PrintCompletionStats displays statistics after scan completion
func PrintCompletionStats(tracker *ProgressTracker, startTime time.Time, options ScanOptions) {
	elapsed := time.Since(startTime)

	// Log final statistics
	if options.DebugMode {
		logging.DebugLog("Scan completed in %v. Processed: %d, Errors: %d, RAW files: %d, RAW errors: %d, TIF files: %d, TIF errors: %d",
			elapsed, tracker.processed, tracker.errors, tracker.rawProcessed, tracker.rawErrors,
			tracker.tifProcessed, tracker.tifErrors)
	}

	fmt.Println("\nIndexing complete.")
	fmt.Printf("Processed %d images in %v.\n", tracker.processed, elapsed.Round(time.Second))

	if tracker.rawProcessed > 0 {
		fmt.Printf("Successfully processed %d/%d RAW image files.\n",
			tracker.rawProcessed-tracker.rawErrors, tracker.rawFiles)
	}

	if tracker.tifProcessed > 0 {
		fmt.Printf("Successfully processed %d/%d TIF image files.\n",
			tracker.tifProcessed-tracker.tifErrors, tracker.tifFiles)
	}

	if tracker.errors > 0 {
		fmt.Printf("Encountered %d errors during indexing.\n", tracker.errors)
		fmt.Println("Check the log file for details.")
	}
}
