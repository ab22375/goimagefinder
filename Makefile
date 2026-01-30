.PHONY: build clean test run build-macos build-universal build-macos-arm64 \
	build-linux build-windows build-cross docker docker-build docker-push

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

# Package macOS application (will use ARM64-only binary)
package-macos:
	@echo "Building for Apple Silicon before packaging..."
	@mkdir -p $(DIST_DIR)/macos-arm64
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/macos-arm64/$(APP_NAME) ./main.go
	@echo "Using ARM64 binary for packaging..."
	@echo "Packaging macOS application..."
	@mkdir -p $(DIST_DIR)/$(APP_NAME).app/Contents/MacOS
	@mkdir -p $(DIST_DIR)/$(APP_NAME).app/Contents/Resources
	@cp $(DIST_DIR)/macos-arm64/$(APP_NAME) $(DIST_DIR)/$(APP_NAME).app/Contents/MacOS/
	
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

# Package macOS application with bundled libraries (for distribution)
package-macos-dist:
	@echo "Building distributable macOS application with bundled libraries..."
	@mkdir -p $(DIST_DIR)/macos-arm64
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/macos-arm64/$(APP_NAME) ./main.go
	@echo "Creating application bundle..."
	@rm -rf $(DIST_DIR)/$(APP_NAME).app
	@mkdir -p $(DIST_DIR)/$(APP_NAME).app/Contents/MacOS
	@mkdir -p $(DIST_DIR)/$(APP_NAME).app/Contents/Resources
	@mkdir -p $(DIST_DIR)/$(APP_NAME).app/Contents/Frameworks
	@cp $(DIST_DIR)/macos-arm64/$(APP_NAME) $(DIST_DIR)/$(APP_NAME).app/Contents/MacOS/
	@# Copy icon if exists
	@if [ -f ./resources/AppIcon.icns ]; then \
		cp ./resources/AppIcon.icns $(DIST_DIR)/$(APP_NAME).app/Contents/Resources/; \
	else \
		touch $(DIST_DIR)/$(APP_NAME).app/Contents/Resources/AppIcon.icns; \
	fi
	@# Create Info.plist
	@echo "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n\
<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n\
<plist version=\"1.0\">\n\
<dict>\n\
\t<key>CFBundleExecutable</key>\n\
\t<string>$(APP_NAME)</string>\n\
\t<key>CFBundleIdentifier</key>\n\
\t<string>com.goimagefinder.cli</string>\n\
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
	@echo "Bundling dynamic libraries..."
	@bash ./scripts/bundle-dylibs.sh $(DIST_DIR)/$(APP_NAME).app/Contents/MacOS/$(APP_NAME) $(DIST_DIR)/$(APP_NAME).app/Contents/Frameworks
	@echo "Application package created: $(DIST_DIR)/$(APP_NAME).app"

# Create a DMG for distribution (requires create-dmg tool)
create-dmg: package-macos-dist
	@echo "Creating DMG for distribution..."
	@if ! command -v create-dmg > /dev/null; then \
		echo "create-dmg tool not found, installing via Homebrew..."; \
		brew install create-dmg || { echo "Error: Failed to install create-dmg. Please install manually."; exit 1; }; \
	fi
	create-dmg --volname "$(APP_NAME) Installer" \
		--window-pos 200 120 --window-size 800 400 --icon-size 100 --icon "$(APP_NAME).app" 200 190 \
		--hide-extension "$(APP_NAME).app" --app-drop-link 600 185 \
		"$(DIST_DIR)/$(APP_NAME).dmg" "$(DIST_DIR)/$(APP_NAME).app"
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
	@$(GOTEST) -v ./tests/...
	@echo "Tests complete"

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	@$(GOTEST) -v -cover ./tests/... -coverprofile=coverage.out
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run benchmarks
test-bench:
	@echo "Running benchmarks..."
	@$(GOTEST) -bench=. -benchmem ./tests/integration

# Run all tests using test runner script
test-all:
	@cd tests && ./run_tests.sh

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

