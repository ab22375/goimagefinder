# Refactoring Plan for GoImageFinder

This plan addresses the duplication between the root-level `imageprocessor` package and the `scanner/imageprocessor` package.

## Current Issues

1. **Duplication**: Two separate `imageprocessor` packages with overlapping functionality
2. **Inconsistent Interfaces**: Similar but slightly different interfaces for image loading
3. **Scattered Logic**: Image format detection logic duplicated across packages
4. **Dependency Confusion**: Scanner's imageprocessor imports the root imageprocessor, but also reimplements parts

## Refactoring Strategy

### 1. Consolidate Core Functionality (root `imageprocessor`)

- Make the root `imageprocessor` package the single source of truth for:
  - Image loading interfaces and registry
  - Hash computation and comparison
  - Format detection utilities
  - Image processing primitives

### 2. Create a Scanner-Specific Adapter (new `scanner/processor`)

- Rename `scanner/imageprocessor` to `scanner/processor` 
- Convert to a lightweight adapter/facade that uses core functionality
- Remove duplicated code and fully delegate to root package
- Maintain scanner-specific logic for handling progress & concurrency

### 3. Standardize Interfaces

- Define consistent interfaces in the root package
- Use composition over inheritance for specialized implementations
- Create clear extension points for format-specific handlers

## Implementation Tasks

1. **Core Consolidation**:
   - Move all image format detection to root `imageprocessor/formats.go`
   - Consolidate image loading interfaces in root package
   - Centralize utility functions in one place

2. **Interface Standardization**:
   - Create a unified `ImageLoader` interface in root package
   - Ensure all format-specific loaders use consistent interfaces
   - Add extension methods for specialized formats

3. **Scanner Adaptation**:
   - Refactor `scanner/imageprocessor` to `scanner/processor`
   - Update all imports in scanner-related files
   - Convert implementation to delegate to root package

4. **Testing**:
   - Create tests for core functionality 
   - Verify behavior consistency after refactoring

## Benefits

- **Reduced Duplication**: Single source of truth for core functionality
- **Clearer Responsibilities**: Well-defined boundaries between packages
- **Better Maintainability**: Easier to add new image formats
- **Improved Clarity**: Clear separation between core logic and scanner-specific concerns
- **Performance**: Potential performance improvements by eliminating redundant operations