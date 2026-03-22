// Copyright (c) Your Org
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"os"

	"github.com/andrewCluey/terraform-provider-azurefoundry/internal/client"
	"github.com/andrewCluey/terraform-provider-azurefoundry/internal/resources"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ provider.Provider = &AzureFoundryProvider{}
var _ provider.ProviderWithFunctions = &AzureFoundryProvider{}

type AzureFoundryProvider struct {
	version string
}

type AzureFoundryProviderModel struct {
	ProjectEndpoint types.String `tfsdk:"project_endpoint"`
	TenantID        types.String `tfsdk:"tenant_id"`
	ClientID        types.String `tfsdk:"client_id"`
	ClientSecret    types.String `tfsdk:"client_secret"`
	APIKey          types.String `tfsdk:"api_key"`
	UseAzureCLI     types.Bool   `tfsdk:"use_azure_cli"`
}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &AzureFoundryProvider{version: version}
	}
}

func (p *AzureFoundryProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "azurefoundry"
	resp.Version = p.version
}

func (p *AzureFoundryProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		MarkdownDescription: "The azurefoundry provider manages resources in Azure AI Foundry.",
		Attributes: map[string]schema.Attribute{
			"project_endpoint": schema.StringAttribute{
				MarkdownDescription: "The Azure AI Foundry project endpoint. " +
					"Can also be set via `AZURE_AI_FOUNDRY_PROJECT_ENDPOINT`.",
				Optional: true,
			},
			// ── API Key ───────────────────────────────────────────────────────
			"api_key": schema.StringAttribute{
				MarkdownDescription: "An API key for the Foundry project. " +
					"Can also be set via `AZURE_AI_FOUNDRY_API_KEY`.",
				Optional:  true,
				Sensitive: true,
			},
			// ── Service Principal ─────────────────────────────────────────────
			"tenant_id": schema.StringAttribute{
				MarkdownDescription: "Azure AD tenant ID. Can also be set via `AZURE_TENANT_ID`.",
				Optional:            true,
			},
			"client_id": schema.StringAttribute{
				MarkdownDescription: "Service principal client ID. Can also be set via `AZURE_CLIENT_ID`.",
				Optional:            true,
			},
			"client_secret": schema.StringAttribute{
				MarkdownDescription: "Service principal client secret. Can also be set via `AZURE_CLIENT_SECRET`.",
				Optional:            true,
				Sensitive:           true,
			},
			// ── Azure CLI ─────────────────────────────────────────────────────
			"use_azure_cli": schema.BoolAttribute{
				MarkdownDescription: "Use credentials from `az login`. Defaults to `false`.",
				Optional:            true,
			},
		},
	}
}

func (p *AzureFoundryProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	tflog.Info(ctx, "Configuring azurefoundry provider")

	var config AzureFoundryProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// Resolve project endpoint.
	projectEndpoint := os.Getenv("AZURE_AI_FOUNDRY_PROJECT_ENDPOINT")
	if !config.ProjectEndpoint.IsNull() && !config.ProjectEndpoint.IsUnknown() {
		projectEndpoint = config.ProjectEndpoint.ValueString()
	}
	if projectEndpoint == "" {
		resp.Diagnostics.AddError(
			"Missing project_endpoint",
			"Set project_endpoint in the provider block or the AZURE_AI_FOUNDRY_PROJECT_ENDPOINT environment variable.",
		)
		return
	}

	// ── Auth method 1: API Key ────────────────────────────────────────────────
	apiKey := os.Getenv("AZURE_AI_FOUNDRY_API_KEY")
	if !config.APIKey.IsNull() && !config.APIKey.IsUnknown() {
		apiKey = config.APIKey.ValueString()
	}
	if apiKey != "" {
		tflog.Info(ctx, "azurefoundry: authenticating with API key")
		apiClient := client.NewFoundryClientWithAPIKey(projectEndpoint, apiKey)
		resp.DataSourceData = apiClient
		resp.ResourceData = apiClient
		return
	}

	// ── Auth method 2: Service Principal ─────────────────────────────────────
	tenantID := firstNonEmpty(attrString(config.TenantID), os.Getenv("AZURE_TENANT_ID"))
	clientID := firstNonEmpty(attrString(config.ClientID), os.Getenv("AZURE_CLIENT_ID"))
	clientSecret := firstNonEmpty(attrString(config.ClientSecret), os.Getenv("AZURE_CLIENT_SECRET"))

	if tenantID != "" && clientID != "" && clientSecret != "" {
		cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret, nil)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create service principal credential", err.Error())
			return
		}
		tflog.Info(ctx, "azurefoundry: authenticating with service principal")
		apiClient := client.NewFoundryClientWithCredential(projectEndpoint, cred)
		resp.DataSourceData = apiClient
		resp.ResourceData = apiClient
		return
	}

	// ── Auth method 3: Azure CLI ──────────────────────────────────────────────
	useAzureCLI := !config.UseAzureCLI.IsNull() && config.UseAzureCLI.ValueBool()
	if useAzureCLI {
		cred, err := azidentity.NewAzureCLICredential(nil)
		if err != nil {
			resp.Diagnostics.AddError("Failed to create Azure CLI credential", err.Error())
			return
		}
		tflog.Info(ctx, "azurefoundry: authenticating with Azure CLI")
		apiClient := client.NewFoundryClientWithCredential(projectEndpoint, cred)
		resp.DataSourceData = apiClient
		resp.ResourceData = apiClient
		return
	}

	// ── Auth method 4: Default Azure credential chain ─────────────────────────
	// Covers managed identity, workload identity, environment variables, etc.
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to create Azure credential",
			"No valid authentication method was found. Set api_key, service principal credentials, "+
				"or use_azure_cli = true. Error: "+err.Error(),
		)
		return
	}
	tflog.Info(ctx, "azurefoundry: authenticating with default Azure credential chain")
	apiClient := client.NewFoundryClientWithCredential(projectEndpoint, cred)
	resp.DataSourceData = apiClient
	resp.ResourceData = apiClient
}

func (p *AzureFoundryProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		resources.NewFoundryAgentResource,
		resources.NewFoundryFileResource,
		resources.NewFoundryVectorStoreResource,
		resources.NewFoundryAgentV2Resource,
		resources.NewFoundryFileV2Resource,
		resources.NewFoundryVectorStoreV2Resource,
	}
}

func (p *AzureFoundryProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{}
}

func (p *AzureFoundryProvider) Functions(_ context.Context) []func() function.Function {
	return []func() function.Function{}
}

func attrString(attr types.String) string {
	if attr.IsNull() || attr.IsUnknown() {
		return ""
	}
	return attr.ValueString()
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}