# Cross-platform builds
build-linux-amd64:
	@echo "Building for Linux AMD64..."
	@mkdir -p $(DIST_DIR)/linux-amd64
	@GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/linux-amd64/$(APP_NAME) ./main.go
	@echo "Build complete! Binary: $(DIST_DIR)/linux-amd64/$(APP_NAME)"

build-linux-arm64:
	@echo "Building for Linux ARM64..."
	@mkdir -p $(DIST_DIR)/linux-arm64
	@GOOS=linux GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/linux-arm64/$(APP_NAME) ./main.go
	@echo "Build complete! Binary: $(DIST_DIR)/linux-arm64/$(APP_NAME)"

build-windows-amd64:
	@echo "Building for Windows AMD64..."
	@mkdir -p $(DIST_DIR)/windows-amd64
	@GOOS=windows GOARCH=amd64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/windows-amd64/$(APP_NAME).exe ./main.go
	@echo "Build complete! Binary: $(DIST_DIR)/windows-amd64/$(APP_NAME).exe"

build-windows-arm64:
	@echo "Building for Windows ARM64..."
	@mkdir -p $(DIST_DIR)/windows-arm64
	@GOOS=windows GOARCH=arm64 CGO_ENABLED=0 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/windows-arm64/$(APP_NAME).exe ./main.go
	@echo "Build complete! Binary: $(DIST_DIR)/windows-arm64/$(APP_NAME).exe"

# Build all cross-platform binaries
build-cross: build-linux-amd64 build-linux-arm64 build-macos-arm64 build-windows-amd64 build-windows-arm64
	@echo "All cross-platform builds complete!"
	@ls -la $(DIST_DIR)/*/$(APP_NAME)* 2>/dev/null || true

# Docker targets
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(APP_NAME):latest .
	@echo "Docker image built: $(APP_NAME):latest"

docker-build-multiarch:
	@echo "Building multi-architecture Docker images..."
	@./scripts/build-docker.sh all

docker-run:
	@echo "Running Docker container..."
	@docker run --rm -v ./images:/data/images:ro -v ./db:/data/db $(APP_NAME):latest info

docker-compose-up:
	@docker-compose up -d

docker-compose-down:
	@docker-compose down

# Help target
help:
	@echo "Available targets:"
	@echo ""
	@echo "Build targets:"
	@echo "  build                  - Build the application for current platform"
	@echo "  build-macos-arm64      - Build for macOS ARM64 (Apple Silicon)"
	@echo "  build-linux-amd64      - Build for Linux x86_64"
	@echo "  build-linux-arm64      - Build for Linux ARM64"
	@echo "  build-windows-amd64    - Build for Windows x86_64"
	@echo "  build-windows-arm64    - Build for Windows ARM64"
	@echo "  build-cross            - Build for all platforms"
	@echo "  build-all              - Install dependencies, tools, and build"
	@echo ""
	@echo "Package targets:"
	@echo "  package-macos          - Create a macOS .app package (development)"
	@echo "  package-macos-dist     - Create distributable .app with bundled libraries"
	@echo "  create-dmg             - Create a distributable DMG file"
	@echo ""
	@echo "Docker targets:"
	@echo "  docker-build           - Build Docker image for current platform"
	@echo "  docker-build-multiarch - Build multi-arch Docker images (amd64 + arm64)"
	@echo "  docker-run             - Run Docker container"
	@echo "  docker-compose-up      - Start services with docker-compose"
	@echo "  docker-compose-down    - Stop docker-compose services"
	@echo ""
	@echo "Test targets:"
	@echo "  test                   - Run tests"
	@echo "  test-coverage          - Run tests with coverage report"
	@echo "  test-bench             - Run benchmarks"
	@echo ""
	@echo "Other targets:"
	@echo "  clean                  - Remove build artifacts"
	@echo "  deps                   - Install Go dependencies"
	@echo "  install-tools          - Install external tools for RAW image processing"
	@echo "  run-debug-scan         - Run the scan command with debug enabled"
	@echo "  run-debug-search       - Run the search command with debug enabled"
	@echo "  help                   - Show this help message"