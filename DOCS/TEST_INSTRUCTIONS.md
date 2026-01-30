# Image Matching Test Instructions

This document describes how to test the image matching functionality using lower-resolution samples.

## Setup

```bash
cd /Users/z/dev/golang/goimagefinder

ORIGINALS_FOLDER="/Users/z/Downloads/VB/TESTS/new_F100"
LOWRES_FOLDER="/Users/z/Downloads/VB/TESTS/lowres_samples"
DB="/Users/z/Downloads/VB/TESTS/test_originals.db"
```

## 1. Create Lower-Resolution Samples

Create test samples from original images at different resolutions and quality levels:

```bash
cd "$ORIGINALS_FOLDER"

# Small (800px width, quality 60)
magick 000058370030.jpg -resize 800x -quality 60 "$LOWRES_FOLDER/000058370030_small.jpg"
magick 000058350016.jpg -resize 800x -quality 60 "$LOWRES_FOLDER/000058350016_small.jpg"

# Thumbnail (400px width, quality 50)
magick 000058370030.jpg -resize 400x -quality 50 "$LOWRES_FOLDER/000058370030_thumb.jpg"
magick 000058350016.jpg -resize 400x -quality 50 "$LOWRES_FOLDER/000058350016_thumb.jpg"
```

## 2. Scan the Originals Folder

Index all original images into the database:

```bash
cd /Users/z/dev/golang/goimagefinder
./build/goimagefinder scan --folder="$ORIGINALS_FOLDER" --database="$DB" --prefix="originals"
```

## 3. Verify Database Contents

Count total scanned images:

```bash
sqlite3 "$DB" "SELECT COUNT(*) FROM images;"
```

Check if a specific image was scanned:

```bash
sqlite3 "$DB" "SELECT id, path, average_hash, perceptual_hash FROM images WHERE path LIKE '%000058350025%';"
```

List all scanned images:

```bash
sqlite3 "$DB" "SELECT id, path FROM images ORDER BY id;"
```

## 4. Test Image Matching

Search for matches using lower-resolution samples:

```bash
cd /Users/z/dev/golang/goimagefinder

# Test with thumbnail
IMG="/Users/z/Downloads/VB/TESTS/lowres_samples/000058370030_thumb.jpg"
./build/goimagefinder search --image="$IMG" --database="$DB" --threshold=0.8 --debug

# Test with small version
IMG="/Users/z/Downloads/VB/TESTS/lowres_samples/000058350016_small.jpg"
./build/goimagefinder search --image="$IMG" --database="$DB" --threshold=0.8 --debug
```

### Threshold Guidelines

- `0.9` - Very strict, nearly identical images only
- `0.8` - Default, good for finding resized/recompressed versions
- `0.7` - More lenient, may include similar but different images
- `0.5` - Very lenient, useful for debugging to see all potential matches

## 5. Debugging

For detailed debug output showing hash comparisons and similarity scores:

```bash
./build/goimagefinder search --image="$IMG" --database="$DB" --threshold=0.5 --debug
```

Check the computed hashes for a query image manually:

```bash
# Compare query image hash with database entries
sqlite3 "$DB" "SELECT path, average_hash, perceptual_hash FROM images LIMIT 10;"
```

## 6. Clean Up

Remove test database and samples:

```bash
rm -f "$DB"
rm -rf "$LOWRES_FOLDER"/*
```

## Expected Results

When searching with a lower-resolution sample, the original image should appear in the results with a high similarity score (typically > 0.85 for resized versions of the same image).
