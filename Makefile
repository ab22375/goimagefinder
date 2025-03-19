.PHONY: build clean test run build-macos build-universal build-macos-arm64

# Application name
APP_NAME := goimagefinder

# Build directory
BUILD_DIR := ./build

# Distribution directory
DIST_DIR := ./dist

# Go commands
GO := go
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOTEST := $(GO) test
GOGET := $(GO) get

# Build flags
LDFLAGS := -ldflags="-s -w"

# Module name (use the correct module name from go.mod)
MODULE_NAME := github.com/yourusername/imagefinder

# Build the application 
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME) ./main.go
	@echo "Build complete! Binary: $(BUILD_DIR)/$(APP_NAME)"

# Build specifically for macOS ARM64 (Apple Silicon)
build-macos-arm64:
	@echo "Building for macOS ARM64 (Apple Silicon)..."
	@mkdir -p $(DIST_DIR)/macos-arm64
	@GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/macos-arm64/$(APP_NAME) ./main.go
	@echo "Build complete! Binary: $(DIST_DIR)/macos-arm64/$(APP_NAME)"

# Build specifically for macOS AMD64 (Intel) 
# Note: This target may fail if gocv has architecture-specific compilation issues
build-macos-amd64:
	@echo "Building for macOS AMD64 (Intel)..."
	@mkdir -p $(DIST_DIR)/macos-amd64
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/macos-amd64/$(APP_NAME) ./main.go || \
	(echo "Error: Failed to build for Intel Macs. This may be due to gocv architecture compatibility issues." && exit 1)
	@echo "Build complete! Binary: $(DIST_DIR)/macos-amd64/$(APP_NAME)"

# Build universal macOS binary (works on both Intel and Apple Silicon)
# Fallback to just ARM64 if AMD64 build fails
build-macos-universal:
	@echo "Building for macOS ARM64 (Apple Silicon)..."
	@mkdir -p $(DIST_DIR)/macos-arm64
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/macos-arm64/$(APP_NAME) ./main.go
	
	@echo "Attempting to build for macOS AMD64 (Intel)..."
	@mkdir -p $(DIST_DIR)/macos-amd64
	@GOOS=darwin GOARCH=amd64 CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/macos-amd64/$(APP_NAME) ./main.go; \
	if [ $? -eq 0 ]; then \
		echo "Creating universal binary..."; \
		mkdir -p $(DIST_DIR)/macos-universal; \
		lipo -create -output $(DIST_DIR)/macos-universal/$(APP_NAME) \
			$(DIST_DIR)/macos-arm64/$(APP_NAME) $(DIST_DIR)/macos-amd64/$(APP_NAME); \
		echo "Universal binary created: $(DIST_DIR)/macos-universal/$(APP_NAME)"; \
	else \
		echo "Warning: AMD64 build failed. Creating ARM64-only binary instead."; \
		mkdir -p $(DIST_DIR)/macos-universal; \
		cp $(DIST_DIR)/macos-arm64/$(APP_NAME) $(DIST_DIR)/macos-universal/$(APP_NAME); \
		echo "ARM64-only binary copied to: $(DIST_DIR)/macos-universal/$(APP_NAME)"; \
	fi

