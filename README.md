# ImageFinder

ImageFinder is a Go application for indexing and finding similar images across multiple directories. It uses perceptual hashing and structural similarity (SSIM) to find matches.

## Features

* Index images from directories into a SQLite database
* Search for similar images using perceptual hash and SSIM comparison
* Support for multiple image formats (JPEG, PNG, TIFF, and basic support for RAW and HEIC)
* Debug mode for detailed logging of operations
* Continue processing even when errors occur with individual images
* Source prefixes for organizing images from different sources

## Installation

### Prerequisites

* Go 1.18 or higher
* OpenCV 4.x (required for gocv)
* SQLite3

### Building from Source

1. Clone the repository:
   ```
   git clone https://github.com/yourusername/imagefinder.git
   cd imagefinder
   ```
2. Initialize the Go module (if not already done):
   ```
   make init
   ```
3. Install dependencies:
   ```
   make deps
   ```
4. Build the application:
   ```
   make build
   ```

## Usage

### Indexing Images

To scan and index a directory of images:

```
./build/imagefinder scan --folder=/path/to/images [options]
```

Options:

* `--database=PATH`: Path to database file (default: executable's directory/images.db)
* `--prefix=NAME`: Source prefix for scanning (e.g., "ExternalDrive1")
* `--force`: Force rewrite existing entries
* `--debug`: Enable debug mode with detailed logging
* `--logfile=PATH`: Specify custom log file path (default: imagefinder.log)

### Searching for Similar Images

To search for images similar to a query image:

```
./build/imagefinder search --image=/path/to/query.jpg [options]
```

Options:

* `--database=PATH`: Path to database file (default: executable's directory/images.db)
* `--threshold=VALUE`: Similarity threshold (0.0-1.0, default: 0.8)
* `--prefix=NAME`: Source prefix for filtering results
* `--debug`: Enable debug mode with detailed logging
* `--logfile=PATH`: Specify custom log file path (default: imagefinder.log)

## Debug Mode

When using the `--debug` flag, the application will create a detailed log file with information about:

* Each image processed
* Processing errors and their causes
* Hash values generated
* Image comparison details
* Database operations

This is particularly useful for:

* Identifying problematic image files
* Troubleshooting performance issues
* Understanding why certain images are matched or not matched

Example debug log output:

```
2025/03/15 10:15:23 --- ImageFinder Debug Log Started at 2025-03-15T10:15:23Z ---
2025/03/15 10:15:23 Starting image scan on folder: /home/user/photos
2025/03/15 10:15:23 Force rewrite: false, Source prefix: myCollection
2025/03/15 10:15:24 Found 1283 image files to process
2025/03/15 10:15:25 PROCESSED: /home/user/photos/img001.jpg
2025/03/15 10:15:25 FAILED: /home/user/photos/corrupted.jpg - Error: failed to load image: invalid file format
2025/03/15 10:15:25 PROCESSED: /home/user/photos/img002.jpg
...
2025/03/15 10:20:45 Scan completed in 5m22s. Processed: 1283, Errors: 7
```

## Project Structure

The application is divided into several modules:

* `main.go`: Entry point and command handling
* `database/`: Database operations and schema management
* `imageprocessor/`: Image loading, hashing, and comparison
* `scanner/`: Directory traversal and processing
* `logging/`: Debug and error logging
* `types/`: Shared data structures
* `utils/`: Utility functions for argument parsing, etc.

## Development

### Running Tests

```
make test
```

### Setting Up Development Environment

```
make setup
```

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request
