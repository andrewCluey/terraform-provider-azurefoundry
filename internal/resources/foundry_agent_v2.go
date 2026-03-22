// Copyright (c) Your Org
// SPDX-License-Identifier: MPL-2.0

package resources

import (
	"context"
	"fmt"

	"github.com/andrewCluey/terraform-provider-azurefoundry/internal/client"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ resource.Resource = &FoundryAgentV2Resource{}
var _ resource.ResourceWithImportState = &FoundryAgentV2Resource{}

func NewFoundryAgentV2Resource() resource.Resource {
	return &FoundryAgentV2Resource{}
}

type FoundryAgentV2Resource struct {
	client *client.FoundryClient
}

type FoundryAgentV2ResourceModel struct {
    ID          types.String `tfsdk:"id"`
    Name        types.String `tfsdk:"name"`
    Description types.String `tfsdk:"description"`
    CreatedAt   types.Int64  `tfsdk:"created_at"`
    Version     types.String `tfsdk:"version"`
    Metadata    types.Map    `tfsdk:"metadata"`
    Kind        types.String `tfsdk:"kind"`
    Model       types.String `tfsdk:"model"`
    Instructions types.String `tfsdk:"instructions"`
	Tools        types.List   `tfsdk:"tools"`
}
type toolModelV2 struct {
    Type           types.String `tfsdk:"type"`
    VectorStoreIDs types.List   `tfsdk:"vector_store_ids"`
    MaxNumResults  types.Int64  `tfsdk:"max_num_results"`
}


func (r *FoundryAgentV2Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_v2"
}

func (r *FoundryAgentV2Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "Manages an Azure AI Foundry Agent.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.UseStateForUnknown(),
				},
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
                	stringplanmodifier.RequiresReplace(),
				},
			},
			"description": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
			"created_at": schema.Int64Attribute{
				Computed: true,
			},
			"version": schema.StringAttribute{
				Computed: true,
			},
			"metadata": schema.MapAttribute{
				Optional:    true,
				Computed:    true,
				ElementType: types.StringType,
			},
			"kind": schema.StringAttribute{
				Required: true,
				Validators: []validator.String{
					stringvalidator.OneOf("prompt", "hosted", "container_app", "workflow"),
				},
			},
			"model": schema.StringAttribute{
				Required: true,
			},
			"instructions": schema.StringAttribute{
				Optional: true,
				Computed: true,
			},
		},
		Blocks: map[string]schema.Block{
			"tools": schema.ListNestedBlock{
				MarkdownDescription: "Tools enabled for the agent.",
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"type": schema.StringAttribute{
							Required: true,
							Validators: []validator.String{
								stringvalidator.OneOf("file_search"),
							},
						},
						"vector_store_ids": schema.ListAttribute{
							Optional:    true,
                    		ElementType: types.StringType,
						},
						"max_num_results": schema.Int64Attribute{
							Optional: true,
						},
					},
				},
			},
		},
	}
}