# Package macOS application (will use ARM64-only binary if universal build fails)
package-macos:
	@echo "Building for Apple Silicon before packaging..."
	@mkdir -p $(DIST_DIR)/macos-arm64
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/macos-arm64/$(APP_NAME) ./main.go
	@mkdir -p $(DIST_DIR)/macos-universal
	@cp $(DIST_DIR)/macos-arm64/$(APP_NAME) $(DIST_DIR)/macos-universal/$(APP_NAME)
	@echo "Using ARM64 binary for packaging..."
	@echo "Packaging macOS application..."
	@mkdir -p $(DIST_DIR)/$(APP_NAME).app/Contents/MacOS
	@mkdir -p $(DIST_DIR)/$(APP_NAME).app/Contents/Resources
	@cp $(DIST_DIR)/macos-universal/$(APP_NAME) $(DIST_DIR)/$(APP_NAME).app/Contents/MacOS/
	
	@# Create a placeholder icon if none exists
	@if [ ! -f ./resources/AppIcon.icns ]; then \
		echo "No icon file found, creating a placeholder..."; \
		mkdir -p ./resources; \
		touch $(DIST_DIR)/$(APP_NAME).app/Contents/Resources/AppIcon.icns; \
	else \
		cp ./resources/AppIcon.icns $(DIST_DIR)/$(APP_NAME).app/Contents/Resources/; \
	fi
	
	@echo "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n\
<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n\
<plist version=\"1.0\">\n\
<dict>\n\
\t<key>CFBundleExecutable</key>\n\
\t<string>$(APP_NAME)</string>\n\
\t<key>CFBundleIdentifier</key>\n\
\t<string>com.yourdomain.$(APP_NAME)</string>\n\
\t<key>CFBundleName</key>\n\
\t<string>$(APP_NAME)</string>\n\
\t<key>CFBundleIconFile</key>\n\
\t<string>AppIcon</string>\n\
\t<key>CFBundleShortVersionString</key>\n\
\t<string>1.0</string>\n\
\t<key>CFBundleInfoDictionaryVersion</key>\n\
\t<string>6.0</string>\n\
\t<key>CFBundlePackageType</key>\n\
\t<string>APPL</string>\n\
\t<key>CFBundleVersion</key>\n\
\t<string>1</string>\n\
\t<key>NSHighResolutionCapable</key>\n\
\t<true/>\n\
</dict>\n\
</plist>" > $(DIST_DIR)/$(APP_NAME).app/Contents/Info.plist
	@echo "Application package created: $(DIST_DIR)/$(APP_NAME).app"
	@mkdir -p $(DIST_DIR)/$(APP_NAME).app/Contents/MacOS
	@mkdir -p $(DIST_DIR)/$(APP_NAME).app/Contents/Resources
	@cp $(DIST_DIR)/macos-universal/$(APP_NAME) $(DIST_DIR)/$(APP_NAME).app/Contents/MacOS/
	@echo "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n\
<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n\
<plist version=\"1.0\">\n\
<dict>\n\
\t<key>CFBundleExecutable</key>\n\
\t<string>$(APP_NAME)</string>\n\
\t<key>CFBundleIdentifier</key>\n\
\t<string>com.yourdomain.$(APP_NAME)</string>\n\
\t<key>CFBundleName</key>\n\
\t<string>$(APP_NAME)</string>\n\
\t<key>CFBundleIconFile</key>\n\
\t<string>AppIcon</string>\n\
\t<key>CFBundleShortVersionString</key>\n\
\t<string>1.0</string>\n\
\t<key>CFBundleInfoDictionaryVersion</key>\n\
\t<string>6.0</string>\n\
\t<key>CFBundlePackageType</key>\n\
\t<string>APPL</string>\n\
\t<key>CFBundleVersion</key>\n\
\t<string>1</string>\n\
\t<key>NSHighResolutionCapable</key>\n\
\t<true/>\n\
</dict>\n\
</plist>" > $(DIST_DIR)/$(APP_NAME).app/Contents/Info.plist
	@echo "Application package created: $(DIST_DIR)/$(APP_NAME).app"

# Create a DMG for distribution (requires create-dmg tool)
create-dmg: package-macos
	@echo "Creating DMG for distribution..."
	@if ! command -v create-dmg > /dev/null; then \
		echo "create-dmg tool not found, installing via Homebrew..."; \
		brew install create-dmg || { echo "Error: Failed to install create-dmg. Please install manually."; exit 1; }; \
	fi
	@# Check if icon exists before using it in create-dmg command
	@if [ -s "$(DIST_DIR)/$(APP_NAME).app/Contents/Resources/AppIcon.icns" ]; then \
		create-dmg --volname "$(APP_NAME) Installer" --volicon "$(DIST_DIR)/$(APP_NAME).app/Contents/Resources/AppIcon.icns" \
			--window-pos 200 120 --window-size 800 400 --icon-size 100 --icon "$(APP_NAME).app" 200 190 \
			--hide-extension "$(APP_NAME).app" --app-drop-link 600 185 \
			"$(DIST_DIR)/$(APP_NAME).dmg" "$(DIST_DIR)/$(APP_NAME).app"; \
	else \
		echo "Creating DMG without custom icon..."; \
		create-dmg --volname "$(APP_NAME) Installer" \
			--window-pos 200 120 --window-size 800 400 --icon-size 100 --icon "$(APP_NAME).app" 200 190 \
			--hide-extension "$(APP_NAME).app" --app-drop-link 600 185 \
			"$(DIST_DIR)/$(APP_NAME).dmg" "$(DIST_DIR)/$(APP_NAME).app"; \
	fi
	@echo "DMG created: $(DIST_DIR)/$(APP_NAME).dmg"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR) $(DIST_DIR)
	@echo "Clean complete"

