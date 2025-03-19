.PHONY: build clean test run

# Application name
APP_NAME := imagefinder

# Build directory
BUILD_DIR := ./build

# Go commands
GO := go
GOBUILD := $(GO) build
GOCLEAN := $(GO) clean
GOTEST := $(GO) test
GOGET := $(GO) get

# Module name
MODULE_NAME := imagefinder

# Build the application
build:
	@echo "Building $(APP_NAME)..."
	@mkdir -p $(BUILD_DIR)
	@$(GOBUILD) -o $(BUILD_DIR)/$(APP_NAME) ./main.go
	@echo "Build complete! Binary: $(BUILD_DIR)/$(APP_NAME)"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@$(GOCLEAN)
	@rm -rf $(BUILD_DIR)
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
	@echo "  build          - Build the application"
	@echo "  build-all      - Install dependencies, tools, and build the application"
	@echo "  clean          - Remove build artifacts"
	@echo "  test           - Run tests"
	@echo "  deps           - Install Go dependencies"
	@echo "  install-tools  - Install external tools for RAW image processing"
	@echo "  run-debug-scan - Run the scan command with debug enabled"
	@echo "  run-debug-search - Run the search command with debug enabled"
	@echo "  init           - Initialize the Go module (run once)"
	@echo "  setup          - Create project directory structure"
	@echo "  help           - Show this help message"