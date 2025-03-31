package scanner

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"time"

	"imagefinder/database"
	"imagefinder/imageprocessor"
	"imagefinder/logging"
	"imagefinder/scanner/processor"
	"imagefinder/types"
)

// ScanAndStoreFolder scans a folder and stores image information in the database
func ScanAndStoreFolder(db *sql.DB, options ScanOptions) error {
	// Determine concurrency limit
	maxWorkers := 8 // Default
	if options.MaxWorkers > 0 {
		maxWorkers = options.MaxWorkers
	}

	// Initialize components for parallel processing
	var wg sync.WaitGroup
	resultsChan := make(chan ProcessImageResult, maxWorkers*2) // Buffer size increased
	semaphore := make(chan struct{}, maxWorkers)

	// Count and classify files before processing
	fileStats := countFilesToProcess(options)

	// Display initial information
	PrintStartupInfo(fileStats, options)

	// Set up progress tracking
	progressTracker := NewProgressTracker(fileStats, resultsChan)
	defer progressTracker.Stop()

	// Process files
	startTime := time.Now()
	err := walkAndProcessFiles(db, options, &wg, resultsChan, semaphore)

	// Wait for all processing to complete
	wg.Wait()
	close(resultsChan)

	// Wait a short time for the result processor to finish
	time.Sleep(100 * time.Millisecond)

	// Clean up
	close(semaphore)

	// Print final statistics
	PrintCompletionStats(progressTracker, startTime, options)

	return err
}

// countFilesToProcess counts and classifies files to be processed
func countFilesToProcess(options ScanOptions) FileStats {
	stats := FileStats{}
	loaderRegistry := imageprocessor.NewImageLoaderRegistry() // Use root registry directly

	if options.DebugMode {
		logging.DebugLog("Starting image scan on folder: %s", options.FolderPath)
		logging.DebugLog("Force rewrite: %v, Source prefix: %s", options.ForceRewrite, options.SourcePrefix)
	}

	filepath.Walk(options.FolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			logging.LogError("Error accessing path %s: %v", path, err)
			return nil
		}

		if info == nil || info.IsDir() {
			return nil
		}

		// Check if this is an image file we can process
		if loaderRegistry.CanLoadFile(path) || IsImageFile(path) {
			stats.totalFiles++

			// Check if it's a RAW file
			if IsRawFormat(path) {
				stats.rawFiles++
			}

			// Check if it's a TIF file
			if IsTiffFormat(path) {
				stats.tifFiles++
			}
		}
		return nil
	})

	return stats
}

