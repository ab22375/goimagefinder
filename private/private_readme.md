
```sh
# Install Go dependencies
make deps

# Install required external tools for RAW image processing
make install-tools

# Clean previous build artifacts
make clean

# Build for ARM64 only
make build-macos-arm64

# Package as an app
make package-macos

make create-dmg
```