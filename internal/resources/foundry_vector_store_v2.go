// Copyright (c) Your Org
// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/andrewCluey/terraform-provider-azurefoundry/internal/client"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/mapvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &FoundryVectorStoreV2Resource{}

func NewFoundryVectorStoreV2Resource() resource.Resource {
	return &FoundryVectorStoreV2Resource{}
}

type FoundryVectorStoreV2Resource struct {
	client *client.FoundryClient
}

type FoundryVectorStoreV2ResourceModel struct {
	// Computed
	ID           types.String `tfsdk:"id"`
	CreatedAt    types.Int64  `tfsdk:"created_at"`
	Status       types.String `tfsdk:"status"`
	UsageBytes   types.Int64  `tfsdk:"usage_bytes"`
	FilesTotal   types.Int64  `tfsdk:"files_total"`
	FilesReady   types.Int64  `tfsdk:"files_ready"`
	FilesFailed  types.Int64  `tfsdk:"files_failed"`

	// Optional
	Name     types.String `tfsdk:"name"`
	FileIDs  types.List   `tfsdk:"file_ids"`
	Metadata types.Map    `tfsdk:"metadata"`

	// Optional expiry
	ExpiryAnchor types.String `tfsdk:"expiry_anchor"`
	ExpiryDays   types.Int64  `tfsdk:"expiry_days"`
}

func (r *FoundryVectorStoreV2Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_vector_store_v2"
}

func (r *FoundryVectorStoreV2Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: `
Manages an Azure AI Foundry Vector Store.

A vector store processes and indexes uploaded files so they can be searched by
the ` + "`file_search`" + ` tool on an agent. Attach the vector store to an agent using
` + "`file_search_vector_store_ids`" + ` in the ` + "`azurefoundry_agent`" + ` resource.

When files are provided via ` + "`file_ids`" + `, Terraform will wait for the vector
store to finish indexing them before marking the resource as created.
`,
		Attributes: map[string]schema.Attribute{
			// ── Computed ───────────────────────────────────────────────────────
			"id": schema.StringAttribute{
				MarkdownDescription: "The vector store ID assigned by the Foundry service.",
				Computed:            true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"created_at": schema.Int64Attribute{
				MarkdownDescription: "Unix timestamp when the vector store was created.",
				Computed:            true,
				PlanModifiers: []planmodifier.Int64{
					int64planmodifier.UseStateForUnknown(),
				},
			},
			"status": schema.StringAttribute{
				MarkdownDescription: "Current status of the vector store: `in_progress`, `completed`, or `expired`.",
				Computed:            true,
			},
			"usage_bytes": schema.Int64Attribute{
				MarkdownDescription: "Storage used by this vector store in bytes.",
				Computed:            true,
			},
			"files_total": schema.Int64Attribute{
				MarkdownDescription: "Total number of files associated with this vector store.",
				Computed:            true,
			},
			"files_ready": schema.Int64Attribute{
				MarkdownDescription: "Number of files successfully indexed.",
				Computed:            true,
			},
			"files_failed": schema.Int64Attribute{
				MarkdownDescription: "Number of files that failed to index.",
				Computed:            true,
			},

			// ── Optional ───────────────────────────────────────────────────────
			"name": schema.StringAttribute{
				MarkdownDescription: "Display name for the vector store.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.LengthAtMost(256),
				},
			},
			"file_ids": schema.ListAttribute{
				MarkdownDescription: "List of file IDs (from `azurefoundry_file`) to index in this vector store.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Validators: []validator.List{
					listvalidator.SizeBetween(0, 500),
					listvalidator.ValueStringsAre(stringvalidator.LengthAtLeast(1)),
				},
				PlanModifiers: []planmodifier.List{
					listplanmodifier.RequiresReplace(),
				},
			},
			"metadata": schema.MapAttribute{
				MarkdownDescription: "Up to 16 key/value string pairs.",
				Optional:            true,
				Computed:            true,
				ElementType:         types.StringType,
				Validators: []validator.Map{
					mapvalidator.SizeBetween(0, 16),
					mapvalidator.KeysAre(stringvalidator.LengthAtMost(64)),
				},
			},
			"expiry_anchor": schema.StringAttribute{
				MarkdownDescription: "The expiry anchor. Currently only `last_active_at` is supported.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.String{
					stringvalidator.OneOf("last_active_at"),
				},
			},
			"expiry_days": schema.Int64Attribute{
				MarkdownDescription: "Number of days after the anchor before the vector store expires.",
				Optional:            true,
				Computed:            true,
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
				},
			},
		},
	}
}

