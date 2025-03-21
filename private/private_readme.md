
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

```bash
make clean
make build-macos-arm64
make package-macos
make create-dmg

sudo rm -f /usr/local/bin/goimagefinder
echo '#!/bin/bash' | sudo tee /usr/local/bin/goimagefinder > /dev/null
echo 'exec /Applications/goimagefinder.app/Contents/MacOS/goimagefinder "$@"' | sudo tee -a /usr/local/bin/goimagefinder > /dev/null
sudo chmod +x /usr/local/bin/goimagefinder
```

Then you can use 

goimagefinder scan ...
goimagefinder search ...


```
make clean
make build-macos-arm64
./dist/macos-arm64/goimagefinder
```
# after changing packages
go build ./...

F="/Users/z/Downloads/_vittorio/images/RAW/tif"
L="/Users/z/Downloads/_vittorio/databases/250319_1849/images.log"
D="/Users/z/Downloads/_vittorio/databases/250319_1849/images.db"
P="macab"

go run ./main.go scan ... 
goimagefinder scan ...

--folder=$F --database=$IMAGE_DATABASE --prefix=$P --debug --logfile=$IMAGE_LOGFILE --force


I="/Users/z/Downloads/_vittorio/images/RAW/tif/79330_SFA_001_01.tif"

go run ./main.go search ...
goimagefinder search ...

--database=$IMAGE_DATABASE --image=$I
