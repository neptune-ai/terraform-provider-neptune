package provider

import (
	"context"
	"os"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// Ensure the implementation satisfies the expected interfaces.
var (
	_ provider.Provider = &neptuneProvider{}
)

// New is a helper function to simplify provider server and testing implementation.
func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &neptuneProvider{
			version: version,
		}
	}
}

// neptuneProvider is the provider implementation.
type neptuneProvider struct {
	// version is set to the provider version on release, "dev" when the
	// provider is built and ran locally, and "test" when running acceptance
	// testing.
	version string
}

type neptuneProviderModel struct {
	NeptuneToken types.String `tfsdk:"neptune_token"`
	Workspace    types.String `tfsdk:"workspace"`
	Timeout      types.Int64  `tfsdk:"timeout"`
}

// Metadata returns the provider type name.
func (p *neptuneProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "neptune"
	resp.Version = p.version
}

// Schema defines the provider-level schema for configuration data.
func (p *neptuneProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"neptune_token": schema.StringAttribute{
				Description: "The Neptune API token. Can be taken from User or a Service Account. Can also be provided via NEPTUNE_API_TOKEN environment variable.",
				Optional:    true,
				Sensitive:   true,
			},
			"workspace": schema.StringAttribute{
				Description: "The Neptune workspace name. Can also be provided via NEPTUNE_WORKSPACE environment variable.",
				Optional:    true,
			},
			"timeout": schema.Int64Attribute{
				Description: "The timeout for the Neptune API client",
				Optional:    true,
			},
		},
	}
	resp.Schema.MarkdownDescription = "The Neptune provider is used to interact with the Neptune.ai resources"
}

// Configure prepares a Neptune API client for data sources and resources.
func (p *neptuneProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	// Retrieve provider data from configuration
	var config neptuneProviderModel
	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve configuration from explicit values or environment variables
	token := strings.TrimSpace(config.NeptuneToken.ValueString())
	if token == "" {
		// Value from environment variable
		token = strings.TrimSpace(os.Getenv("NEPTUNE_API_TOKEN"))
	}

	workspace := strings.TrimSpace(config.Workspace.ValueString())
	if workspace == "" {
		workspace = strings.TrimSpace(os.Getenv("NEPTUNE_WORKSPACE"))
	}

	if token == "" {
		resp.Diagnostics.AddError(
			"Missing Neptune API token",
			"Provide `neptune_token` in the provider configuration or set the NEPTUNE_API_TOKEN environment variable.",
		)
		return
	}

	if workspace == "" {
		resp.Diagnostics.AddError(
			"Missing Neptune workspace",
			"Provide `workspace` in the provider configuration or set the NEPTUNE_WORKSPACE environment variable.",
		)
		return
	}

	client, err := NewNeptuneClient(token, workspace, config.Timeout.ValueInt64(), p.version)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Create Neptune API Client",
			"An unexpected error occurred when creating the Neptune API client. If the error is not clear, please contact the provider developers.\n\n"+
				"Neptune Client Error: "+err.Error(),
		)
		return
	}

	resp.DataSourceData = client
	resp.ResourceData = client
}

// DataSources defines the data sources implemented in the provider.
func (p *neptuneProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewProjectDataSource,
		// NewProjectEmailAssignmentDataSource,
	}
}

// Resources defines the resources implemented in the provider.
func (p *neptuneProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewProjectResource,
		NewProjectEmailAssignmentResource,
	}
}