func (r *FoundryVectorStoreV2Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FoundryVectorStoreV2Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FoundryVectorStoreV2ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := modelToCreateVectorStoreV2Request(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Foundry vector store", map[string]interface{}{"name": apiReq.Name})

	vsResp, err := r.client.CreateVectorStoreV2(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating vector store", err.Error())
		return
	}

	// If files were provided, wait for indexing to complete.
	if len(apiReq.FileIDs) > 0 {
		tflog.Debug(ctx, "Waiting for vector store to finish indexing", map[string]interface{}{"id": vsResp.ID})
		vsResp, err = r.client.WaitForVectorStore(ctx, vsResp.ID)
		if err != nil {
			resp.Diagnostics.AddError("Error waiting for vector store", err.Error())
			return
		}
	}

	resp.Diagnostics.Append(vectorStoreV2ResponseToModel(vsResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FoundryVectorStoreV2Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FoundryVectorStoreV2ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	vsResp, err := r.client.GetVectorStoreV2(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			tflog.Warn(ctx, "Vector store no longer exists, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading vector store", err.Error())
		return
	}

	resp.Diagnostics.Append(vectorStoreV2ResponseToModel(vsResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *FoundryVectorStoreV2Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FoundryVectorStoreV2ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	var state FoundryVectorStoreV2ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiReq := client.UpdateVectorStoreRequest{
		Name: plan.Name.ValueString(),
	}

	if !plan.Metadata.IsNull() && !plan.Metadata.IsUnknown() {
		meta := make(map[string]types.String, len(plan.Metadata.Elements()))
		resp.Diagnostics.Append(plan.Metadata.ElementsAs(ctx, &meta, false)...)
		metadata := make(map[string]string, len(meta))
		for k, v := range meta {
			metadata[k] = v.ValueString()
		}
		apiReq.Metadata = metadata
	}

	if !plan.ExpiryAnchor.IsNull() && !plan.ExpiryDays.IsNull() {
		apiReq.ExpiresAfter = &client.VectorStoreExpirationPolicy{
			Anchor: plan.ExpiryAnchor.ValueString(),
			Days:   plan.ExpiryDays.ValueInt64(),
		}
	}

	tflog.Debug(ctx, "Updating vector store", map[string]interface{}{"id": state.ID.ValueString()})

	vsResp, err := r.client.UpdateVectorStoreV2(ctx, state.ID.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating vector store", err.Error())
		return
	}

	resp.Diagnostics.Append(vectorStoreV2ResponseToModel(vsResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FoundryVectorStoreV2Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FoundryVectorStoreV2ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting vector store", map[string]interface{}{"id": state.ID.ValueString()})

	_, err := r.client.DeleteVectorStoreV2(ctx, state.ID.ValueString())
	if err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting vector store", err.Error())
		return
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Mapping helpers
// ─────────────────────────────────────────────────────────────────────────────

func modelToCreateVectorStoreV2Request(ctx context.Context, m FoundryVectorStoreV2ResourceModel) (client.CreateVectorStoreRequest, diag.Diagnostics) {
	var diags diag.Diagnostics
	req := client.CreateVectorStoreRequest{
		Name: m.Name.ValueString(),
	}

	if !m.FileIDs.IsNull() && !m.FileIDs.IsUnknown() {
		var fileIDs []string
		diags.Append(m.FileIDs.ElementsAs(ctx, &fileIDs, false)...)
		req.FileIDs = fileIDs
	}

	if !m.Metadata.IsNull() && !m.Metadata.IsUnknown() {
		meta := make(map[string]types.String, len(m.Metadata.Elements()))
		diags.Append(m.Metadata.ElementsAs(ctx, &meta, false)...)
		metadata := make(map[string]string, len(meta))
		for k, v := range meta {
			metadata[k] = v.ValueString()
		}
		req.Metadata = metadata
	}

	if !m.ExpiryAnchor.IsNull() && !m.ExpiryDays.IsNull() {
		req.ExpiresAfter = &client.VectorStoreExpirationPolicy{
			Anchor: m.ExpiryAnchor.ValueString(),
			Days:   m.ExpiryDays.ValueInt64(),
		}
	}

	return req, diags
}

func vectorStoreV2ResponseToModel(r *client.VectorStoreResponse, m *FoundryVectorStoreV2ResourceModel) diag.Diagnostics {
	var diags diag.Diagnostics

	m.ID = types.StringValue(r.ID)
	m.CreatedAt = types.Int64Value(r.CreatedAt)
	m.Status = types.StringValue(string(r.Status))
	m.UsageBytes = types.Int64Value(r.UsageBytes)
	m.FilesTotal = types.Int64Value(r.FileCounts.Total)
	m.FilesReady = types.Int64Value(r.FileCounts.Completed)
	m.FilesFailed = types.Int64Value(r.FileCounts.Failed)
	m.Name = types.StringValue(r.Name)

	if r.ExpiresAfter != nil {
		m.ExpiryAnchor = types.StringValue(r.ExpiresAfter.Anchor)
		m.ExpiryDays = types.Int64Value(r.ExpiresAfter.Days)
	} else {
		m.ExpiryAnchor = types.StringNull()
		m.ExpiryDays = types.Int64Null()
	}

	if r.Metadata != nil {
		metaAttrs := make(map[string]attr.Value, len(r.Metadata))
		for k, v := range r.Metadata {
			metaAttrs[k] = types.StringValue(v)
		}
		metaMap, d := types.MapValue(types.StringType, metaAttrs)
		diags.Append(d...)
		m.Metadata = metaMap
	} else {
		m.Metadata = types.MapValueMust(types.StringType, map[string]attr.Value{})
	}

	return diags
}