# GoImageFinder Test Suite

Comprehensive test suite for the GoImageFinder application covering unit tests, integration tests, and benchmarks.

## Test Structure

```
tests/
├── database/          # Database package tests
├── imageprocessor/    # Image processing tests
├── scanner/           # Scanner package tests
├── webserver/         # Web server tests
├── integration/       # End-to-end integration tests
├── utils/            # Test utilities and helpers
├── testdata/         # Test data files (created at runtime)
└── run_tests.sh      # Test runner script
```

## Running Tests

### Run All Tests
```bash
cd tests
./run_tests.sh
```

### Run Tests with Benchmarks
```bash
./run_tests.sh --bench
```

### Run Specific Package Tests
```bash
go test -v ./tests/database
go test -v ./tests/imageprocessor
go test -v ./tests/scanner
go test -v ./tests/webserver
go test -v ./tests/integration
```

### Run with Coverage
```bash
go test -v -cover ./tests/...
```

### Generate Coverage Report
```bash
go test -coverprofile=coverage.out ./tests/...
go tool cover -html=coverage.out -o coverage.html
```

## Test Categories

### Unit Tests

#### Database Tests (`database/`)
- Database initialization and schema creation
- Image insertion and updates
- Duplicate handling
- Query operations
- Index verification

#### Image Processor Tests (`imageprocessor/`)
- Image loading from various formats
- Hash computation (average and perceptual)
- Metadata extraction
- Similarity scoring
- Hamming distance calculation

#### Scanner Tests (`scanner/`)
- Directory traversal
- Image file detection
- Concurrent processing
- Incremental scanning
- Source prefix handling

#### Web Server Tests (`webserver/`)
- HTTP endpoint structure
- Request/response formats
- File upload handling
- Server-sent events (SSE)
- Static file serving

### Integration Tests (`integration/`)
- Complete workflow testing
- Duplicate detection
- Cross-format similarity
- Incremental scanning
- Performance benchmarks

## Test Utilities (`utils/`)

### TestImageBuilder
Creates test images with various patterns:
- Solid colors
- Gradients
- Checkerboard patterns
- Noise patterns

### TestDatabaseHelper
Utilities for database testing:
- Test database creation
- Image insertion
- Database queries
- Cleanup operations

### TestFileSystem
File system testing utilities:
- Directory creation
- File creation
- Image gallery generation
- File existence assertions

## Writing New Tests

### Example Unit Test
```go
func TestNewFeature(t *testing.T) {
    // Setup
    db := utils.NewTestDatabase(t)
    defer db.Close()
    
    // Test logic
    result, err := YourFunction()
    if err != nil {
        t.Fatalf("Unexpected error: %v", err)
    }
    
    // Assertions
    if result != expected {
        t.Errorf("Expected %v, got %v", expected, result)
    }
}
```

### Example Integration Test
```go
func TestCompleteWorkflow(t *testing.T) {
    // Setup test environment
    fs := utils.NewTestFileSystem(t)
    gallery := fs.CreateImageGallery(t)
    
    // Run workflow
    // ... test complete operation
}
```

## Test Data

Test images are generated programmatically to ensure reproducibility. No external test data files are required.

## Performance Testing

Run benchmarks to measure performance:
```bash
go test -bench=. -benchmem ./tests/integration
```

## Continuous Integration

The test suite is designed to run in CI environments. All tests create temporary directories and clean up after themselves.

## Requirements

- Go 1.18+
- GoCV (OpenCV bindings)
- SQLite3
- Same dependencies as main application

## Troubleshooting

### Tests Failing
1. Ensure all dependencies are installed
2. Check GoCV is properly configured
3. Verify external tools (dcraw, exiftool) are available if testing RAW formats

### Coverage Issues
1. Run tests from project root
2. Ensure all packages are built
3. Check for compilation errors

## Future Improvements

- Mock external dependencies
- Add fuzz testing
- Expand RAW format testing
- Add stress tests for large datasets
- Web UI automation tests