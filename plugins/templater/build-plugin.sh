#!/bin/bash

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

# Function to run command and check for errors
run_command() {
    local cmd="$1"
    echo "Running: $cmd"
    output=$(eval "$cmd" 2>&1)
    exit_code=$?
    if [ $exit_code -ne 0 ]; then
        gum style \
            --foreground 196 --border-foreground 196 --border normal \
            --align left --width 70 --margin "1 2" --padding "1 2" \
            "Command failed: $cmd\nError output:\n$output"
        return 1
    fi
    echo "$output"
    return 0
}

# Update dependencies
if ! run_command "go mod tidy"; then
    exit 1
fi

# Build standalone binary
if ! run_command "go build -o \"$OUTPUT_DIR/${PLUGIN_NAME}\" \"$PLUGIN_DIR\""; then
    exit 1
fi

# Build plugin .so file
if ! run_command "go build -buildmode=plugin -o \"$OUTPUT_DIR/${PLUGIN_NAME}.so\" \"$PLUGIN_DIR\""; then
    exit 1
fi

# Check if files were actually created
if [ ! -f "$OUTPUT_DIR/${PLUGIN_NAME}" ] || [ ! -f "$OUTPUT_DIR/${PLUGIN_NAME}.so" ]; then
    gum style \
        --foreground 196 --border-foreground 196 --border normal \
        --align center --width 70 --margin "1 2" --padding "1 2" \
        "Build failed: Output files were not created."
    exit 1
fi

# Print summary
gum style \
    --foreground 82 --border-foreground 82 --border normal \
    --align left --width 70 --margin "1 2" --padding "1 2" \
    "Build complete!
Standalone binary: $OUTPUT_DIR/${PLUGIN_NAME}
Plugin .so file: $OUTPUT_DIR/${PLUGIN_NAME}.so"
