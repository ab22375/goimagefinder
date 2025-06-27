#!/bin/bash

# GoImageFinder Test Runner Script
# This script runs all tests and generates coverage reports

set -e

echo "=== GoImageFinder Test Suite ==="
echo

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Determine if we're in the tests directory or project root
if [ -d "tests" ]; then
    # We're in project root
    TEST_PREFIX="./tests"
else
    # We're already in tests directory
    TEST_PREFIX="."
fi

# Test categories
declare -a TEST_PACKAGES=(
    "$TEST_PREFIX/database"
    "$TEST_PREFIX/imageprocessor"
    "$TEST_PREFIX/scanner"
    "$TEST_PREFIX/webserver"
    "$TEST_PREFIX/integration"
)

# Function to run tests for a package
run_tests() {
    local package=$1
    local name=$(basename $package)
    
    echo -e "${YELLOW}Running $name tests...${NC}"
    
    if go test -v -cover -coverprofile=coverage_$name.out $package; then
        echo -e "${GREEN}✓ $name tests passed${NC}"
        return 0
    else
        echo -e "${RED}✗ $name tests failed${NC}"
        return 1
    fi
}

# Function to run benchmarks
run_benchmarks() {
    echo -e "${YELLOW}Running benchmarks...${NC}"
    go test -bench=. -benchmem $TEST_PREFIX/integration
}

# Main test execution
FAILED=0
COVERAGE_FILES=""

# Run unit tests
echo "Running unit tests..."
echo

for package in "${TEST_PACKAGES[@]}"; do
    if run_tests $package; then
        name=$(basename $package)
        COVERAGE_FILES="$COVERAGE_FILES coverage_$name.out"
    else
        FAILED=$((FAILED + 1))
    fi
    echo
done

# Generate combined coverage report
if [ -n "$COVERAGE_FILES" ]; then
    echo -e "${YELLOW}Generating coverage report...${NC}"
    
    # Combine coverage files
    echo "mode: set" > coverage_combined.out
    for file in $COVERAGE_FILES; do
        if [ -f $file ]; then
            tail -n +2 $file >> coverage_combined.out
        fi
    done
    
    # Generate HTML report
    go tool cover -html=coverage_combined.out -o coverage.html
    echo -e "${GREEN}Coverage report generated: coverage.html${NC}"
    
    # Show coverage summary
    echo
    echo "Coverage Summary:"
    go tool cover -func=coverage_combined.out | grep "total:" || echo "No coverage data"
fi

# Run benchmarks if requested
if [ "$1" = "--bench" ]; then
    echo
    run_benchmarks
fi

# Clean up individual coverage files
rm -f coverage_*.out

# Summary
echo
echo "=== Test Summary ==="
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}$FAILED test suite(s) failed${NC}"
    exit 1
fi