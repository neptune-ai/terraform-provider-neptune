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

# Basic project with minimal configuration
resource "neptune_project" "basic" {
  name = "my-basic-project"
}

# Project with description
resource "neptune_project" "with_description" {
  name        = "ml-experiment-project"
  description = "Machine learning experiments and model tracking"
}

# Private project with custom settings
resource "neptune_project" "private" {
  name          = "private-research-project"
  description   = "Confidential research project"
  project_key   = "PRIV"
  visibility    = "priv"
  avatar        = "https://example.com/avatar.png"
  avatar_source = "user"
  color         = "blue"
}

# Public project for demonstrations
resource "neptune_project" "public" {
  name          = "public-demo-project"
  description   = "Public project for demonstrations and tutorials"
  project_key   = "DEMO"
  visibility    = "pub"
  avatar_source = "default"
}

# Workspace-visible project
resource "neptune_project" "workspace" {
  name        = "team-workspace-project"
  description = "Shared project visible to all workspace members"
  visibility  = "workspace"
  color       = "#00FF00"
}

# Project with custom avatar and colors
resource "neptune_project" "customized" {
  name          = "customized-project"
  description   = "Project with custom branding"
  project_key   = "CUSTOM"
  visibility    = "priv"
  avatar        = "https://example.com/custom-logo.png"
  avatar_source = "user"
  color         = "red"
}

# Output project details
output "basic_project_id" {
  description = "UUID of the basic project"
  value       = neptune_project.basic.id
}

output "basic_project_key" {
  description = "Auto-generated project key"
  value       = neptune_project.basic.project_key
}

output "private_project_details" {
  description = "Complete details of the private project"
  value = {
    id          = neptune_project.private.id
    name        = neptune_project.private.name
    project_key = neptune_project.private.project_key
    visibility  = neptune_project.private.visibility
    description = neptune_project.private.description
    avatar      = neptune_project.private.avatar
    color       = neptune_project.private.color
  }
}

output "all_project_keys" {
  description = "All project keys for reference"
  value = {
    basic      = neptune_project.basic.project_key
    private    = neptune_project.private.project_key
    public     = neptune_project.public.project_key
    workspace  = neptune_project.workspace.project_key
    customized = neptune_project.customized.project_key
  }
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
