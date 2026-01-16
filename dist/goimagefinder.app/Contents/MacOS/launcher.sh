#!/bin/bash
#
# GoImageFinder Launcher Script
# This script is used by the macOS .app bundle to launch the webserver
#

# Get the directory where this script is located
DIR="$(cd "$(dirname "$0")" && pwd)"

# Config file location
CONFIG_FILE="$HOME/.goimagefinder/webserver.json"

# Default port
PORT=8012

# Read port from config if it exists
if [ -f "$CONFIG_FILE" ]; then
    # Extract port using grep and sed (works without jq)
    CONFIG_PORT=$(grep -o '"port"[[:space:]]*:[[:space:]]*[0-9]*' "$CONFIG_FILE" | grep -o '[0-9]*')
    if [ -n "$CONFIG_PORT" ]; then
        PORT=$CONFIG_PORT
    fi
fi

# Log file
LOG_DIR="$HOME/.goimagefinder/logs"
mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/webserver.log"

echo "$(date): Starting GoImageFinder on port $PORT" >> "$LOG_FILE"

# Start the webserver
"$DIR/goimagefinder-webserver" "$PORT" >> "$LOG_FILE" 2>&1 &
PID=$!

# Save PID for later cleanup
echo $PID > "$HOME/.goimagefinder/webserver.pid"

# Wait for the server to be ready (max 30 seconds)
echo "$(date): Waiting for server to start..." >> "$LOG_FILE"
for i in {1..60}; do
    if curl -s "http://localhost:$PORT" > /dev/null 2>&1; then
        echo "$(date): Server is ready, opening browser" >> "$LOG_FILE"
        open "http://localhost:$PORT"
        break
    fi
    sleep 0.5
done

# Keep the process running (for the app to stay in Dock)
wait $PID
