#!/bin/bash

set -e

PLUGIN_NAME="templater"
PLUGIN_DIR="."
OUTPUT_DIR="./build"

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

# Build standalone binary
echo "Building standalone binary..."
go build -o "$OUTPUT_DIR/${PLUGIN_NAME}" "$PLUGIN_DIR"

# Build plugin .so file
echo "Building plugin .so file..."
go build -buildmode=plugin -o "$OUTPUT_DIR/${PLUGIN_NAME}.so" "$PLUGIN_DIR"

echo "Build complete!"
echo "Standalone binary: $OUTPUT_DIR/${PLUGIN_NAME}"
echo "Plugin .so file: $OUTPUT_DIR/${PLUGIN_NAME}.so"