# Run tests
test:
	@echo "Running tests..."
	@$(GOTEST) ./...
	@echo "Tests complete"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@$(GOGET) -u gocv.io/x/gocv
	@$(GOGET) -u github.com/mattn/go-sqlite3
	@echo "Dependencies installed"

# Run the application with debug mode enabled
run-debug-scan:
	@echo "Running in debug mode..."
	@$(GO) run main.go scan --folder=./test_images --debug

# Run the application with debug mode enabled for search
run-debug-search:
	@echo "Running search in debug mode..."
	@$(GO) run main.go search --image=./test_images/sample.jpg --debug

# Initialize the module (run once at the beginning)
init:
	@echo "Initializing Go module..."
	@$(GO) mod init $(MODULE_NAME)
	@echo "Module initialized: $(MODULE_NAME)"

# Create the project directory structure
setup:
	@echo "Setting up project structure..."
	@mkdir -p database imageprocessor logging scanner types utils
	@echo "Project structure created"

# Install required external tools for RAW image processing
install-tools:
	@echo "Installing external tools for RAW image processing..."
	@if [ "$(shell uname)" = "Darwin" ]; then \
		echo "Detected macOS, using Homebrew..."; \
		brew install dcraw exiftool libraw rawtherapee || echo "Error installing tools with Homebrew. Please install manually."; \
	elif [ -f /etc/debian_version ]; then \
		echo "Detected Debian/Ubuntu, using apt..."; \
		sudo apt-get update && sudo apt-get install -y dcraw exiftool libraw-bin rawtherapee || echo "Error installing tools with apt. Please install manually."; \
	elif [ -f /etc/redhat-release ]; then \
		echo "Detected RHEL/CentOS/Fedora, using dnf/yum..."; \
		sudo dnf install -y dcraw perl-Image-ExifTool libraw rawtherapee || sudo yum install -y dcraw perl-Image-ExifTool libraw rawtherapee || echo "Error installing tools. Please install manually."; \
	else \
		echo "Unsupported OS. Please install these tools manually:"; \
		echo "- dcraw (for RAW image conversion)"; \
		echo "- exiftool (for extracting image metadata)"; \
		echo "- libraw (for processing RAW images)"; \
		echo "- rawtherapee (optional, for alternative RAW processing)"; \
	fi
	@echo "External tools installation complete or already installed."

# Build with all dependencies
build-all: deps install-tools build
	@echo "Complete build with all dependencies finished!"

# Help target
help:
	@echo "Available targets:"
	@echo "  build                - Build the application for current platform"
	@echo "  build-macos-arm64    - Build for macOS ARM64 (Apple Silicon)"
	@echo "  build-macos-amd64    - Build for macOS AMD64 (Intel) - may fail with gocv"
	@echo "  build-macos-universal - Build universal binary (falls back to ARM64-only if Intel build fails)"
	@echo "  package-macos        - Create a macOS .app package (ARM64-only)"
	@echo "  create-dmg           - Create a distributable DMG file"
	@echo "  build-all            - Install dependencies, tools, and build the application"
	@echo "  clean                - Remove build artifacts"
	@echo "  test                 - Run tests"
	@echo "  deps                 - Install Go dependencies"
	@echo "  install-tools        - Install external tools for RAW image processing"
	@echo "  run-debug-scan       - Run the scan command with debug enabled"
	@echo "  run-debug-search     - Run the search command with debug enabled"
	@echo "  init                 - Initialize the Go module (run once)"
	@echo "  setup                - Create project directory structure"
	@echo "  help                 - Show this help message"