func (r *FoundryAgentV2Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *FoundryAgentV2Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan FoundryAgentV2ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := modelToCreateV2Request(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Creating Foundry agent", map[string]interface{}{"name": apiReq.Name, "model": apiReq.Definition.Model})

	agentResp, err := r.client.CreateAgentV2(ctx, apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error creating Foundry agent", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToV2Model(ctx, agentResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FoundryAgentV2Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state FoundryAgentV2ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	agentResp, err := r.client.GetAgentV2(ctx, state.Name.ValueString())
	if err != nil {
		if isNotFound(err) {
			tflog.Warn(ctx, "Foundry agent no longer exists, removing from state")
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Error reading Foundry agent", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToV2Model(ctx, agentResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *FoundryAgentV2Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan FoundryAgentV2ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)

	var state FoundryAgentV2ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	apiReq, diags := modelToUpdateV2Request(ctx, plan)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Updating Foundry agent", map[string]interface{}{"id": state.Name.ValueString()})

	agentResp, err := r.client.UpdateAgentV2(ctx, state.Name.ValueString(), apiReq)
	if err != nil {
		resp.Diagnostics.AddError("Error updating Foundry agent", err.Error())
		return
	}

	resp.Diagnostics.Append(responseToV2Model(ctx, agentResp, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *FoundryAgentV2Resource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state FoundryAgentV2ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Deleting Foundry agent", map[string]interface{}{"id": state.Name.ValueString()})

	_, err := r.client.DeleteAgentV2(ctx, state.Name.ValueString())
	if err != nil {
		if isNotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Error deleting Foundry agent", err.Error())
		return
	}
}

func (r *FoundryAgentV2Resource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	agentResp, err := r.client.GetAgentV2(ctx, req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Error importing Foundry agent", err.Error())
		return
	}

	var state FoundryAgentV2ResourceModel
	resp.Diagnostics.Append(responseToV2Model(ctx, agentResp, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// ─────────────────────────────────────────────────────────────────────────────
// Mapping helpers
// ─────────────────────────────────────────────────────────────────────────────

func modelToCreateV2Request(ctx context.Context, m FoundryAgentV2ResourceModel) (client.CreateAgentV2Request, diag.Diagnostics) {
    var diags diag.Diagnostics
    req := client.CreateAgentV2Request{
        Name:        m.Name.ValueString(),
        Description: m.Description.ValueString(),
        Definition: client.AgentDefinitionV2{
            Kind:         m.Kind.ValueString(),
            Model:        m.Model.ValueString(),
            Instructions: m.Instructions.ValueString(),
        },
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
	tools, d := extractV2Tools(ctx, m.Tools)
    diags.Append(d...)
    req.Definition.Tools = tools
    return req, diags
}

func modelToUpdateV2Request(ctx context.Context, m FoundryAgentV2ResourceModel) (client.UpdateAgentV2Request, diag.Diagnostics) {
    var diags diag.Diagnostics
    req := client.UpdateAgentV2Request{
        Description: m.Description.ValueString(),
        Definition: client.AgentDefinitionV2{
            Kind:         m.Kind.ValueString(),
            Model:        m.Model.ValueString(),
            Instructions: m.Instructions.ValueString(),
        },
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
	tools, d := extractV2Tools(ctx, m.Tools)
    diags.Append(d...)
    req.Definition.Tools = tools
    return req, diags
}

func responseToV2Model(ctx context.Context, r *client.AgentResponseV2, m *FoundryAgentV2ResourceModel) diag.Diagnostics {
    var diags diag.Diagnostics
    m.ID          = types.StringValue(r.ID)
    m.Name        = types.StringValue(r.Name)
    m.Version     = types.StringValue(r.Versions.Latest.Version)
    m.CreatedAt   = types.Int64Value(r.Versions.Latest.CreatedAt)
    m.Description = types.StringValue(r.Versions.Latest.Description)
    m.Kind        = types.StringValue(r.Versions.Latest.Definition.Kind)
    m.Model       = types.StringValue(r.Versions.Latest.Definition.Model)
    m.Instructions = types.StringValue(r.Versions.Latest.Definition.Instructions)

    if r.Versions.Latest.Metadata != nil {
        metaAttrs := make(map[string]attr.Value, len(r.Versions.Latest.Metadata))
        for k, v := range r.Versions.Latest.Metadata {
            metaAttrs[k] = types.StringValue(v)
        }
        metaMap, d := types.MapValue(types.StringType, metaAttrs)
        diags.Append(d...)
        m.Metadata = metaMap
    } else {
        m.Metadata = types.MapValueMust(types.StringType, map[string]attr.Value{})
    }
	toolAttrTypes := map[string]attr.Type{
		"type":             types.StringType,
		"vector_store_ids": types.ListType{ElemType: types.StringType},
		"max_num_results":  types.Int64Type,
	}

	toolObjects := make([]attr.Value, 0, len(r.Versions.Latest.Definition.Tools))
	for _, t := range r.Versions.Latest.Definition.Tools {
		toolMap, ok := t.(map[string]interface{})
		if !ok {
			continue
		}
		toolType, _ := toolMap["type"].(string)
    
		vsIDsRaw, _ := toolMap["vector_store_ids"].([]interface{})
		vsIDVals := make([]attr.Value, len(vsIDsRaw))
		for i, v := range vsIDsRaw {
			vsIDVals[i] = types.StringValue(fmt.Sprintf("%v", v))
		}
		vsList, d := types.ListValue(types.StringType, vsIDVals)
    	diags.Append(d...)
    	maxResults, _ := toolMap["max_num_results"].(float64)
		
		obj, d := types.ObjectValue(toolAttrTypes, map[string]attr.Value{
			"type":             types.StringValue(toolType),
			"vector_store_ids": vsList,
			"max_num_results":  types.Int64Value(int64(maxResults)),
		})
		diags.Append(d...)
		toolObjects = append(toolObjects, obj)
	}
	toolList, d := types.ListValue(types.ObjectType{AttrTypes: toolAttrTypes}, toolObjects)
	diags.Append(d...)
	m.Tools = toolList
    return diags
}


func extractV2Tools(ctx context.Context, toolsList types.List) ([]interface{}, diag.Diagnostics) {
    var diags diag.Diagnostics
    if toolsList.IsNull() || toolsList.IsUnknown() {
        return nil, diags
    }

    var tools []toolModelV2
    diags.Append(toolsList.ElementsAs(ctx, &tools, false)...)
    if diags.HasError() {
        return nil, diags
    }

    result := make([]interface{}, len(tools))
    for i, t := range tools {
        var vsIDs []string
        diags.Append(t.VectorStoreIDs.ElementsAs(ctx, &vsIDs, false)...)
        result[i] = client.FileSearchToolV2{
            Type:           t.Type.ValueString(),
            VectorStoreIDs: vsIDs,
            MaxNumResults:  int(t.MaxNumResults.ValueInt64()),
        }
    }
    return result, diags
}