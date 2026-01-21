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
	"strings"
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
	BaseURL         types.String `tfsdk:"base_url"`
	Email           types.String `tfsdk:"email"`
	Password        types.String `tfsdk:"password"`
	Organization_id types.String `tfsdk:"organization_id"`
	Subscription_id types.String `tfsdk:"subscription_id"`
}

func (p *stringFunctionsProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	tflog.Info(ctx, "Loading saasutils metadata...")
	resp.TypeName = "saasutils"
	resp.Version = p.version
}

func (p *stringFunctionsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"subscription_id": schema.StringAttribute{
				Optional: true,
			},
			"organization_id": schema.StringAttribute{
				Optional: true,
			},
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

func isSet(v types.String) bool {
	return !v.IsNull() &&
		!v.IsUnknown() &&
		strings.TrimSpace(v.ValueString()) != ""
}

func (p *stringFunctionsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerConfigModel

	diags := req.Config.Get(ctx, &config)

	anySet :=
		isSet(config.BaseURL) ||
			isSet(config.Organization_id) ||
			isSet(config.Subscription_id) ||
			isSet(config.Email) ||
			isSet(config.Password)

	allSet :=
		isSet(config.BaseURL) &&
			isSet(config.Organization_id) &&
			isSet(config.Subscription_id) &&
			isSet(config.Email) &&
			isSet(config.Password)

	if allSet {
		p.client = ckboxapi.NewCkboxClient(
			config.BaseURL.ValueString()+"v1",
			config.Organization_id.ValueString(),
			config.Subscription_id.ValueString(),
			60*time.Second,
		)

		p.client.SetHeader("Origin", config.BaseURL.ValueString())
		p.client.SetHeader("Referer", config.BaseURL.ValueString()+"/")
		p.client.SetHeader("Accept", "*/*")
		p.client.SetHeader("organizationid", config.Organization_id.ValueString())

		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
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

		resp.ResourceData = p.client
		resp.DataSourceData = p.client
	} else if anySet && !allSet {
		resp.Diagnostics.AddError(
			"Invalid Configuration, all fields must be set if you intend to use it as the ckbox provider",
			"Invalid Configuration",
		)
	}
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
