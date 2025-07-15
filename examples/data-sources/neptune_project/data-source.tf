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

# Look up project by UUID
data "neptune_project" "by_id" {
  id = "01234567-89ab-cdef-0123-456789abcdef"
}

# Look up project by name (uses workspace from provider)
data "neptune_project" "by_name" {
  project_name = "my-existing-project"
}

# Look up another project by name
data "neptune_project" "ml_experiments" {
  project_name = "machine-learning-experiments"
}

# Create a new project and then reference it in a data source
resource "neptune_project" "new_project" {
  name        = "data-source-example"
  description = "Project created to demonstrate data source usage"
  visibility  = "priv"
}

# Use data source to read the created project
data "neptune_project" "from_resource" {
  id = neptune_project.new_project.id
}

# Output project information for the project looked up by ID
output "project_by_id" {
  description = "Details of project looked up by ID"
  value = {
    id            = data.neptune_project.by_id.id
    name          = data.neptune_project.by_id.name
    project_key   = data.neptune_project.by_id.project_key
    visibility    = data.neptune_project.by_id.visibility
    description   = data.neptune_project.by_id.description
    avatar        = data.neptune_project.by_id.avatar
    avatar_source = data.neptune_project.by_id.avatar_source
    color         = data.neptune_project.by_id.color
  }
}

# Output project information for the project looked up by name
output "project_by_name" {
  description = "Details of project looked up by name"
  value = {
    id          = data.neptune_project.by_name.id
    name        = data.neptune_project.by_name.name
    project_key = data.neptune_project.by_name.project_key
    visibility  = data.neptune_project.by_name.visibility
    description = data.neptune_project.by_name.description
  }
}

# Output comparison between resource and data source
output "resource_vs_datasource" {
  description = "Comparison between created resource and data source lookup"
  value = {
    resource_id   = neptune_project.new_project.id
    datasource_id = data.neptune_project.from_resource.id
    names_match   = neptune_project.new_project.name == data.neptune_project.from_resource.name
    keys_match    = neptune_project.new_project.project_key == data.neptune_project.from_resource.project_key
  }
}

# Output all project keys for reference
output "all_project_keys" {
  description = "Project keys from all data sources"
  value = {
    by_id          = data.neptune_project.by_id.project_key
    by_name        = data.neptune_project.by_name.project_key
    ml_experiments = data.neptune_project.ml_experiments.project_key
    from_resource  = data.neptune_project.from_resource.project_key
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
