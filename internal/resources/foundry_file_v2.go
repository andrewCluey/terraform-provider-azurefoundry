// Copyright (c) Your Org
// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"fmt"
	"os"

	"github.com/andrewCluey/terraform-provider-azurefoundry/internal/client"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &FoundryFileV2Resource{}

func NewFoundryFileV2Resource() resource.Resource {
	return &FoundryFileV2Resource{}
}

type FoundryFileV2Resource struct {
	client *client.FoundryClient
}

// FoundryFileV2ResourceModel is the Terraform state model for azurefoundry_file.
type FoundryFileV2ResourceModel struct {
	// Computed
	ID        types.String `tfsdk:"id"`
	CreatedAt types.Int64  `tfsdk:"created_at"`
	Bytes     types.Int64  `tfsdk:"bytes"`
	Filename  types.String `tfsdk:"filename"`

	// Required
	// source is the local path to the file to upload.
	Source types.String `tfsdk:"source"`

	// Optional
	// purpose defaults to "assistants" which is correct for file_search and code_interpreter.
	Purpose types.String `tfsdk:"purpose"`
}

func (r *FoundryFileV2Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_file_v2"
}

func (r *FoundryFileV2Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Uploads a local file to the Azure AI Foundry project.

The resulting file ID can be used with ` + "`azurefoundry_vector_store`" + ` (for
` + "`file_search`" + `) or passed directly in ` + "`code_interpreter_file_ids`" + ` on an
` + "`azurefoundry_agent`" + `.

~> **Note** Files are immutable in the Foundry API. If you change ` + "`source`" + `,
Terraform will delete the old file and upload a new one (replace).
`,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				MarkdownDescription: "The file ID assigned by the Foundry service.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Unix timestamp when the file was uploaded.",
				Computed:            true,
			},
			"bytes": schema.Int64Attribute{
				MarkdownDescription: "Size of the uploaded file in bytes.",
				Computed:            true,
			},
			"filename": schema.StringAttribute{
				MarkdownDescription: "The filename as stored in the Foundry service.",
				Computed:            true,
			},
			"source": schema.StringAttribute{
				MarkdownDescription: "Path to the local file to upload. Changing this forces a new file to be uploaded.",
				Required:            true,
				PlanModifiers: []planmodifier.String{
					// Any change to source forces replacement.
					stringplanmodifier.RequiresReplace(),
				},
			},
			"purpose": schema.StringAttribute{
				MarkdownDescription: "The intended use of the file. Currently only `assistants` is supported. Defaults to `assistants`.",
				Optional:            true,
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
				Validators: []validator.String{
					stringvalidator.OneOf("assistants"),
				},
			},
		},
	}
}

func (r *FoundryFileV2Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	apiClient, ok := req.ProviderData.(*client.FoundryClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected provider data type",
			fmt.Sprintf("Expected *client.FoundryClient, got %T.", req.ProviderData),
		)
		return
	}
	r.client = apiClient
}

func (r *FoundryFileV2Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FoundryFileV2ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	source := plan.Source.ValueString()
	fileData, err := os.ReadFile(source)
	if err != nil {
		resp.Diagnostics.AddError(
			"Cannot read source file",
			fmt.Sprintf("Failed to read %q: %s", source, err.Error()),
		)
		return
	}

	purpose := client.FilePurposeAssistants
	if !plan.Purpose.IsNull() && !plan.Purpose.IsUnknown() {
		purpose = client.FilePurpose(plan.Purpose.ValueString())
	}

	tflog.Debug(ctx, "Uploading file to Foundry", map[string]interface{}{"source": source})

	fileResp, err := r.client.UploadFile(ctx, source, fileData, purpose)
	if err != nil {
		resp.Diagnostics.AddError("Error uploading file", err.Error())
		return
	}

	plan.ID = types.StringValue(fileResp.ID)
	plan.CreatedAt = types.Int64Value(fileResp.CreatedAt)
	plan.Bytes = types.Int64Value(fileResp.Bytes)
	plan.Filename = types.StringValue(fileResp.Filename)
	plan.Purpose = types.StringValue(string(fileResp.Purpose))

	tflog.Debug(ctx, "Uploaded file", map[string]interface{}{"id": fileResp.ID})
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FoundryFileV2Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FoundryFileV2ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	fileResp, err := r.client.GetFile(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			tflog.Warn(ctx, "Foundry file no longer exists, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Foundry file", err.Error())
		return
	}

	state.ID = types.StringValue(fileResp.ID)
	state.CreatedAt = types.Int64Value(fileResp.CreatedAt)
	state.Bytes = types.Int64Value(fileResp.Bytes)
	state.Filename = types.StringValue(fileResp.Filename)
	state.Purpose = types.StringValue(string(fileResp.Purpose))

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is not needed — any change to source forces a replacement.
// Purpose also forces replacement. So this should never be called.
func (r *FoundryFileV2Resource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError(
		"Update not supported",
		"azurefoundry_file does not support in-place updates. Change 'source' to upload a new file.",
	)
}

func (r *FoundryFileV2Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FoundryFileV2ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Foundry file", map[string]interface{}{"id": state.ID.ValueString()})

	_, err := r.client.DeleteFile(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting Foundry file", err.Error())
		return
	}
}