# main.tf

terraform {
  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.34.0"
    }
  }
}

# Variables
variable "do_token" {
  description = "DigitalOcean API token"
  type        = string
  sensitive   = true
}

variable "spaces_access_key" {
  description = "DigitalOcean Spaces access key"
  type        = string
  sensitive   = true
}

variable "spaces_secret_key" {
  description = "DigitalOcean Spaces secret key"
  type        = string
  sensitive   = true
}

variable "bucket_name" {
  description = "Name of the Spaces bucket"
  type        = string
}

variable "bucket_path" {
  description = "Path within the bucket"
  type        = string
}

variable "region" {
  description = "DigitalOcean region"
  type        = string
  default     = "nyc3"
}

# Provider configuration
provider "digitalocean" {
  token             = var.do_token
  spaces_access_id  = var.spaces_access_key
  spaces_secret_key = var.spaces_secret_key
}

# Space bucket
resource "digitalocean_spaces_bucket" "gitea_backup" {
  name   = var.bucket_name
  region = var.region
  acl    = "private"

  versioning {
    enabled = true
  }

  lifecycle_rule {
    enabled = true
    prefix  = "${var.bucket_path}/"

    noncurrent_version_expiration {
      days = 30
    }

    expiration {
      days = 90
    }
  }
}

# CDN Endpoint (optional, commented out by default)
# resource "digitalocean_cdn" "gitea_backup_cdn" {
#   origin = digitalocean_spaces_bucket.gitea_backup.bucket_domain_name
#   ttl    = 3600
# }

# Outputs
output "space_bucket_name" {
  value       = digitalocean_spaces_bucket.gitea_backup.name
  description = "Name of the created Spaces bucket"
}

output "space_bucket_endpoint" {
  value       = digitalocean_spaces_bucket.gitea_backup.bucket_domain_name
  description = "Endpoint for the Spaces bucket"
}

output "space_region" {
  value       = digitalocean_spaces_bucket.gitea_backup.region
  description = "Region where the Space is created"
}

output "space_path" {
  value       = var.bucket_path
  description = "Path within the bucket for backups"
}

output "configuration_summary" {
  value = {
    bucket_name = digitalocean_spaces_bucket.gitea_backup.name
    endpoint    = "${var.region}.digitaloceanspaces.com"
    region     = var.region
    path       = var.bucket_path
  }
  description = "Summary of the Spaces configuration"
}
