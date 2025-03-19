
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

# after changing packages
go build ./...


F="/Users/z/Downloads/_vittorio/images/RAW/tif"
L="/Users/z/Downloads/_vittorio/databases/250319_1849/images.log"
D="/Users/z/Downloads/_vittorio/databases/250319_1849/images.db"
P="macab"

go run ./main.go scan --folder=$F --database=$D --prefix=$P --debug --logfile=$L --force

I="/Users/z/Downloads/_vittorio/images/RAW/tif/79330_SFA_001_01.tif"

go run ./main.go search --database=$D --debug --logfile=$L --image=$I
