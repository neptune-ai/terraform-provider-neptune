package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Ensure provider defined types fully satisfy framework interfaces.
var _ resource.Resource = &ProjectResource{}
var _ resource.ResourceWithImportState = &ProjectResource{}

func NewProjectResource() resource.Resource {
	return &ProjectResource{}
}

// ProjectResource defines the resource implementation.
type ProjectResource struct {
	client *NeptuneClient
}

// ProjectResourceModel describes the resource data model.
type ProjectResourceModel struct {
	Id           types.String `tfsdk:"id"`
	Name         types.String `tfsdk:"name"`
	Description  types.String `tfsdk:"description"`
	ProjectKey   types.String `tfsdk:"project_key"`
	Visibility   types.String `tfsdk:"visibility"`
	Avatar       types.String `tfsdk:"avatar"`
	AvatarSource types.String `tfsdk:"avatar_source"`
	Color        types.String `tfsdk:"color"`
}

// NewProjectRequest represents the request payload for creating a project.
type NewProjectRequest struct {
	Name                   string `json:"name"`
	Description            string `json:"description,omitempty"`
	OrganizationIdentifier string `json:"organizationIdentifier"`
	ProjectKey             string `json:"projectKey,omitempty"`
	Visibility             string `json:"visibility,omitempty"`
	AvatarUrl              string `json:"avatarUrl,omitempty"`
	AvatarSource           string `json:"avatarSource,omitempty"`
	Color                  string `json:"displayClass,omitempty"`
}

// ProjectUpdateRequest represents the request payload for updating a project.
type ProjectUpdateRequest struct {
	Name         *string `json:"name,omitempty"`
	Description  *string `json:"description,omitempty"`
	Visibility   *string `json:"visibility,omitempty"`
	AvatarUrl    *string `json:"avatarUrl,omitempty"`
	AvatarSource *string `json:"avatarSource,omitempty"`
	Color        *string `json:"displayClass,omitempty"`
}

