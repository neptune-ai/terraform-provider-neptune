package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ datasource.DataSource = &ProjectDataSource{}

func NewProjectDataSource() datasource.DataSource {
	return &ProjectDataSource{}
}

// ProjectDataSource defines the data source implementation.
type ProjectDataSource struct {
	client *NeptuneClient
}

// ProjectDataSourceModel describes the data source data model.
type ProjectDataSourceModel struct {
	Id           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	ProjectName  types.String `tfsdk:"project_name"`
	ProjectKey   types.String `tfsdk:"project_key"`
	Visibility   types.String `tfsdk:"visibility"`
	Avatar       types.String `tfsdk:"avatar"`
	AvatarSource types.String `tfsdk:"avatar_source"`
	Color        types.String `tfsdk:"color"`
}

func (d *ProjectDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (d *ProjectDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Neptune project data source",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Project identifier (UUID). Either `id` or `project_name` must be specified.",
			},
			"name": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project name",
			},
			"description": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project description",
			},
			"project_name": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Project name for lookup. Either `id` or `project_name` must be specified.",
			},
			"project_key": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project key (unique within organization)",
			},
			"visibility": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project visibility: `priv` (private), `pub` (public), or `workspace` (workspace)",
			},
			"avatar": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project avatar URL",
			},
			"avatar_source": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Avatar source: `default`, `thirdParty`, `user`, `inherited`, or `unicode`",
			},
			"color": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project color",
			},
		},
	}
}

func (d *ProjectDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*NeptuneClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *NeptuneClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.client = client
}

func (d *ProjectDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data ProjectDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Initialize computed fields to null to ensure proper state
	data.Description = types.StringNull()
	data.Avatar = types.StringNull()
	data.AvatarSource = types.StringNull()
	data.Color = types.StringNull()

	// Validate that we have either ID or project_name
	hasId := !data.Id.IsNull() && data.Id.ValueString() != ""
	hasProjectName := !data.ProjectName.IsNull() && data.ProjectName.ValueString() != ""

	if !hasId && !hasProjectName {
		resp.Diagnostics.AddError(
			"Invalid Configuration",
			"Either 'id' or 'project_name' must be specified.",
		)
		return
	}

	var endpoint string
	var projectIdentifier string

	if hasId {
		projectIdentifier = data.Id.ValueString()
	} else {
		// Construct project identifier from workspace/project_name
		projectIdentifier = fmt.Sprintf("%s/%s", d.client.workspace, data.ProjectName.ValueString())
	}

	// Get project details using IAAC endpoint
	endpoint = fmt.Sprintf("/api/backend/v1/iaac/projects?projectIdentifier=%s", url.QueryEscape(projectIdentifier))
	httpResp, err := d.client.Get(ctx, endpoint)
	if err != nil {
		// Check if this is a 404 error (resource not found)
		if neptuneErr, ok := err.(*NeptuneError); ok && neptuneErr.StatusCode == 404 {
			resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Project not found: %s", projectIdentifier))
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read project, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == 404 {
		resp.Diagnostics.AddError("Not Found", fmt.Sprintf("Project not found: %s", projectIdentifier))
		return
	}

	if httpResp.StatusCode != 200 {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read project, got status: %d", httpResp.StatusCode))
		return
	}

	var projectResp ProjectResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&projectResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse project response: %s", err))
		return
	}

	// Update the model with response data
	d.updateModelFromResponse(&data, &projectResp)

	tflog.Trace(ctx, "read a project data source")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

// updateModelFromResponse updates the Terraform model from the API response.
func (d *ProjectDataSource) updateModelFromResponse(data *ProjectDataSourceModel, resp *ProjectResponse) {
	data.Id = types.StringValue(resp.Id)
	data.Name = types.StringValue(resp.Name)
	data.ProjectKey = types.StringValue(resp.ProjectKey)
	data.Visibility = types.StringValue(resp.Visibility)

	if resp.Description != "" {
		data.Description = types.StringValue(resp.Description)
	} else {
		data.Description = types.StringNull()
	}

	if resp.AvatarUrl != "" {
		data.Avatar = types.StringValue(resp.AvatarUrl)
	} else {
		data.Avatar = types.StringNull()
	}

	if resp.AvatarSource != "" {
		data.AvatarSource = types.StringValue(resp.AvatarSource)
	} else {
		data.AvatarSource = types.StringNull()
	}

	if resp.Color != "" {
		data.Color = types.StringValue(resp.Color)
	} else {
		data.Color = types.StringNull()
	}
}