func walkAndProcessFiles(db *sql.DB, options ScanOptions, wg *sync.WaitGroup, resultsChan chan ProcessImageResult, semaphore chan struct{}) error {
	logging.DebugLog("Starting walkAndProcessFiles - folder: %s, debug: %t, semaphore capacity: %d",
		options.FolderPath, options.DebugMode, cap(semaphore))

	// Track statistics for reporting with enhanced semaphore tracking
	stats := struct {
		sync.Mutex
		filesFound            int
		filesProcessed        int
		filesSkipped          int
		errorCount            int
		rawCount              int
		tifCount              int
		bufferOverflows       int
		resultsSent           int
		resultsForwarded      int
		sendTimeouts          int
		semaphoreAcquisitions int
		semaphoreReleases     int
		semaphoreTimeouts     int
		semaphoreAbandonments int
	}{}

	// Create image processor from our new package
	imgProcessor := processor.NewImageProcessor(options.DebugMode)
	logging.DebugLog("Image processor created")

	// Create registry to identify image files
	loaderRegistry := imageprocessor.NewImageLoaderRegistry()

	logging.DebugLog("Image loader registry created")

	// Create a properly sized results buffer
	bufferSize := 1000 // Moderate buffer size
	resultsBuffer := make(chan ProcessImageResult, bufferSize)
	logging.DebugLog("Created results buffer with size: %d", bufferSize)

	// Create channels for coordination
	forwarderDone := make(chan struct{})
	resultsForwarderCompleted := make(chan struct{})

	// Add a buffer status monitor
	bufferMonitorDone := make(chan struct{})
	go func() {
		defer close(bufferMonitorDone)
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				bufferLen := len(resultsBuffer)
				bufferCap := cap(resultsBuffer)
				usage := float64(bufferLen) / float64(bufferCap) * 100

				stats.Lock()
				semaphoreDiff := stats.semaphoreAcquisitions - stats.semaphoreReleases
				statsSnapshot := struct {
					found, processed, sent, forwarded, semAcq, semRel, semDiff, semTimeouts, semAbandoned int
				}{
					stats.filesFound,
					stats.filesProcessed,
					stats.resultsSent,
					stats.resultsForwarded,
					stats.semaphoreAcquisitions,
					stats.semaphoreReleases,
					semaphoreDiff,
					stats.semaphoreTimeouts,
					stats.semaphoreAbandonments,
				}
				stats.Unlock()

				logging.DebugLog("STATUS: Buffer: %d/%d (%.1f%%) | Files: found=%d processed=%d | Results: sent=%d forwarded=%d | Semaphore: acq=%d rel=%d diff=%d timeouts=%d abandoned=%d",
					bufferLen, bufferCap, usage,
					statsSnapshot.found, statsSnapshot.processed,
					statsSnapshot.sent, statsSnapshot.forwarded,
					statsSnapshot.semAcq, statsSnapshot.semRel, statsSnapshot.semDiff,
					statsSnapshot.semTimeouts, statsSnapshot.semAbandoned)

				// Check for potential leaks
				if statsSnapshot.semDiff > 0 && statsSnapshot.semDiff >= cap(semaphore)/2 {
					logging.LogError("WARNING: Potential semaphore leak detected - %d more acquisitions than releases",
						statsSnapshot.semDiff)
				}

				if usage > 80 {
					logging.LogError("WARNING: Results buffer is getting full (%d/%d, %.1f%%)",
						bufferLen, bufferCap, usage)
				}
			case <-forwarderDone:
				return
			}
		}
	}()

	// Start improved result forwarder
	go func() {
		defer func() {
			if r := recover(); r != nil {
				stack := debug.Stack()
				logging.LogError("Result forwarder goroutine panicked: %v\nStack: %s", r, string(stack))
			}
			logging.DebugLog("Result forwarder goroutine completed")
			close(resultsForwarderCompleted)
		}()

		logging.DebugLog("Result forwarder goroutine started")
		forwardCount := 0
		timeoutCount := 0
		consecutiveTimeouts := 0
		maxConsecutiveTimeouts := 5

		forwarderRunning := true
		for forwarderRunning {
			select {
			case result, ok := <-resultsBuffer:
				if !ok {
					logging.DebugLog("Results buffer channel closed, exiting forwarder")
					forwarderRunning = false
					break
				}

				logging.DebugLog("Attempting to forward result #%d for: %s", forwardCount+1, result.Path)

				// Use tiered timeout approach for forwarding
				sendStart := time.Now()
				sendSuccess := false

				// Try a quick send first, then gradually increase timeout
				for _, timeout := range []time.Duration{500 * time.Millisecond, 1 * time.Second, 2 * time.Second} {
					select {
					case resultsChan <- result:
						consecutiveTimeouts = 0 // Reset on success
						forwardCount++
						stats.Lock()
						stats.resultsForwarded++
						stats.Unlock()
						sendSuccess = true

						logging.DebugLog("Successfully forwarded result #%d for: %s",
							forwardCount, result.Path)
						break
					case <-time.After(timeout):
						// Try next timeout level
						logging.DebugLog("Forward timeout at %v for: %s, trying longer timeout",
							timeout, result.Path)
						continue
					case <-forwarderDone:
						logging.DebugLog("Received stop signal in forwarder while forwarding %s", result.Path)
						forwarderRunning = false
						break
					}

					if sendSuccess || !forwarderRunning {
						break
					}
				}

				if !sendSuccess && forwarderRunning {
					timeoutCount++
					consecutiveTimeouts++
					stats.Lock()
					stats.sendTimeouts++
					stats.Unlock()

					logging.LogError("TIMEOUT (#%d) sending result for %s after %v: result channel blocked",
						timeoutCount, result.Path, time.Since(sendStart))

					if consecutiveTimeouts >= maxConsecutiveTimeouts {
						logging.LogError("CRITICAL: %d consecutive timeouts in forwarder - potential deadlock",
							consecutiveTimeouts)
					}
				}

			case <-forwarderDone:
				logging.DebugLog("Received stop signal in forwarder")
				forwarderRunning = false
				break
			}
		}

		logging.DebugLog("Result forwarder finished. Forwarded: %d, Timeouts: %d", forwardCount, timeoutCount)
	}()

	// Track worker goroutine status
	workerStatus := struct {
		sync.Mutex
		active   int
		complete int
		failed   int
	}{}

	// Create a WaitGroup specific for file processing workers
	var fileWorkersWg sync.WaitGroup

	// Collect all files first before processing
	var filesToProcess []string
	logging.DebugLog("Starting directory scan to collect files: %s", options.FolderPath)
	scanStartTime := time.Now()

	err := filepath.Walk(options.FolderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if options.DebugMode {
				logging.DebugLog("Failed to access path %s: %v", path, err)
				logging.LogError("Failed to access path %s: %v", path, err)
			}

			stats.Lock()
			stats.errorCount++
			stats.Unlock()

			return nil // Continue with other files
		}

		// Skip directories
		if info == nil || info.IsDir() {
			return nil
		}

		// Add to found files counter
		stats.Lock()
		stats.filesFound++
		currentFound := stats.filesFound
		stats.Unlock()

		// Log progress periodically
		if options.DebugMode && currentFound%1000 == 0 {
			logging.DebugLog("Found %d files so far during scan", currentFound)
		}

		// Skip files that we can't handle
		if !loaderRegistry.CanLoadFile(path) && !imageprocessor.IsImageFile(path) {
			if options.DebugMode {
				logging.DebugLog("Skipping non-image file: %s", path)
			}

			stats.Lock()
			stats.filesSkipped++
			stats.Unlock()

			return nil
		}

		// Add path to the list
		filesToProcess = append(filesToProcess, path)

		return nil
	})

	scanDuration := time.Since(scanStartTime)
	logging.DebugLog("Directory scan completed in %v, found %d files to process",
		scanDuration, len(filesToProcess))

	if err != nil {
		logging.LogError("Error during directory scan: %v", err)
		return err
	}

	// Process files in chunks to control concurrency
	chunkSize := 100 // Process this many files at a time
	totalFiles := len(filesToProcess)

	logging.DebugLog("Processing %d files in chunks of %d", totalFiles, chunkSize)

	// Process files in chunks
	for chunkStart := 0; chunkStart < totalFiles; chunkStart += chunkSize {
		chunkEnd := chunkStart + chunkSize
		if chunkEnd > totalFiles {
			chunkEnd = totalFiles
		}

		currentChunk := filesToProcess[chunkStart:chunkEnd]
		logging.DebugLog("Processing chunk %d-%d of %d files", chunkStart+1, chunkEnd, totalFiles)

		// Create worker goroutines for this chunk
		for fileIndex, filePath := range currentChunk {
			// Add to main WaitGroup
			wg.Add(1)

			// Also add to file workers WaitGroup
			fileWorkersWg.Add(1)

			// Track active workers
			workerStatus.Lock()
			workerStatus.active++
			activeWorkers := workerStatus.active
			workerStatus.Unlock()

			if options.DebugMode && activeWorkers%10 == 0 {
				logging.DebugLog("Currently %d active worker goroutines", activeWorkers)
			}

			// Global file number (for logging)
			fileNum := chunkStart + fileIndex + 1

			// Launch worker goroutine with better timeout handling
			go func(filePath string, fileNum int) {
				startTime := time.Now()

				// Mark worker as complete when done
				defer func() {
					workerStatus.Lock()
					workerStatus.active--
					workerStatus.complete++
					completeCount := workerStatus.complete
					workerStatus.Unlock()

					if options.DebugMode && completeCount%100 == 0 {
						logging.DebugLog("Completed %d/%d files so far", completeCount, totalFiles)
					}

					// Notify both WaitGroups when done
					fileWorkersWg.Done()
					wg.Done()
				}()

				if options.DebugMode {
					logging.DebugLog("Starting worker for file #%d: %s", fileNum, filePath)
				}

				// Acquire semaphore with timeout to prevent deadlock
				if options.DebugMode {
					logging.DebugLog("Worker #%d waiting for semaphore", fileNum)
				}

				// Use timeout when acquiring semaphore to prevent deadlocks
				semaphoreAcquired := false

				// Track semaphore acquisition attempt
				stats.Lock()
				stats.Unlock()

				for i := 0; i < 3; i++ { // Try 3 times
					select {
					case semaphore <- struct{}{}:
						semaphoreAcquired = true

						// Track successful acquisition
						stats.Lock()
						stats.semaphoreAcquisitions++
						stats.Unlock()

						if options.DebugMode {
							logging.DebugLog("Worker #%d acquired semaphore", fileNum)
						}
						break
					case <-time.After(3 * time.Second):
						stats.Lock()
						stats.semaphoreTimeouts++
						stats.Unlock()

						logging.LogError("Worker #%d timed out waiting for semaphore (attempt %d/3)",
							fileNum, i+1)
					}
					if semaphoreAcquired {
						break
					}
				}

				if !semaphoreAcquired {
					stats.Lock()
					stats.semaphoreAbandonments++
					stats.Unlock()

					logging.LogError("Worker #%d FAILED to acquire semaphore after 3 attempts - ABANDONING", fileNum)
					workerStatus.Lock()
					workerStatus.failed++
					workerStatus.Unlock()
					return
				}

				// Ensure semaphore is released even if processing panics
				defer func() {
					<-semaphore

					// Track release
					stats.Lock()
					stats.semaphoreReleases++
					stats.Unlock()

					if options.DebugMode {
						logging.DebugLog("Worker #%d released semaphore", fileNum)
					}
				}()

				// Check file type
				isRawImage := imageprocessor.IsRawFormat(filePath)
				isTifImage := imageprocessor.IsTiffFormat(filePath)

				if isRawImage {
					stats.Lock()
					stats.rawCount++
					stats.Unlock()
				}

				if isTifImage {
					stats.Lock()
					stats.tifCount++
					stats.Unlock()
				}

				// Process image with panic recovery
				var result ProcessImageResult

				func() {
					// Use defer to catch panics
					defer func() {
						if r := recover(); r != nil {
							stack := debug.Stack()
							err := fmt.Errorf("panic during image processing: %v", r)
							result = ProcessImageResult{
								Path:    filePath,
								Success: false,
								Error:   err,
								IsRaw:   isRawImage,
								IsTif:   isTifImage,
							}
							logging.LogError("Recovered from panic processing #%d %s: %v\nStack: %s",
								fileNum, filePath, r, string(stack))

							workerStatus.Lock()
							workerStatus.failed++
							workerStatus.Unlock()
						}
					}()

					// Process the image
					if options.DebugMode {
						logging.DebugLog("Processing file #%d: %s", fileNum, filePath)
					}

					result = processAndStoreImage(db, filePath, options.SourcePrefix, options, imgProcessor)
					result.IsRaw = isRawImage
					result.IsTif = isTifImage

					if options.DebugMode {
						logging.DebugLog("Completed processing file #%d: %s, success: %t",
							fileNum, filePath, result.Success)
					}

					// Log the processed image result
					if result.Success {
						logging.LogImageProcessed(filePath, true, "")
					} else if result.Error != nil {
						logging.LogImageProcessed(filePath, false, result.Error.Error())
					}
				}()

				// Track statistics
				stats.Lock()
				stats.filesProcessed++
				stats.Unlock()

				// Use tiered retry approach for sending to buffer
				if options.DebugMode {
					logging.DebugLog("Worker #%d attempting to send result to buffer", fileNum)
				}

				sendStartTime := time.Now()
				sendSuccess := false

				// Try multiple times with increasing timeouts
				for attemptNum, timeout := range []time.Duration{
					100 * time.Millisecond,
					300 * time.Millisecond,
					500 * time.Millisecond,
					1 * time.Second,
				} {
					select {
					case resultsBuffer <- result:
						sendSuccess = true

						if options.DebugMode {
							elapsed := time.Since(sendStartTime)
							logging.DebugLog("Worker #%d successfully sent to buffer (attempt %d, after %v)",
								fileNum, attemptNum+1, elapsed)
						}

						stats.Lock()
						stats.resultsSent++
						sentCount := stats.resultsSent
						stats.Unlock()

						if options.DebugMode && sentCount%100 == 0 {
							logging.DebugLog("Sent %d results to buffer so far", sentCount)
						}
						break
					case <-time.After(timeout):
						if options.DebugMode {
							logging.DebugLog("Worker #%d timeout #%d (%v) sending to buffer - retrying...",
								fileNum, attemptNum+1, timeout)
						}
						continue
					}

					if sendSuccess {
						break
					}
				}

				if !sendSuccess {
					stats.Lock()
					stats.bufferOverflows++
					stats.Unlock()

					logging.LogError("Worker #%d FAILED to send to buffer after multiple retries - DROPPING RESULT",
						fileNum)
				}

				elapsed := time.Since(startTime)
				if options.DebugMode {
					logging.DebugLog("Worker for file #%d: %s completed in %v", fileNum, filePath, elapsed)
				}
			}(filePath, fileNum)
		}

		// Wait for all workers in this chunk to complete
		chunkWaitStart := time.Now()
		logging.DebugLog("Waiting for chunk workers to complete...")
		fileWorkersWg.Wait()
		logging.DebugLog("Chunk processing completed in %v", time.Since(chunkWaitStart))
	}

	logging.DebugLog("All file processors completed")

	// Signal the forwarder to stop and wait for it to finish
	logging.DebugLog("Signaling result forwarder to stop")
	close(forwarderDone)

	// Close the results buffer - this will cause the forwarder to exit
	logging.DebugLog("Closing results buffer")
	close(resultsBuffer)

	// Wait for the forwarder to complete with timeout
	logging.DebugLog("Waiting for result forwarder to complete")
	select {
	case <-resultsForwarderCompleted:
		logging.DebugLog("Result forwarder completed normally")
	case <-time.After(30 * time.Second):
		logging.LogError("TIMEOUT: Result forwarder did not complete within timeout - continuing anyway")
	}

	// Wait for buffer monitor to complete
	<-bufferMonitorDone

	// Final stats
	stats.Lock()
	semaphoreDiff := stats.semaphoreAcquisitions - stats.semaphoreReleases
	statsSnapshot := struct {
		found, processed, skipped, errors, raw, tif        int
		sent, forwarded, timeouts, bufferOverflows         int
		semAcq, semRel, semDiff, semTimeouts, semAbandoned int
	}{
		stats.filesFound, stats.filesProcessed, stats.filesSkipped, stats.errorCount,
		stats.rawCount, stats.tifCount,
		stats.resultsSent, stats.resultsForwarded, stats.sendTimeouts, stats.bufferOverflows,
		stats.semaphoreAcquisitions, stats.semaphoreReleases, semaphoreDiff,
		stats.semaphoreTimeouts, stats.semaphoreAbandonments,
	}
	stats.Unlock()

	logging.DebugLog("Walk completed with stats:\n"+
		"  Files: Found=%d, Processed=%d, Skipped=%d, Errors=%d\n"+
		"  Types: RAW=%d, TIF=%d\n"+
		"  Results: Sent=%d, Forwarded=%d, Timeouts=%d, Dropped=%d\n"+
		"  Semaphore: Acquired=%d, Released=%d, Diff=%d, Timeouts=%d, Abandonments=%d",
		statsSnapshot.found, statsSnapshot.processed, statsSnapshot.skipped, statsSnapshot.errors,
		statsSnapshot.raw, statsSnapshot.tif,
		statsSnapshot.sent, statsSnapshot.forwarded, statsSnapshot.timeouts, statsSnapshot.bufferOverflows,
		statsSnapshot.semAcq, statsSnapshot.semRel, statsSnapshot.semDiff,
		statsSnapshot.semTimeouts, statsSnapshot.semAbandoned)

	return err
}

