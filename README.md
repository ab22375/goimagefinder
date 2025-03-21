# Image Similarity Indexing and Search Tool

## Overview

This program is a command-line tool designed to **scan, index, and search** for similar images based on perceptual and average hash comparisons. It supports various image formats, including **JPG, PNG, TIFF, RAW (e.g., NEF, CR2), and HEIC**.

The program uses **SQLite** for database storage and **OpenCV (GoCV)** for image processing, allowing users to:

- Scan a folder of images and store metadata and hash values in a database.
- Search for similar images by comparing a query image against the indexed database using **hash matching and similarity scoring**.

## Features

- **Multi-format support**: Handles standard (JPG, PNG), RAW (NEF, CR2, RAF, ARW, CR3, DNG), and TIFF formats
- **Robust hash algorithms**: Uses average hash (aHash) and perceptual hash (pHash) for image comparison
- **Smart image similarity**: Computes weighted similarity scores with filename matching boosts
- **Incremental scanning**: Avoids reprocessing unchanged files to save time
- **Multi-threaded processing**: Efficiently processes images in parallel
- **Specialized format handling**: Format-specific loaders for different camera RAW formats
- **Detailed logging**: Comprehensive debug logging system

## Dependencies

To use this tool, you need to install:

- **Go** (Golang) 1.18+
- **GoCV** (OpenCV bindings for Go): `go get gocv.io/x/gocv`
- **SQLite3**: `go get github.com/mattn/go-sqlite3`

### External Tools (Optional)

Some advanced features require external tools:

- **exiftool**: For extracting preview images from RAW files
- **dcraw**: For converting RAW images
- **rawtherapee-cli**: Alternative RAW processor
- **ImageMagick/VIPS/GDAL**: For advanced TIFF processing

## Installation for Mac Silicon (ARM64)

**Download the DMG from:**

https://github.com/ab22375/search_image/tree/main/dist

After installing, create a symlink in your PATH:

```bash
sudo rm -f /usr/local/bin/goimagefinder
echo '#!/bin/bash' | sudo tee /usr/local/bin/goimagefinder > /dev/null
echo 'exec /Applications/goimagefinder.app/Contents/MacOS/goimagefinder "$@"' | sudo tee -a /usr/local/bin/goimagefinder > /dev/null
sudo chmod +x /usr/local/bin/goimagefinder
```

## Usage

### Indexing Images

To scan and index a directory of images:

```bash
goimagefinder scan --folder=/path/to/images [options]
```

Options:

* `--database=PATH` or `--db=PATH`: Path to database file (default: executable's directory/images.db)
* `--prefix=NAME`: Source prefix for scanning (e.g., "ExternalDrive1")
* `--force`: Force rewrite existing entries
* `--debug`: Enable debug mode with detailed logging
* `--logfile=PATH`: Specify custom log file path (default: imagefinder.log)

Terminal convenience example:

```bash
F="/path/to/folder/to/scan"
L="/path/to/log/file.log"
D="/path/to/sqlite/database.db"
P="prefix-name"
goimagefinder scan --folder=$F --database=$D --prefix=$P --debug --logfile=$L
```

### Searching for Similar Images

To search for images similar to a query image:

```bash
goimagefinder search --image=/path/to/query.jpg [options]
```

Options:

* `--database=PATH` or `--db=PATH`: Path to database file (default: executable's directory/images.db)
* `--threshold=VALUE`: Similarity threshold (0.0-1.0, default: 0.8)
* `--prefix=NAME`: Source prefix for filtering results
* `--debug`: Enable debug mode with detailed logging
* `--logfile=PATH`: Specify custom log file path (default: imagefinder.log)

Terminal convenience example:

```bash
D="/path/to/sqlite/database.db"
I="/path/to/image/to/search.jpg"
L="/path/to/log/file.log"
goimagefinder search --database=$D --debug --logfile=$L --image=$I
```

## Example Workflow

1. **Index a directory of images**

   ```bash
   goimagefinder scan --folder=/home/user/photos --database=photos.db --prefix=DSLR --force
   ```

2. **Find similar images to a given file**

   ```bash
   goimagefinder search --image=/home/user/photos/query.jpg --database=photos.db --threshold=0.85
   ```

## Technical Details

### Image Processing

The tool uses multiple approaches for image similarity detection:

- **Average Hash (aHash)**: Calculates an 8x8 pixel grayscale representation and compares each pixel to the mean brightness.
- **Perceptual Hash (pHash)**: Uses a **32x32** DCT-based transformation and median filtering for robust comparisons.
- **Filename similarity**: Adds a small boost when filenames are similar (e.g., IMG_1234.JPG and IMG_1234.CR2).

### RAW Image Handling

The program implements specialized loaders for various RAW formats:

- Uses embedded preview extraction when possible (via exiftool)
- Falls back to dcraw/rawtherapee for RAW conversion
- Supports format-specific optimizations for RAF, NEF, ARW, CR2, CR3, and DNG files

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
    features BLOB,
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

- **Concurrency**: Uses a semaphore to limit the number of concurrent processing threads (default: optimal for your CPU).
- **Incremental updates**: Skips unchanged images unless `--force` is specified.
- **Optimized queries**: Uses SQLite indexes to speed up searches.
- **Specialized loaders**: Format-specific handling improves processing efficiency.

## Debug Mode

When using the `--debug` flag, the application will create a detailed log file with information about:

* Image processing workflow
* Hash computation details
* Errors and their causes
* Processing statistics for different file types
* Search matches and near-matches

Example debug log output:

```
2025/03/15 10:15:23 --- ImageFinder Debug Log Started at 2025-03-15T10:15:23Z ---
2025/03/15 10:15:23 Starting image scan on folder: /home/user/photos
2025/03/15 10:15:23 Force rewrite: false, Source prefix: myCollection
2025/03/15 10:15:24 Found 1283 image files to process (123 RAW files, 45 TIF files)
2025/03/15 10:15:25 PROCESSED: /home/user/photos/img001.jpg
2025/03/15 10:15:25 FAILED: /home/user/photos/corrupted.jpg - Error: failed to load image
2025/03/15 10:15:25 PROCESSED: /home/user/photos/img002.jpg
...
2025/03/15 10:20:45 Scan completed in 5m22s. Processed: 1283, Errors: 7, RAW files: 123, RAW errors: 2
```

## Project Structure

The application is organized into several packages:

* `main.go`: Entry point and command handling
* `database/`: Database operations and schema management
* `imageprocessor/`: Image loading, hashing, and comparison
* `scanner/`: Directory traversal and processing
* `logging/`: Debug and error logging
* `types/`: Shared data structures
* `utils/`: Utility functions for argument parsing, etc.

## License

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request