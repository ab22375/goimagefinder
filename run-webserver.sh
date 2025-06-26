#!/bin/bash

# Build and run the GoImageFinder web server

echo "Building GoImageFinder web server..."
go build -o webserver cmd/webserver/main.go

if [ $? -eq 0 ]; then
    echo "Starting server on port 8012..."
    ./webserver
else
    echo "Build failed!"
    exit 1
fi