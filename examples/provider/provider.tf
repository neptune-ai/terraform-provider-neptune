terraform {
  required_providers {
    neptune = {
      source = "neptune-ai/neptune"
    }
  }
}

provider "neptune" {
  # Neptune API token (can be from user or service account)
  # Can also be set via NEPTUNE_TOKEN environment variable
  neptune_token = var.neptune_token

  # Neptune workspace name (organization identifier)
  # Can also be set via NEPTUNE_WORKSPACE environment variable
  workspace = var.workspace

  # Optional: Request timeout in seconds (default: 30)
  # timeout = 60
}

variable "neptune_token" {
  description = "Neptune API token from user or service account"
  type        = string
  sensitive   = true
}

variable "workspace" {
  description = "Neptune workspace name (organization identifier)"
  type        = string
}
