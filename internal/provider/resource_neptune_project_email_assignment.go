package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &ProjectEmailAssignmentResource{}
var _ resource.ResourceWithImportState = &ProjectEmailAssignmentResource{}

func NewProjectEmailAssignmentResource() resource.Resource {
	return &ProjectEmailAssignmentResource{}
}

type ProjectEmailAssignmentResource struct {
	client *NeptuneClient
}

type ProjectEmailAssignmentResourceModel struct {
	Id      types.String `tfsdk:"id"`
	Project types.String `tfsdk:"project"`
	Email   types.String `tfsdk:"email"`
	Role    types.String `tfsdk:"role"`
}

// ProjectMemberUpdateRequest represents the request payload for updating a project member.
type ProjectMemberUpdateRequest struct {
	Role string `json:"role"`
}

// ProjectMember represents a project member.
type ProjectMember struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// ProjectMembersResponse represents the API response for listing project members.
type ProjectMembersResponse struct {
	Members []ProjectMember `json:"members"`
}

func (r *ProjectEmailAssignmentResource) Metadata(ctx context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_project_email_assignment"
}

func (r *ProjectEmailAssignmentResource) Schema(ctx context.Context, req resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages assignment of a user to a Neptune project.",

		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:            true,
				MarkdownDescription: "The ID of the project member assignment, in the format `<project_id>/<email>`.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"project": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The identifier of the project to which the user will be assigned.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"email": schema.StringAttribute{
				Required:            true,
				MarkdownDescription: "The email address of the user to assign to the project.",
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
			"role": schema.StringAttribute{
				Optional:            true,
				Computed:            true,
				Default:             stringdefault.StaticString("member"),
				MarkdownDescription: "The role to assign to the user within the project. Must be one of `owner` or `member`. Defaults to `member`.",
				Validators: []validator.String{
					stringvalidator.OneOf("owner", "member"),
				},
			},
		},
	}
}

func (r *ProjectEmailAssignmentResource) Configure(ctx context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *ProjectEmailAssignmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var data ProjectEmailAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectIdentifier := data.Project.ValueString()
	email := data.Email.ValueString()

	// Use the same request structure as Update since PUT now handles both create and update
	memberReq := ProjectMemberUpdateRequest{
		Role: data.Role.ValueString(),
	}

	// Use PUT method with email as query parameter (same as Update)
	endpoint := fmt.Sprintf("/api/backend/v1/iaac/projects/members?projectIdentifier=%s&email=%s", url.QueryEscape(projectIdentifier), url.QueryEscape(email))
	httpResp, err := r.client.Put(ctx, endpoint, memberReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to add project member, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to add project member, got status: %d", httpResp.StatusCode))
		return
	}

	var memberResp ProjectMember
	if err := json.NewDecoder(httpResp.Body).Decode(&memberResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse project member response: %s", err))
		return
	}

	data.Id = types.StringValue(fmt.Sprintf("%s/%s", projectIdentifier, email))
	data.Email = types.StringValue(memberResp.Email)
	data.Role = types.StringValue(memberResp.Role)

	tflog.Trace(ctx, "Created a project member assignment resource")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectEmailAssignmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var data ProjectEmailAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectIdentifier := data.Project.ValueString()
	email := data.Email.ValueString()

	endpoint := fmt.Sprintf("/api/backend/v1/iaac/projects/members?projectIdentifier=%s", url.QueryEscape(projectIdentifier))
	httpResp, err := r.client.Get(ctx, endpoint)
	if err != nil {
		if neptuneErr, ok := err.(*NeptuneError); ok && neptuneErr.StatusCode == http.StatusNotFound {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to list project members, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode == http.StatusNotFound {
		resp.State.RemoveResource(ctx)
		return
	}

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to list project members, got status: %d", httpResp.StatusCode))
		return
	}

	var listResp ProjectMembersResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&listResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse project members list response: %s", err))
		return
	}

	var foundMember *ProjectMember
	for i := range listResp.Members {
		if listResp.Members[i].Email == email {
			foundMember = &listResp.Members[i]
			break
		}
	}

	if foundMember == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	data.Project = types.StringValue(projectIdentifier)
	data.Email = types.StringValue(foundMember.Email)
	data.Role = types.StringValue(foundMember.Role)
	data.Id = types.StringValue(fmt.Sprintf("%s/%s", projectIdentifier, email))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectEmailAssignmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var data ProjectEmailAssignmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectIdentifier := data.Project.ValueString()
	email := data.Email.ValueString()

	updateReq := ProjectMemberUpdateRequest{
		Role: data.Role.ValueString(),
	}

	endpoint := fmt.Sprintf("/api/backend/v1/iaac/projects/members?projectIdentifier=%s&email=%s", url.QueryEscape(projectIdentifier), url.QueryEscape(email))
	httpResp, err := r.client.Put(ctx, endpoint, updateReq)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to update project member, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to update project member, got status: %d", httpResp.StatusCode))
		return
	}

	var memberResp ProjectMember
	if err := json.NewDecoder(httpResp.Body).Decode(&memberResp); err != nil {
		resp.Diagnostics.AddError("Parse Error", fmt.Sprintf("Unable to parse project member response: %s", err))
		return
	}

	data.Role = types.StringValue(memberResp.Role)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func (r *ProjectEmailAssignmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var data ProjectEmailAssignmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	projectIdentifier := data.Project.ValueString()
	email := data.Email.ValueString()

	endpoint := fmt.Sprintf("/api/backend/v1/iaac/projects/members?projectIdentifier=%s&email=%s", url.QueryEscape(projectIdentifier), url.QueryEscape(email))
	httpResp, err := r.client.Delete(ctx, endpoint)
	if err != nil {
		resp.Diagnostics.AddError("Client Error", fmt.Sprintf("Unable to delete project member, got error: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK && httpResp.StatusCode != http.StatusNotFound {
		resp.Diagnostics.AddError("API Error", fmt.Sprintf("Unable to delete project member, got status: %d", httpResp.StatusCode))
		return
	}
}

func (r *ProjectEmailAssignmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	idParts := strings.Split(req.ID, "/")
	if len(idParts) != 2 || idParts[0] == "" || idParts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected Import Identifier",
			fmt.Sprintf("Expected import identifier with format: `<project_identifier>/<email>`. Got: %q", req.ID),
		)
		return
	}

	projectIdentifier := idParts[0]
	email := idParts[1]

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("project"), projectIdentifier)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("email"), email)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
}