// ProjectResponse represents the API response for project operations.
type ProjectResponse struct {
	Id               string `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	OrganizationName string `json:"organizationName"`
	OrganizationId   string `json:"organizationId"`
	ProjectKey       string `json:"projectKey"`
	Visibility       string `json:"visibility"`
	AvatarUrl        string `json:"avatarUrl"`
	AvatarSource     string `json:"avatarSource"`
	Color            string `json:"displayClass"`
	CodeAccess       string `json:"codeAccess"`
	Archived         bool   `json:"archived"`
	Featured         bool   `json:"featured"`
	BackgroundUrl    string `json:"backgroundUrl"`
}

func (r *ProjectResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project"
}

func (r *ProjectResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Neptune project resource",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "Project identifier (UUID)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "Project name",
			},
			"description": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Project description",
			},
			"project_key": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				MarkdownDescription: "Project key (unique within organization)",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"visibility": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("priv"),
				MarkdownDescription: "Project visibility: `priv` (private), `pub` (public), or `workspace` (workspace). Default is `priv`.",
				Validators: []validator.String{
					stringvalidator.OneOf("priv", "pub", "workspace"),
				},
			},
			"avatar": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Project avatar URL (must be HTTP or HTTPS URL if provided)",
			},
			"avatar_source": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("default"),
				MarkdownDescription: "Avatar source: `default`, `thirdParty`, `user`, `inherited`, or `unicode`",
				Validators: []validator.String{
					stringvalidator.OneOf("default", "thirdParty", "user", "inherited", "unicode"),
				},
			},
			"color": schema.StringAttribute{
				Optional:            true,
				MarkdownDescription: "Project color or display class",
			},
		},
	}
}

func (r *ProjectResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*NeptuneClient)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Resource Configure Type",
			fmt.Sprintf("Expected *NeptuneClient, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *ProjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Prepare the request payload - use workspace from provider configuration
	projectReq := NewProjectRequest{
		Name:                   data.Name.ValueString(),
		OrganizationIdentifier: r.client.workspace,
	}

	if !data.Description.IsNull() {
		projectReq.Description = data.Description.ValueString()
	}
	if !data.ProjectKey.IsNull() {
		projectReq.ProjectKey = data.ProjectKey.ValueString()
	}
	if !data.Visibility.IsNull() {
		projectReq.Visibility = data.Visibility.ValueString()
	}
	if !data.Avatar.IsNull() {
		projectReq.AvatarUrl = data.Avatar.ValueString()
	}
	if !data.AvatarSource.IsNull() {
		projectReq.AvatarSource = data.AvatarSource.ValueString()
	}
	if !data.Color.IsNull() {
		projectReq.Color = data.Color.ValueString()
	}

	// Create the project using IAAC endpoint
	httpResp, err := r.client.Post(ctx, "/api/backend/v1/iaac/projects", projectReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to create project, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to create project, got status: %d", httpResp.StatusCode))
		return
	}

	var projectResp ProjectResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&projectResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse project response: %s", err))
		return
	}

	// Update the model with response data
	r.updateModelFromResponse(&data, &projectResp)

	tflog.Trace(ctx, "created a project resource")

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Get project details using IAAC endpoint
	endpoint := fmt.Sprintf("/api/backend/v1/iaac/projects?projectIdentifier=%s", url.QueryEscape(data.Id.ValueString()))
	httpResp, err := r.client.Get(ctx, endpoint)
	if err != nil {
		// Check if this is a 404 error (resource not found)
		if neptuneErr, ok := err.(*NeptuneError); ok && neptuneErr.StatusCode == 404 {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to read project, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to read project, got status: %d", httpResp.StatusCode))
		return
	}

	var projectResp ProjectResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&projectResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse project response: %s", err))
		return
	}

	// Update the model with response data
	r.updateModelFromResponse(&data, &projectResp)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectResourceModel

	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Prepare the update request payload
	updateReq := ProjectUpdateRequest{}

	// Always include required fields
	if !data.Name.IsNull() {
		name := data.Name.ValueString()
		updateReq.Name = &name
	}

	// For description, always include it - send empty string to clear if null
	desc := data.Description.ValueString()
	updateReq.Description = &desc

	// Always include fields with defaults to ensure they're not lost
	vis := data.Visibility.ValueString()
	updateReq.Visibility = &vis

	src := data.AvatarSource.ValueString()
	updateReq.AvatarSource = &src

	// Other optional fields - always include to allow clearing with empty string
	avatarUrl := data.Avatar.ValueString()
	updateReq.AvatarUrl = &avatarUrl

	class := data.Color.ValueString()
	updateReq.Color = &class

	// Update the project using IAAC endpoint
	endpoint := fmt.Sprintf("/api/backend/v1/iaac/projects?projectIdentifier=%s", url.QueryEscape(data.Id.ValueString()))
	httpResp, err := r.client.Put(ctx, endpoint, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update project, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update project, got status: %d", httpResp.StatusCode))
		return
	}

	var projectResp ProjectResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&projectResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse project response: %s", err))
		return
	}

	// Update only the fields that can change during update - preserve computed fields
	r.updateModelFromResponseForUpdate(&data, &projectResp)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectResourceModel

	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Delete the project using IAAC endpoint
	endpoint := fmt.Sprintf("/api/backend/v1/iaac/projects?projectIdentifier=%s", url.QueryEscape(data.Id.ValueString()))
	httpResp, err := r.client.Delete(ctx, endpoint)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete project, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNotFound {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete project, got status: %d", httpResp.StatusCode))
		return
	}
}

func (r *ProjectResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	projectIdentifier := fmt.Sprintf("%s/%s", r.client.workspace, req.ID)

	// Get project details to validate and get the actual UUID using IAAC endpoint
	endpoint := fmt.Sprintf("/api/backend/v1/iaac/projects?projectIdentifier=%s", url.QueryEscape(projectIdentifier))
	httpResp, err := r.client.Get(ctx, endpoint)
	if err != nil {
		resp.Diagnostics.AddError("Import Error", fmt.Sprintf("Unable to find project '%s': %s", req.ID, err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.Diagnostics.AddError("Import Error", fmt.Sprintf("Project '%s' not found", req.ID))
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("Import Error", fmt.Sprintf("Unable to read project '%s', got status: %d", req.ID, httpResp.StatusCode))
		return
	}

	var projectResp ProjectResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&projectResp); err != nil {
		resp.Diagnostics.AddError("Import Error", fmt.Sprintf("Unable to parse project response: %s", err))
		return
	}

	// Set the actual project UUID as the ID
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), projectResp.Id)...)
}

// updateModelFromResponse updates the Terraform model from the API response.
func (r *ProjectResource) updateModelFromResponse(data *ProjectResourceModel, resp *ProjectResponse) {
	// Always update ID (for Create operations)
	data.Id = types.StringValue(resp.Id)

	// Update managed fields that can change
	data.Name = types.StringValue(resp.Name)
	data.ProjectKey = types.StringValue(resp.ProjectKey)
	data.Visibility = types.StringValue(resp.Visibility)

	// Update optional fields, converting empty strings to null to match Terraform expectations
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

// updateModelFromResponseForUpdate updates only fields that can change during update operations.
// This prevents "known after apply" for computed fields that don't actually change.
func (r *ProjectResource) updateModelFromResponseForUpdate(data *ProjectResourceModel, resp *ProjectResponse) {
	// Update managed fields that can change during update
	data.Name = types.StringValue(resp.Name)
	data.Visibility = types.StringValue(resp.Visibility)

	// Update optional fields, converting empty strings to null
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

	// Always ensure computed fields have known values (even if they don't change)
	// This prevents "unknown value" errors after apply
	data.Id = types.StringValue(resp.Id)
	data.ProjectKey = types.StringValue(resp.ProjectKey)
}
