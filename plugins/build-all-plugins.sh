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
            log "Initializing Go module for $plugin_name"
            go mod init github.com/ssotops/gitspace-catalog/plugins/$plugin_name
        fi
        
        log "Getting latest dependencies for $plugin_name"
        go get github.com/ssotops/gitspace-plugin-sdk@latest
        go get github.com/charmbracelet/huh@latest
        go mod tidy
        
        if [ $? -ne 0 ]; then
            error "Failed to set up dependencies for $plugin_name"
            return 1
        fi
        success "Dependencies set up for $plugin_name"
    )
}

# Function to build a plugin
build_plugin() {
    local plugin_dir="$1"
    local plugin_name="$2"
    (
        cd "$plugin_dir"
        log "Building $plugin_name..."
        go build -o "$plugin_name"
        
        if [ $? -ne 0 ]; then
            error "Failed to build $plugin_name"
            return 1
        fi
        
        # Make the binary executable
        chmod +x "$plugin_name"
        success "Built $plugin_name successfully"
    )
}

# Function to install plugin
install_plugin() {
    local plugin_name="$1"
    local source_dir="$2"
    local install_dir="$HOME/.ssot/gitspace/plugins/$plugin_name"
    
    # Create installation directory
    mkdir -p "$install_dir"
    
    # Copy the plugin binary to the installation directory
    if [ -f "$source_dir/$plugin_name" ]; then
        cp "$source_dir/$plugin_name" "$install_dir/"
        chmod +x "$install_dir/$plugin_name"
        success "Installed binary for $plugin_name"
    else
        error "Plugin binary not found at $source_dir/$plugin_name"
        return 1
    fi
    
    # Create data directory and copy support files
    local data_dir="$HOME/.ssot/gitspace/plugins/data/$plugin_name"
    mkdir -p "$data_dir"
    cp -R "$source_dir"/* "$data_dir/"
    
    success "Installed $plugin_name to $install_dir"
}

# Function to update root .gitignore
update_gitignore() {
    local plugin_name="$1"
    local gitignore_file="$(git rev-parse --show-toplevel)/.gitignore"
    
    touch "$gitignore_file"
    
    if ! grep -q "^plugins/$plugin_name/$plugin_name$" "$gitignore_file"; then
        echo "plugins/$plugin_name/$plugin_name" >> "$gitignore_file"
        success "Updated .gitignore for $plugin_name"
    fi
}

# Main plugin build and installation function
process_plugin() {
    local plugin_dir="$1"
    local plugin_name=$(basename "$plugin_dir")
    
    log "Processing plugin: $plugin_name"
    
    # Set up dependencies
    if ! setup_plugin_dependencies "$plugin_dir" "$plugin_name"; then
        error "Failed to set up dependencies for $plugin_name"
        return 1
    fi
    
    # Build the plugin
    if ! build_plugin "$plugin_dir" "$plugin_name"; then
        error "Failed to build $plugin_name"
        return 1
    fi
    
    # Install the plugin
    if ! install_plugin "$plugin_name" "$plugin_dir"; then
        error "Failed to install $plugin_name"
        return 1
    fi
    
    # Update .gitignore
    update_gitignore "$plugin_name"
    
    success "Successfully processed $plugin_name"
}

# Main execution
main() {
    cd "$(git rev-parse --show-toplevel)/plugins"
    log "Building and installing all plugins in the catalog..."
    
    local failed_plugins=()
    
    for dir in */; do
        if [ -d "$dir" ]; then
            if ! process_plugin "$dir"; then
                failed_plugins+=("$dir")
            fi
        fi
    done
    
    if [ ${#failed_plugins[@]} -eq 0 ]; then
        success "All plugins processed successfully"
    else
        error "Failed to process the following plugins:"
        for plugin in "${failed_plugins[@]}"; do
            error "  - $plugin"
        done
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main
fi