// processAndStoreImage processes a single image and stores it in the database
func processAndStoreImage(db *sql.DB, path string, sourcePrefix string, options ScanOptions, imgProcessor *processor.ImageProcessor) ProcessImageResult {
	result := ProcessImageResult{
		Path:    path,
		Success: false,
	}

	// Skip processing if the image already exists and hasn't been modified
	if !options.ForceRewrite {
		if skipResult := checkAndSkipIfUnchanged(db, path, sourcePrefix, options); skipResult != nil {
			return *skipResult
		}
	}

	// Get file info and format
	fileInfo, err := os.Stat(path)
	if err != nil {
		result.Error = fmt.Errorf("cannot stat file %s: %v", path, err)
		return result
	}

	fileFormat := string(imageprocessor.GetFileFormat(path))
	isRawImage := imageprocessor.IsRawFormat(path)
	isTifImage := imageprocessor.IsTiffFormat(path)

	// Load and process the image
	img, err := imgProcessor.ProcessImage(path, isRawImage, isTifImage)
	if err != nil {
		result.Error = fmt.Errorf("failed to load image %s: %v", path, err)
		return result
	}
	defer img.Close()

	// Skip empty images
	if img.Empty() {
		result.Error = fmt.Errorf("image is empty after loading: %s", path)
		return result
	}

	// Compute hashes
	imageHashes, err := imgProcessor.ComputeImageHashes(img, path, fileFormat, isRawImage, isTifImage)
	if err != nil {
		result.Error = err
		return result
	}

	// Create and store image info
	imageInfo := types.ImageInfo{
		Path:           path,
		SourcePrefix:   sourcePrefix,
		Format:         fileFormat,
		Width:          img.Cols(),
		Height:         img.Rows(),
		ModifiedAt:     fileInfo.ModTime().Format(time.RFC3339),
		Size:           fileInfo.Size(),
		AverageHash:    imageHashes.AvgHash,
		PerceptualHash: imageHashes.PHash,
		IsRawFormat:    isRawImage,
	}

	// Store in database
	err = database.StoreImageInfo(db, imageInfo, options.ForceRewrite)
	if err != nil {
		result.Error = fmt.Errorf("cannot store data for %s: %v", path, err)
		return result
	}

	if options.DebugMode && (isRawImage || isTifImage) {
		logging.DebugLog("Successfully indexed %s image: %s", fileFormat, path)
	}

	result.Success = true
	return result
}
