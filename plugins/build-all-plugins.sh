#!/bin/bash

set -e

# Function to print styled log message
log() {
    echo "➡ $1"
}

# Function to print styled success message
success() {
    echo "✓ $1"
}

# Function to print styled error message
error() {
    echo "✗ $1" >&2
}

# Function to set up plugin dependencies
setup_plugin_dependencies() {
    local plugin_dir="$1"
    local plugin_name="$2"
    (
        cd "$plugin_dir"
        if [ ! -f "go.mod" ]; then
            go mod init github.com/ssotops/gitspace-catalog/plugins/$plugin_name
        fi
        go get github.com/ssotops/gitspace-plugin-sdk@latest
        go get github.com/charmbracelet/huh@latest
        go mod tidy
    )
}

# Function to install plugin
install_plugin() {
    local plugin_name="$1"
    local plugin_path="$2"
    local install_dir="$HOME/.ssot/gitspace/plugins/$plugin_name"
    local data_dir="$HOME/.ssot/gitspace/plugins/data/$plugin_name"
    
    # Ensure the installation and data directories exist
    mkdir -p "$install_dir"
    mkdir -p "$data_dir"
    
    # Copy the plugin binary to the installation directory
    cp "$plugin_path/$plugin_name" "$install_dir/"
    
    # Copy all files and directories recursively from the plugin directory to the data directory
    cp -R "$plugin_path"/* "$data_dir/"
    
    # Set up Node.js environment for scmtea plugin
    if [ "$plugin_name" == "scmtea" ]; then
        setup_nodejs_env "$data_dir"
    fi
    
    log "Installed $plugin_name to $install_dir and copied all files to $data_dir"
}

setup_nodejs_env() {
    local dir="$1"
    
    # Check if bun is installed
    if ! command -v bun &> /dev/null; then
        log "Installing bun..."
        curl -fsSL https://bun.sh/install | bash
    fi

    # Navigate to the directory
    cd "$dir"

    # Initialize a new package if package.json doesn't exist
    if [ ! -f "package.json" ]; then
        log "Initializing new package..."
        bun init -y
    fi

    # Ensure the package.json has "type": "module"
    if command -v jq &> /dev/null; then
        jq '.type = "module"' package.json > package.json.tmp && mv package.json.tmp package.json
    else
        log "jq is not installed. Manually ensure that package.json has \"type\": \"module\""
    fi

    # Install dependencies
    log "Installing dependencies..."
    bun add puppeteer

    # Return to the original directory
    cd -
}

# Function to update root .gitignore
update_gitignore() {
    local plugin_name="$1"
    local gitignore_file="$(git rev-parse --show-toplevel)/.gitignore"
    
    # Create .gitignore if it doesn't exist
    touch "$gitignore_file"
    
    # Check if the binary is already in .gitignore
    if ! grep -q "^plugins/$plugin_name/$plugin_name$" "$gitignore_file"; then
        echo "plugins/$plugin_name/$plugin_name" >> "$gitignore_file"
        log "Added $plugin_name binary to root .gitignore"
    else
        log "$plugin_name binary already in root .gitignore"
    fi
}

# Build all plugins in the catalog
build_plugins() {
    for plugin_dir in */; do
        if [ -d "$plugin_dir" ]; then
            plugin_name=${plugin_dir%/}
            log "Setting up dependencies for plugin: $plugin_name"
            setup_plugin_dependencies "$plugin_dir" "$plugin_name"
            
            log "Building plugin: $plugin_name"
            (
                cd "$plugin_dir"
                go build -o "$plugin_name"
                if [ $? -eq 0 ]; then
                    success "Plugin $plugin_name built successfully."
                    install_plugin "$plugin_name" "$PWD"
                    update_gitignore "$plugin_name"
                else
                    error "Failed to build plugin $plugin_name."
                    exit 1
                fi
            )
        fi
    done
}

# Main execution
cd "$(git rev-parse --show-toplevel)/plugins"
log "Building and installing all plugins in the catalog..."
build_plugins
success "All plugins built, installed, and root .gitignore updated successfully."
