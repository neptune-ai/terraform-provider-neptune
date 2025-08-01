terraform {
  required_providers {
    neptune = {
      source = "neptune-ai/neptune"
    }
  }
}

provider "neptune" {
  neptune_token = var.neptune_token
  workspace     = var.workspace
}


# Project with custom avatar and colors
resource "neptune_project" "example_project" {
  name        = "example-project"
  description = "Project with examples"
  visibility  = "priv"
}


# Basic project email assignment
resource "neptune_project_email_assignment" "member_assignment" {
  project = neptune_project.example_project.id
  email   = "user@example.com"
  role    = "member"
}

# Assignment with owner role
resource "neptune_project_email_assignment" "owner_assignment" {
  project = neptune_project.example_project.id
  email   = "admin@example.com"
  role    = "owner"
}

# Variables
variable "neptune_token" {
  description = "Neptune API token from user or service account"
  type        = string
  sensitive   = true
}

variable "workspace" {
  description = "Neptune workspace name (organization identifier)"
  type        = string
}