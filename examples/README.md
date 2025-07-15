# Examples

This directory contains examples that are mostly used for documentation, but can also be run/tested manually via the Terraform CLI.

The document generation tool looks for files in the following locations by default. All other *.tf files besides the ones mentioned below are ignored by the documentation tool. This is useful for creating examples that can run and/or are testable even if some parts are not relevant for the documentation.

* **provider/provider.tf** - example file for the provider index page
* **data-sources/`full data source name`/data-source.tf** - example file for the named data source page  
* **resources/`full resource name`/resource.tf** - example file for the named resource page

## Available Examples

### Provider Configuration
- **provider/provider.tf** - Shows how to configure the Neptune provider with authentication and workspace settings

### Resources
- **resources/neptune_project/resource.tf** - Comprehensive examples of creating Neptune projects with various configurations
- **resources/neptune_project/import.sh** - Examples of importing existing Neptune projects into Terraform

### Data Sources  
- **data-sources/neptune_project/data-source.tf** - Examples of reading existing Neptune project data

## Running Examples

To test these examples:

1. Set your Neptune credentials:
   ```bash
   export TF_VAR_neptune_token="your-neptune-token"
   export TF_VAR_workspace="your-workspace-name"
   ```

2. Navigate to an example directory and run:
   ```bash
   terraform init
   terraform plan
   terraform apply
   ```
