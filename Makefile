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

# Package webserver as macOS application
package-webserver-macos:
	@echo "Building webserver for Apple Silicon..."
	@mkdir -p $(DIST_DIR)/macos-arm64
	@GOOS=darwin GOARCH=arm64 CGO_ENABLED=1 $(GOBUILD) $(LDFLAGS) -o $(DIST_DIR)/macos-arm64/goimagefinder-webserver ./cmd/webserver/
	@echo "Packaging GoImageFinder Web application..."
	@mkdir -p $(DIST_DIR)/GoImageFinder.app/Contents/MacOS
	@mkdir -p $(DIST_DIR)/GoImageFinder.app/Contents/Resources
	@cp $(DIST_DIR)/macos-arm64/goimagefinder-webserver $(DIST_DIR)/GoImageFinder.app/Contents/MacOS/
	@cp ./resources/launcher.sh $(DIST_DIR)/GoImageFinder.app/Contents/MacOS/
	@chmod +x $(DIST_DIR)/GoImageFinder.app/Contents/MacOS/launcher.sh

	@# Copy icon if exists
	@if [ -f ./resources/AppIcon.icns ]; then \
		cp ./resources/AppIcon.icns $(DIST_DIR)/GoImageFinder.app/Contents/Resources/; \
	else \
		echo "No icon file found at ./resources/AppIcon.icns"; \
		touch $(DIST_DIR)/GoImageFinder.app/Contents/Resources/AppIcon.icns; \
	fi

	@echo "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n\
<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n\
<plist version=\"1.0\">\n\
<dict>\n\
\t<key>CFBundleExecutable</key>\n\
\t<string>launcher.sh</string>\n\
\t<key>CFBundleIdentifier</key>\n\
\t<string>com.goimagefinder.webserver</string>\n\
\t<key>CFBundleName</key>\n\
\t<string>GoImageFinder</string>\n\
\t<key>CFBundleDisplayName</key>\n\
\t<string>GoImageFinder</string>\n\
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
\t<key>LSUIElement</key>\n\
\t<false/>\n\
</dict>\n\
</plist>" > $(DIST_DIR)/GoImageFinder.app/Contents/Info.plist
	@echo "Application package created: $(DIST_DIR)/GoImageFinder.app"

# Create webserver DMG for distribution
create-webserver-dmg: package-webserver-macos
	@echo "Creating DMG for GoImageFinder Web..."
	@if ! command -v create-dmg > /dev/null; then \
		echo "create-dmg tool not found, installing via Homebrew..."; \
		brew install create-dmg || { echo "Error: Failed to install create-dmg. Please install manually."; exit 1; }; \
	fi
	@rm -f "$(DIST_DIR)/GoImageFinder.dmg"
	create-dmg --volname "GoImageFinder Installer" \
		--window-pos 200 120 --window-size 800 400 --icon-size 100 --icon "GoImageFinder.app" 200 190 \
		--hide-extension "GoImageFinder.app" --app-drop-link 600 185 \
		"$(DIST_DIR)/GoImageFinder.dmg" "$(DIST_DIR)/GoImageFinder.app"
	@echo "DMG created: $(DIST_DIR)/GoImageFinder.dmg"

# Create a DMG for distribution (requires create-dmg tool)
create-dmg: package-macos
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

# Build and run web server
webserver:
	@echo "Building web server..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/webserver ./cmd/webserver/
	@echo "Starting web server on port 8012..."
	@$(BUILD_DIR)/webserver

# Build web server only
build-webserver:
	@echo "Building web server..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/webserver ./cmd/webserver/
	@echo "Build complete! Binary: $(BUILD_DIR)/webserver"

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

# Docker targets
docker-build:
	@echo "Building Docker image..."
	docker build -t goimagefinder:latest .
	@echo "Docker image built: goimagefinder:latest"

docker-run:
	@echo "Running Docker container..."
	docker run -d --name goimagefinder \
		-p 8012:8012 \
		-v goimagefinder_data:/data \
		goimagefinder:latest
	@echo "Container started. Access at http://localhost:8012"

docker-stop:
	@echo "Stopping Docker container..."
	docker stop goimagefinder || true
	docker rm goimagefinder || true
	@echo "Container stopped"

docker-compose-up:
	@echo "Starting with docker-compose..."
	docker-compose up -d --build
	@echo "Started. Access at http://localhost:8012"

docker-compose-down:
	@echo "Stopping docker-compose services..."
	docker-compose down
	@echo "Stopped"

# Help target
help:
	@echo "Available targets:"
	@echo "  build                  - Build the application for current platform"
	@echo "  build-macos-arm64      - Build for macOS ARM64 (Apple Silicon)"
	@echo "  package-macos          - Create a macOS .app package (ARM64-only)"
	@echo "  create-dmg             - Create a distributable DMG file"
	@echo "  package-webserver-macos - Create a macOS .app for the web interface"
	@echo "  create-webserver-dmg   - Create a DMG for the web interface"
	@echo "  build-all              - Install dependencies, tools, and build the application"
	@echo "  webserver              - Build and run the web interface on port 8012"
	@echo "  build-webserver        - Build the web server binary only"
	@echo "  docker-build           - Build Docker image"
	@echo "  docker-run             - Run Docker container"
	@echo "  clean                  - Remove build artifacts"
	@echo "  test                   - Run tests"
	@echo "  deps                   - Install Go dependencies"
	@echo "  install-tools          - Install external tools for RAW image processing"
	@echo "  run-debug-scan         - Run the scan command with debug enabled"
	@echo "  run-debug-search       - Run the search command with debug enabled"
	@echo "  init                   - Initialize the Go module (run once)"
	@echo "  setup                  - Create project directory structure"
	@echo "  help                   - Show this help message"