# Image Similarity Indexing and Search Tool

## Overview

This program is a command-line tool designed to **scan, index, and search** for similar images based on perceptual and average hash comparisons. It supports various image formats, including **JPG, PNG, TIFF, RAW (e.g., NEF, CR2), and HEIC**.

The program uses **SQLite** for database storage and **OpenCV (GoCV)** for image processing, allowing users to:

- Scan a folder of images and store metadata and hash values in a database.
- Search for similar images by comparing a query image against the indexed database using **hash matching and SSIM (Structural Similarity Index Measure)**.

## Features

- Supports **multiple image formats**, including RAW and HEIC.
- Uses **average hash (aHash) and perceptual hash (pHash)** for image comparison.
- Computes **SSIM similarity scores** for more accurate matching.
- Allows **incremental updates** to the database to avoid reprocessing unchanged images.
- Supports **multi-threaded processing** for efficient scanning.
- Provides **CLI commands** for scanning and searching images.

## Dependencies

To use this tool, you need to install:

- **Go** (Golang) 1.18+
- **GoCV** (OpenCV bindings for Go): Install via `go get gocv.io/x/gocv`
- **SQLite3**: Install via `go get github.com/mattn/go-sqlite3`

## Installation

Clone this repository and build the project:

```sh
git clone https://github.com/your-repo/image-similarity-indexer.git
cd image-similarity-indexer
go build -o image-indexer
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

## Example Workflow

1. **Index a directory of images**

   ```sh
   ./image-indexer scan --folder=/home/user/photos --database=photos.db --prefix=DSLR --force
   ```
2. **Find similar images to a given file**

   ```sh
   ./image-indexer search --image=/home/user/photos/query.jpg --database=photos.db --threshold=0.85
   ```

## Technical Details

### Image Processing

- **Average Hash (aHash)**: Calculates an 8x8 pixel grayscale representation and compares each pixel to the mean brightness.
- **Perceptual Hash (pHash)**: Uses a **32x32** DCT-based transformation and median filtering for robust comparisons.
- **SSIM (Structural Similarity Index)**: Measures perceptual differences between images.

### Database Schema

The SQLite database stores image metadata and hash values:

```sql
CREATE TABLE IF NOT EXISTS images (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL,
    source_prefix TEXT,
    format TEXT,
    width INTEGER,
    height INTEGER,
    created_at TEXT,
    modified_at TEXT,
    size INTEGER,
    average_hash TEXT,
    perceptual_hash TEXT,
    UNIQUE(path, source_prefix)
);
```

Indexes are created for fast lookup:

```sql
CREATE INDEX IF NOT EXISTS idx_path ON images(path);
CREATE INDEX IF NOT EXISTS idx_average_hash ON images(average_hash);
CREATE INDEX IF NOT EXISTS idx_perceptual_hash ON images(perceptual_hash);
```

## Performance Considerations

- **Concurrency**: Uses a semaphore to limit the number of concurrent processing threads (default: 8).
- **Incremental updates**: Skips unchanged images unless `--force` is specified.
- **Optimized queries**: Uses SQLite indexes to speed up searches.

## Future Enhancements

- Support for **GPU acceleration** using OpenCV CUDA.
- More **advanced hash algorithms** like Wavelet Hashing.
- Improved **HEIC and RAW processing** via external libraries.

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
