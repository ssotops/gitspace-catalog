# Gitea Backup Infrastructure Setup

This repository contains Terraform configurations and setup scripts for managing Gitea backups using DigitalOcean Spaces and 1Password for secure credential storage.

## Prerequisites

Install the required tools:

```bash
# On macOS
brew install terraform doctl op

# On Linux (Ubuntu/Debian)
# Install Terraform
curl -fsSL https://apt.releases.hashicorp.com/gpg | sudo apt-key add -
sudo apt-add-repository "deb [arch=amd64] https://apt.releases.hashicorp.com $(lsb_release -cs) main"
sudo apt update
sudo apt install terraform

# Install doctl
snap install doctl

# Install 1Password CLI
curl -sS https://downloads.1password.com/linux/keys/1password.asc | sudo gpg --dearmor --output /usr/share/keyrings/1password-archive-keyring.gpg
echo "deb [arch=amd64 signed-by=/usr/share/keyrings/1password-archive-keyring.gpg] https://downloads.1password.com/linux/debian/amd64 stable main" | sudo tee /etc/apt/sources.list.d/1password.list
sudo apt update
sudo apt install 1password-cli
```

## Getting Required Tokens

### DigitalOcean Token (DO_TOKEN)

1. Log into your [DigitalOcean account](https://cloud.digitalocean.com)
2. Go to API > Generate New Token
3. Create a new token with both read and write access
4. Copy the token immediately (it won't be shown again)

Alternatively, if you have `doctl` configured:
```bash
# First time setup
doctl auth init  # Follow the prompts to enter your token

# Then you can get the token with
export DO_TOKEN=dop_***
```

### 1Password Service Account Token (OP_SERVICE_ACCOUNT_TOKEN_SCMTEA)

1. Log into your [1Password account](https://start.1password.com)
2. Navigate to Settings > Service Accounts
3. Click "New Service Account"
4. Fill in the details:
   - Name: `gitea-backup-automation`
   - Description: `Service account for managing Gitea backup credentials`
5. Click "Create Service Account"
6. In the next screen, select the "ssotops" vault and grant access
7. Copy the token shown (it won't be shown again)
   ```bash
   export OP_SERVICE_ACCOUNT_TOKEN_SCMTEA=ops_...
   ```

## Setup Instructions

1. Clone this repository:
   ```bash
   git clone <repo-url>
   cd gitea-backup-infrastructure
   ```

2. Export required environment variables:
   ```bash
   export DO_TOKEN="your-digitalocean-token"
   export OP_SERVICE_ACCOUNT_TOKEN_SCMTEA="your-1password-service-account-token"
   ```

3. Run the setup script:
   ```bash
   chmod +x setup.sh
   ./setup.sh
   ```

## What Gets Created

1. DigitalOcean Space:
   - Name: `gitea-backups`
   - Region: `nyc3`
   - Features:
     - Versioning enabled
     - 30-day retention policy
     - Private ACL

2. Spaces Access Keys:
   - A new key pair specifically for Gitea backups

3. 1Password Entries:
   - Vault: "ssotops"
   - Item: "Gitea Backup - DO Spaces"
   - Stored Information:
     - Spaces Access Key
     - Spaces Secret Key
     - Bucket Name
     - Endpoint
     - Region

## Retrieving Credentials

After setup, you can get the credentials in several ways:

1. From 1Password CLI:
   ```bash
   # List items
   op item list --vault ssotops

   # Get specific credential
   op item get "Gitea Backup - DO Spaces" --vault ssotops
   ```

2. From Terraform outputs:
   ```bash
   # View non-sensitive outputs
   terraform output
   ```

3. From 1Password GUI:
   - Open 1Password
   - Navigate to the "ssotops" vault
   - Find "Gitea Backup - DO Spaces"

## Configuration Values for Gitea

Use these values in the Gitea backup configuration:

```bash
s3_bucket: gitea-backups
s3_path: backups  # optional
endpoint: nyc3.digitaloceanspaces.com
access_key: <from 1Password>
secret_key: <from 1Password>
```

## Maintenance

### Updating Infrastructure

To update the infrastructure:

```bash
# Make changes to terraform files
terraform plan  # Review changes
terraform apply  # Apply changes
```

### Cleanup

To destroy the infrastructure:

```bash
terraform destroy
```

**Warning**: This will delete the Space and all backups. Make sure to download any important backups first.

## Troubleshooting

1. Token Issues:
   ```bash
   # Verify DO token works
   curl -X GET -H "Authorization: Bearer $DO_TOKEN" "https://api.digitalocean.com/v2/account"

   # Verify 1Password token works
   op whoami --token $OP_SERVICE_ACCOUNT_TOKEN_SCMTEA
   ```

2. Space Access Issues:
   ```bash
   # Test Space access
   aws s3 ls s3://gitea-backups --endpoint-url https://nyc3.digitaloceanspaces.com
   ```

## Security Notes

- Store tokens securely; never commit them to version control
- The 1Password service account has limited access to only the ssotops vault
- Spaces are created with private ACL by default
- All credentials are stored securely in 1Password

## Support

For issues:
1. Check the terraform logs: `terraform.tfstate`
2. Check DigitalOcean status: https://status.digitalocean.com/
3. Verify 1Password status: https://1password.status.io/
