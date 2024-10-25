#!/bin/bash
set -euo pipefail

# Default values
DEFAULT_BUCKET="scmtea-backups"

# Styling functions
style_header() {
    local text="$1"
    printf "%s" "$text" | gum style \
        --border double \
        --border-foreground 212 \
        --align center \
        --width 50 \
        --margin "1" \
        --padding "1 2"
}

log() {
    local text="$1"
    printf "○ %s\n" "$text" | gum style --foreground 212 
}

success() {
    local text="$1"
    printf "✓ %s\n" "$text" | gum style --foreground 46
}

warn() {
    local text="$1"
    printf "! %s\n" "$text" | gum style --foreground 178
}

error() {
    local text="$1"
    printf "✗ %s\n" "$text" | gum style --foreground 196
    exit 1
}

style_box() {
    local text="$1"
    printf "%s" "$text" | gum style \
        --border normal \
        --border-foreground 212 \
        --padding "1 2" \
        --margin "1"
}

check_dependencies() {
    log "Checking required dependencies..."
    
    # Check 1Password CLI
    if ! command -v op &> /dev/null; then
        error "1Password CLI (op) is required but not installed"
    fi

    # Check gum
    if ! command -v gum &> /dev/null; then
        error "Gum is required but not installed. Install with: brew install gum"
    fi

    # Check jq
    if ! command -v jq &> /dev/null; then
        error "jq is required but not installed. Install with: brew install jq"
    fi

    # Check terraform
    if ! command -v terraform &> /dev/null; then
        error "Terraform is required but not installed. Install with: brew install terraform"
    fi

    success "All dependencies are installed"
}

check_auth() {
    log "Checking authentication status..."
    
    # Verify 1Password CLI is authenticated
    if ! op account list &>/dev/null; then
        error "Not authenticated with 1Password CLI. Please run 'op signin' first"
    fi

    success "Successfully authenticated with 1Password"
}

get_git_username() {
    local git_name
    git_name=$(git config --global user.name || echo "backups")
    echo "$git_name"
}

prompt_for_values() {
    log "Configuring backup settings..."
    
    # Prompt for bucket name with default value
    local bucket_input
    bucket_input=$(gum input \
        --placeholder "$DEFAULT_BUCKET" \
        --prompt "Enter bucket name: " \
        --value "$DEFAULT_BUCKET"
    )
    BUCKET_NAME=${bucket_input:-$DEFAULT_BUCKET}
    
    # Get git username for default path
    local git_username
    git_username=$(get_git_username)
    
    # Prompt for S3 path with git username as default
    local path_input
    path_input=$(gum input \
        --placeholder "$git_username" \
        --prompt "Enter S3 path: " \
        --value "$git_username"
    )
    S3_PATH=${path_input:-$git_username}
    
    success "Using bucket: $BUCKET_NAME"
    success "Using path: $S3_PATH"
}

get_spaces_credentials() {
    log "Retrieving Spaces credentials from 1Password..."
    
    # Get the item details from 1Password
    if ! op item get "DigitalOcean" --vault ssotops --format json > /tmp/do_creds.json; then
        error "Failed to retrieve DigitalOcean credentials from 1Password"
    fi
    
    # Extract credentials using jq
    local spaces_key spaces_secret do_token
    
    spaces_key=$(jq -r '.fields[] | select(.label=="DO_SPACES_KEY") | .value' /tmp/do_creds.json)
    spaces_secret=$(jq -r '.fields[] | select(.label=="DO_SPACES_SECRET") | .value' /tmp/do_creds.json)
    do_token=$(jq -r '.fields[] | select(.label=="DO_TOKEN") | .value' /tmp/do_creds.json)

    if [[ -z "$spaces_key" || -z "$spaces_secret" || -z "$do_token" ]]; then
        rm -f /tmp/do_creds.json
        error "Could not find required credentials in 1Password"
    fi

    # Save credentials for Terraform
    export TF_VAR_do_token="$do_token"
    export TF_VAR_spaces_access_key="$spaces_key"
    export TF_VAR_spaces_secret_key="$spaces_secret"
    export TF_VAR_bucket_name="$BUCKET_NAME"
    export TF_VAR_bucket_path="$S3_PATH"

    # Create config file
    cat > gitea-backup-config.env <<EOF
# DigitalOcean Spaces Configuration for Gitea Backups
# Retrieved from 1Password vault: ssotops
# Generated: $(date)

S3_BUCKET=$BUCKET_NAME
S3_PATH=$S3_PATH
S3_ENDPOINT=nyc3.digitaloceanspaces.com
S3_ACCESS_KEY=$spaces_key
S3_SECRET_KEY=$spaces_secret

# Optional: DO API Token (if needed for additional operations)
DO_TOKEN=$do_token
EOF

    # Cleanup
    rm -f /tmp/do_creds.json
    
    success "Created gitea-backup-config.env with credentials"

    # Display the configuration
    local config_display="Gitea Backup Configuration
-------------------------
s3_bucket: $BUCKET_NAME
s3_path: $S3_PATH
endpoint: nyc3.digitaloceanspaces.com
access_key: $spaces_key
secret_key: $spaces_secret"

    style_box "$config_display"
}

