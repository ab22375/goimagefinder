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
### 1. Scan and Index Images
To scan a folder and store image data in a database:

```sh
./image-indexer scan --folder=/path/to/images --database=/path/to/images.db --prefix=Camera1 --force
```

- `--folder`: Path to the folder containing images to scan.
- `--database`: Path to the SQLite database file (default: `images.db`).
- `--prefix`: (Optional) Label to differentiate multiple sources.
- `--force`: (Optional) Rewrites existing entries instead of skipping unchanged files.

### 2. Search for Similar Images
To search for similar images given a query image:

```sh
./image-indexer search --image=/path/to/query.jpg --database=/path/to/images.db --threshold=0.85
```

- `--image`: Path to the query image.
- `--database`: Path to the SQLite database file.
- `--threshold`: Similarity threshold (default: 0.8, range: 0.0 to 1.0).
- `--prefix`: (Optional) Filter results by a specific source prefix.

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

## License
This project is licensed under the **MIT License**.

## Contact
For issues or feature requests, open an issue on GitHub or contact the maintainer at **your-email@example.com**.

