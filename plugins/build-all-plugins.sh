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
    
    # Ensure the installation directory exists
    mkdir -p "$install_dir"
    
    # Copy the plugin to the installation directory
    cp "$plugin_path" "$install_dir/"
    
    log "Installed $plugin_name to $install_dir"
}

# Build all plugins in the catalog
build_plugins() {
    for plugin_dir in */; do
        if [ -d "$plugin_dir" ]; then
            plugin_name=${plugin_dir%/}
            log "Setting up dependencies for plugin: $plugin_name"
            setup_plugin_dependencies "$plugin_dir"
            
            log "Building plugin: $plugin_name"
            (
                cd "$plugin_dir"
                go build -o "$plugin_name"
                if [ $? -eq 0 ]; then
                    success "Plugin $plugin_name built successfully."
                    install_plugin "$plugin_name" "$plugin_name"
                else
                    error "Failed to build plugin $plugin_name."
                    exit 1
                fi
            )
        fi
    done
}

# Update gitspace-catalog.toml
update_catalog() {
    log "Updating gitspace-catalog.toml"
    local catalog_file="../gitspace-catalog.toml"
    
    if [ ! -f "$catalog_file" ]; then
        error "gitspace-catalog.toml not found at $catalog_file"
        return 1
    fi

    # Update the catalog version and last updated date
    sed -i.bak "s/version = .*/version = \"$(date +%Y.%m.%d)\"/" "$catalog_file"
    sed -i.bak "s/date = .*/date = \"$(date -u +"%Y-%m-%dT%H:%M:%SZ")\"/" "$catalog_file"
    
    # Update commit hash
    local commit_hash=$(git rev-parse HEAD)
    sed -i.bak "s/commit_hash = .*/commit_hash = \"$commit_hash\"/" "$catalog_file"
    
    rm "${catalog_file}.bak"
    
    success "Updated gitspace-catalog.toml"
}

# Main execution
log "Building and installing all plugins in the catalog..."
build_plugins
update_catalog
success "All plugins built, installed, and catalog updated successfully."