update_1password_config() {
    log "Updating 1Password configuration..."

    # Get Terraform outputs
    local tf_output
    local config_summary
    local bucket_endpoint
    local bucket_name
    local space_path
    local space_region

    tf_output=$(terraform output -json)
    config_summary=$(echo "$tf_output" | jq -r '.configuration_summary.value | tostring')
    bucket_endpoint=$(echo "$tf_output" | jq -r '.space_bucket_endpoint.value')
    bucket_name=$(echo "$tf_output" | jq -r '.space_bucket_name.value')
    space_path=$(echo "$tf_output" | jq -r '.space_path.value')
    space_region=$(echo "$tf_output" | jq -r '.space_region.value')

    # Update the item in 1Password using field assignment syntax
    if ! op item edit "DigitalOcean" --vault ssotops \
        "BACKUP_CONFIG_SUMMARY[password]=$config_summary" \
        "BACKUP_BUCKET_ENDPOINT[text]=$bucket_endpoint" \
        "BACKUP_BUCKET_NAME[text]=$bucket_name" \
        "BACKUP_SPACE_PATH[text]=$space_path" \
        "BACKUP_SPACE_REGION[text]=$space_region" \
        "notesPlain=Gitea Backup Configuration\nLast Updated: $(date)"; then
        error "Failed to update 1Password item"
    fi

    success "1Password configuration updated successfully"

    # Display confirmation with new values
    local confirm_message="Updated 1Password Configuration:
-------------------------
Bucket Name: $bucket_name
Bucket Endpoint: $bucket_endpoint
Space Path: $space_path
Region: $space_region"
    
    style_box "$confirm_message"
}

run_terraform() {
    log "Preparing to create infrastructure..."

    # Initialize Terraform
    log "Initializing Terraform..."
    if ! terraform init > /tmp/tf_init.log 2>&1; then
        error "Terraform initialization failed. Check /tmp/tf_init.log for details"
    fi
    success "Terraform initialized"

    # Run Terraform plan
    log "Planning infrastructure changes..."
    if ! terraform plan -out=tfplan > /tmp/tf_plan.log 2>&1; then
        error "Terraform plan failed. Check /tmp/tf_plan.log for details"
    fi
    success "Terraform plan created"

    # Show the plan summary
    local plan_summary
    plan_summary=$(terraform show tfplan | grep -A 20 "Plan:" || true)
    style_box "Terraform Plan Summary:
-------------------------
$plan_summary"

    # Confirm before applying
    if gum confirm "Would you like to create the infrastructure now?"; then
        log "Applying infrastructure changes..."
        if ! terraform apply tfplan > /tmp/tf_apply.log 2>&1; then
            error "Terraform apply failed. Check /tmp/tf_apply.log for details"
        fi
        success "Infrastructure created successfully"

        # Show outputs
        log "Retrieving infrastructure details..."
        local tf_output
        tf_output=$(terraform output -json | jq -r 'to_entries | .[] | "\(.key): \(.value.value)"')
        style_box "Created Infrastructure:
-------------------------
$tf_output"

        # Update 1Password configuration
        update_1password_config

    else
        warn "Infrastructure creation skipped"
    fi

    # Cleanup
    rm -f tfplan
}

main() {
    clear
    style_header "Gitea Backup Configuration Setup"
    
    check_dependencies
    check_auth
    prompt_for_values
    get_spaces_credentials
    run_terraform
    
    style_header "Configuration Complete! 🚀"
}

main "$@"
