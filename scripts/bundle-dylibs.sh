#!/usr/bin/env bash
#
# bundle-dylibs.sh - Bundle dynamic libraries into a macOS .app bundle
#
# Usage: ./bundle-dylibs.sh <binary_path> <frameworks_dir>
#
# This script:
# 1. Finds all non-system dynamic library dependencies
# 2. Copies them to the Frameworks directory
# 3. Recursively processes dependencies of those libraries
# 4. Rewrites all library paths to use @executable_path/../Frameworks/

set -e

BINARY="$1"
FRAMEWORKS_DIR="$2"

if [ -z "$BINARY" ] || [ -z "$FRAMEWORKS_DIR" ]; then
    echo "Usage: $0 <binary_path> <frameworks_dir>"
    exit 1
fi

if [ ! -f "$BINARY" ]; then
    echo "Error: Binary not found: $BINARY"
    exit 1
fi

mkdir -p "$FRAMEWORKS_DIR"

# Track processed libraries using a temp file (portable alternative to associative arrays)
PROCESSED_FILE=$(mktemp)
trap "rm -f $PROCESSED_FILE" EXIT

is_processed() {
    grep -q "^$1$" "$PROCESSED_FILE" 2>/dev/null
}

mark_processed() {
    echo "$1" >> "$PROCESSED_FILE"
}

# Get all non-system library dependencies
get_deps() {
    local file="$1"
    otool -L "$file" 2>/dev/null | tail -n +2 | awk '{print $1}' | \
        grep -v "^/usr/lib" | \
        grep -v "^/System" | \
        grep -v "@executable_path" | \
        grep -v "@loader_path" | \
        grep -v "@rpath" || true
}

# Process a library: copy and fix paths
process_lib() {
    local lib_path="$1"
    local lib_name=$(basename "$lib_path")

    # Skip if already processed
    if is_processed "$lib_name"; then
        return
    fi
    mark_processed "$lib_name"

    # Skip if library doesn't exist
    if [ ! -f "$lib_path" ]; then
        echo "  Warning: Library not found: $lib_path"
        return
    fi

    local dest="$FRAMEWORKS_DIR/$lib_name"

    # Copy if not already there
    if [ ! -f "$dest" ]; then
        echo "  Copying: $lib_name"
        cp "$lib_path" "$dest"
        chmod 644 "$dest"
    fi

    # Get dependencies of this library and process them
    local deps=$(get_deps "$lib_path")
    for dep in $deps; do
        process_lib "$dep"
    done
}

echo "Finding and copying dependencies..."

# Process the main binary's dependencies
DEPS=$(get_deps "$BINARY")
for dep in $DEPS; do
    process_lib "$dep"
done

# Second pass: find @rpath dependencies from the copied libraries themselves
echo "Scanning copied libraries for additional @rpath dependencies..."
for lib in "$FRAMEWORKS_DIR"/*.dylib; do
    if [ -f "$lib" ]; then
        RPATH_DEPS=$(otool -L "$lib" 2>/dev/null | tail -n +2 | awk '{print $1}' | grep "^@rpath" || true)
        for rpath_dep in $RPATH_DEPS; do
            dep_name=$(basename "$rpath_dep")
            # Try to find this library in common homebrew locations
            for search_path in /opt/homebrew/lib /opt/homebrew/opt/*/lib /usr/local/lib; do
                found_lib=$(find $search_path -name "$dep_name" 2>/dev/null | head -1)
                if [ -n "$found_lib" ] && [ -f "$found_lib" ]; then
                    if [ ! -f "$FRAMEWORKS_DIR/$dep_name" ]; then
                        echo "  Copying (from @rpath): $dep_name"
                        cp "$found_lib" "$FRAMEWORKS_DIR/$dep_name"
                        chmod 644 "$FRAMEWORKS_DIR/$dep_name"
                    fi
                    break
                fi
            done
        done
    fi
done

echo "Rewriting library paths..."

# Fix paths in all copied libraries
for lib in "$FRAMEWORKS_DIR"/*.dylib; do
    if [ -f "$lib" ]; then
        lib_name=$(basename "$lib")

        # Change the library's own ID
        install_name_tool -id "@executable_path/../Frameworks/$lib_name" "$lib" 2>/dev/null || true

        # Fix all dependency paths in this library
        DEPS=$(otool -L "$lib" 2>/dev/null | tail -n +2 | awk '{print $1}')
        for dep in $DEPS; do
            dep_name=$(basename "$dep")
            # Fix non-system paths and @rpath references
            if [[ "$dep" != /usr/lib/* ]] && [[ "$dep" != /System/* ]] && [[ "$dep" != @executable_path/* ]]; then
                install_name_tool -change "$dep" "@executable_path/../Frameworks/$dep_name" "$lib" 2>/dev/null || true
            fi
        done
    fi
done

# Fix paths in the main binary
echo "Fixing main binary..."
DEPS=$(otool -L "$BINARY" 2>/dev/null | tail -n +2 | awk '{print $1}')
for dep in $DEPS; do
    dep_name=$(basename "$dep")
    # Fix non-system paths and @rpath references
    if [[ "$dep" != /usr/lib/* ]] && [[ "$dep" != /System/* ]] && [[ "$dep" != @executable_path/* ]]; then
        install_name_tool -change "$dep" "@executable_path/../Frameworks/$dep_name" "$BINARY" 2>/dev/null || true
    fi
done

# Count bundled libraries
LIB_COUNT=$(ls -1 "$FRAMEWORKS_DIR"/*.dylib 2>/dev/null | wc -l | tr -d ' ')
echo "Done! Bundled $LIB_COUNT libraries into $FRAMEWORKS_DIR"

# Verify the binary no longer references homebrew paths
echo ""
echo "Verifying binary dependencies..."
REMAINING=$(otool -L "$BINARY" | grep "/opt/homebrew" | wc -l | tr -d ' ')
if [ "$REMAINING" -gt 0 ]; then
    echo "Warning: Binary still has $REMAINING homebrew references:"
    otool -L "$BINARY" | grep "/opt/homebrew"
else
    echo "Success! No homebrew paths remaining in binary."
fi

# Re-sign all libraries and the binary (required after modifying with install_name_tool)
echo ""
echo "Re-signing libraries and binary..."
for lib in "$FRAMEWORKS_DIR"/*.dylib; do
    if [ -f "$lib" ]; then
        codesign --force --sign - "$lib" 2>/dev/null || true
    fi
done
codesign --force --sign - "$BINARY" 2>/dev/null || true
echo "Code signing complete."
