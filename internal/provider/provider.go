// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"terraform-provider-saasutils/internal/ckboxapi"
	"time"
)

var _ provider.ProviderWithFunctions = &stringFunctionsProvider{}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &stringFunctionsProvider{version: version}
	}
}

type stringFunctionsProvider struct {
	version string

	client *ckboxapi.APIClient
}

type providerConfigModel struct {
	BaseURL  types.String `tfsdk:"base_url"`
	Email    types.String `tfsdk:"email"`
	Password types.String `tfsdk:"password"`
}

func (p *stringFunctionsProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	tflog.Info(ctx, "Loading saasutils metadata...")
	resp.TypeName = "saasutils"
	resp.Version = p.version
}

func (p *stringFunctionsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"base_url": schema.StringAttribute{
				Optional: true,
			},
			"email": schema.StringAttribute{
				Optional: true,
			},
			"password": schema.StringAttribute{
				Optional: true,
			},
		},
	}
}

func (p *stringFunctionsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerConfigModel

	diags := req.Config.Get(ctx, &config)

	baseURL := "https://portal-api.ckeditor.com/v1"
	if !config.BaseURL.IsNull() && config.BaseURL.ValueString() != "" {
		baseURL = config.BaseURL.ValueString()
	}
	p.client = ckboxapi.NewCkboxClient(baseURL, 60*time.Second)

	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	if !config.Email.IsNull() && !config.Password.IsNull() {
		tflog.Debug(ctx, "Authenticating CKBox client")

		_, err := p.client.Authenticate(
			ctx,
			config.Email.ValueString(),
			config.Password.ValueString(),
		)
		if err != nil {
			resp.Diagnostics.AddError(
				"Authentication failed",
				err.Error(),
			)
			return
		}
	}

	// To provide the client in all the resources/datasources that needs it
	resp.ResourceData = p.client
	resp.DataSourceData = p.client
}

func (*stringFunctionsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewCkboxEnvDataSource,
	}
}
func (*stringFunctionsProvider) Functions(_ context.Context) []func() function.Function {
	return []func() function.Function{
		NewICaseAsgIdFunction,
		NewCustomersConfigFunction,
		NewCustomersSecretsFunction,
	}
}
func (p *stringFunctionsProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewCkboxEnvResource,
		NewCkboxAccessKeyResource,
	}
}
