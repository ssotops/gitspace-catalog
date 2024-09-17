#!/bin/bash

set -e

PLUGIN_NAME="templater"
PLUGIN_DIR="."
OUTPUT_DIR="./dist"

# Function to install gum
install_gum() {
    echo "Installing gum..."
    if [[ "$OSTYPE" == "darwin"* ]]; then
        brew install gum
    elif [[ "$OSTYPE" == "linux-gnu"* ]]; then
        echo "For Ubuntu/Debian:"
        echo "sudo mkdir -p /etc/apt/keyrings"
        echo "curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg"
        echo 'echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list'
        echo "sudo apt update && sudo apt install gum"
        echo ""
        echo "For other Linux distributions, please visit: https://github.com/charmbracelet/gum#installation"
        read -p "Do you want to proceed with the installation for Ubuntu/Debian? (y/n) " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            sudo mkdir -p /etc/apt/keyrings
            curl -fsSL https://repo.charm.sh/apt/gpg.key | sudo gpg --dearmor -o /etc/apt/keyrings/charm.gpg
            echo "deb [signed-by=/etc/apt/keyrings/charm.gpg] https://repo.charm.sh/apt/ * *" | sudo tee /etc/apt/sources.list.d/charm.list
            sudo apt update && sudo apt install gum
        else
            echo "Please install gum manually and run this script again."
            exit 1
        fi
    else
        echo "Unsupported operating system. Please install gum manually:"
        echo "https://github.com/charmbracelet/gum#installation"
        exit 1
    fi
}

# Check if gum is installed
if ! command -v gum &> /dev/null; then
    echo "gum is not installed."
    read -p "Do you want to install gum? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        install_gum
    else
        echo "Please install gum manually and run this script again."
        exit 1
    fi
fi

# Ensure output directory exists
mkdir -p "$OUTPUT_DIR"

# ASCII Art for plugin builder using gum
gum style \
    --foreground 212 --border-foreground 212 --border double \
    --align center --width 70 --margin "1 2" --padding "1 2" \
    "${PLUGIN_NAME} Plugin Builder"

# Build standalone binary
gum spin --spinner dot --title "Building standalone binary..." -- bash -c 'go build -o "$OUTPUT_DIR/${PLUGIN_NAME}" "$PLUGIN_DIR" 2>&1 || echo "Build failed: $?"'

# Build plugin .so file
gum spin --spinner dot --title "Building plugin .so file..." -- bash -c 'go build -buildmode=plugin -o "$OUTPUT_DIR/${PLUGIN_NAME}.so" "$PLUGIN_DIR" 2>&1 || echo "Build failed: $?"'

# Print summary
gum style \
    --foreground 82 --border-foreground 82 --border normal \
    --align left --width 70 --margin "1 2" --padding "1 2" \
    "Build complete!
Standalone binary: $OUTPUT_DIR/${PLUGIN_NAME}
Plugin .so file: $OUTPUT_DIR/${PLUGIN_NAME}.so"

# Check for build errors
if [ $? -ne 0 ]; then
    gum style \
        --foreground 196 --border-foreground 196 --border normal \
        --align center --width 70 --margin "1 2" --padding "1 2" \
        "Build failed. Please check the error messages above."
    exit 1
fi
