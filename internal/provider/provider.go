package provider

import (
	"context"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"terraform-provider-saasutils/internal/ckboxapi"
	"terraform-provider-saasutils/internal/gobrightapi"
)

var _ provider.ProviderWithFunctions = &stringFunctionsProvider{}

func New(version string) func() provider.Provider {
	return func() provider.Provider {
		return &stringFunctionsProvider{version: version}
	}
}

type stringFunctionsProvider struct {
	version string

	data *providerData
}

// providerData bundles each sub-block's authenticated client. Either field may
// be nil if the corresponding sub-block is absent from the provider HCL.
type providerData struct {
	CKBox    *ckboxapi.APIClient
	GoBright *gobrightapi.APIClient
}

type providerConfigModel struct {
	CKBox    *ckboxConfigModel    `tfsdk:"ckbox"`
	GoBright *gobrightConfigModel `tfsdk:"gobright"`
}

type ckboxConfigModel struct {
	BaseURL        types.String `tfsdk:"base_url"`
	Email          types.String `tfsdk:"email"`
	Password       types.String `tfsdk:"password"`
	OrganizationId types.String `tfsdk:"organization_id"`
	SubscriptionId types.String `tfsdk:"subscription_id"`
}

type gobrightConfigModel struct {
	BaseURL          types.String `tfsdk:"base_url"`
	OrganizationCode types.String `tfsdk:"organization_code"`
	Login            types.String `tfsdk:"login"`
	Password         types.String `tfsdk:"password"`
}

func (p *stringFunctionsProvider) Metadata(ctx context.Context, req provider.MetadataRequest, resp *provider.MetadataResponse) {
	tflog.Info(ctx, "Loading saasutils metadata...")
	resp.TypeName = "saasutils"
	resp.Version = p.version
}

func (p *stringFunctionsProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Blocks: map[string]schema.Block{
			"ckbox": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"base_url":        schema.StringAttribute{Optional: true},
					"email":           schema.StringAttribute{Optional: true},
					"password":        schema.StringAttribute{Optional: true, Sensitive: true},
					"organization_id": schema.StringAttribute{Optional: true},
					"subscription_id": schema.StringAttribute{Optional: true},
				},
			},
			"gobright": schema.SingleNestedBlock{
				Attributes: map[string]schema.Attribute{
					"base_url":          schema.StringAttribute{Optional: true},
					"organization_code": schema.StringAttribute{Optional: true},
					"login":             schema.StringAttribute{Optional: true},
					"password":          schema.StringAttribute{Optional: true, Sensitive: true},
				},
			},
		},
	}
}

func isSet(v types.String) bool {
	return !v.IsNull() &&
		!v.IsUnknown() &&
		strings.TrimSpace(v.ValueString()) != ""
}

// configureCKBox builds (and authenticates) a CKBox client when the ckbox
// sub-block is fully populated. It treats partial configuration as a hard
// error.
func configureCKBox(ctx context.Context, cfg *ckboxConfigModel, resp *provider.ConfigureResponse) *ckboxapi.APIClient {
	if cfg == nil {
		return nil
	}

	type pair struct {
		name string
		val  types.String
	}
	fields := []pair{
		{"base_url", cfg.BaseURL},
		{"email", cfg.Email},
		{"password", cfg.Password},
		{"organization_id", cfg.OrganizationId},
		{"subscription_id", cfg.SubscriptionId},
	}

	var missing []string
	setCount := 0
	for _, f := range fields {
		if isSet(f.val) {
			setCount++
		} else {
			missing = append(missing, f.name)
		}
	}
	if setCount == 0 {
		return nil
	}
	if setCount < len(fields) {
		resp.Diagnostics.AddError(
			"Invalid ckbox provider configuration",
			"All ckbox fields must be set together. Missing: "+strings.Join(missing, ", "),
		)
		return nil
	}

	client := ckboxapi.NewCkboxClient(
		cfg.BaseURL.ValueString()+"v1",
		cfg.OrganizationId.ValueString(),
		cfg.SubscriptionId.ValueString(),
		60*time.Second,
	)
	client.SetHeader("Origin", cfg.BaseURL.ValueString())
	client.SetHeader("Referer", cfg.BaseURL.ValueString()+"/")
	client.SetHeader("Accept", "*/*")
	client.SetHeader("organizationid", cfg.OrganizationId.ValueString())

	tflog.Debug(ctx, "Authenticating CKBox client")
	_, err := client.Authenticate(ctx, cfg.Email.ValueString(), cfg.Password.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Authentication failed", err.Error())
		return nil
	}
	return client
}

// configureGoBright builds (and authenticates) a GoBright client when the
// gobright sub-block is fully populated. It treats partial configuration as a
// hard error.
func configureGoBright(ctx context.Context, cfg *gobrightConfigModel, resp *provider.ConfigureResponse) *gobrightapi.APIClient {
	if cfg == nil {
		return nil
	}

	type pair struct {
		name string
		val  types.String
	}
	fields := []pair{
		{"base_url", cfg.BaseURL},
		{"organization_code", cfg.OrganizationCode},
		{"login", cfg.Login},
		{"password", cfg.Password},
	}

	var missing []string
	setCount := 0
	for _, f := range fields {
		if isSet(f.val) {
			setCount++
		} else {
			missing = append(missing, f.name)
		}
	}
	if setCount == 0 {
		return nil
	}
	if setCount < len(fields) {
		resp.Diagnostics.AddError(
			"Invalid gobright provider configuration",
			"All gobright fields must be set together. Missing: "+strings.Join(missing, ", "),
		)
		return nil
	}

	// Strip trailing slash if present so the client controls path joining.
	baseURL := strings.TrimRight(cfg.BaseURL.ValueString(), "/")
	client := gobrightapi.NewClient(
		baseURL,
		cfg.OrganizationCode.ValueString(),
		60*time.Second,
	)

	tflog.Debug(ctx, "Authenticating GoBright client")
	_, err := client.Authenticate(
		ctx,
		cfg.OrganizationCode.ValueString(),
		cfg.Login.ValueString(),
		cfg.Password.ValueString(),
	)
	if err != nil {
		resp.Diagnostics.AddError("GoBright authentication failed", err.Error())
		return nil
	}
	return client
}

func (p *stringFunctionsProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config providerConfigModel

	diags := req.Config.Get(ctx, &config)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	data := &providerData{}

	data.CKBox = configureCKBox(ctx, config.CKBox, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	data.GoBright = configureGoBright(ctx, config.GoBright, resp)
	if resp.Diagnostics.HasError() {
		return
	}

	p.data = data
	resp.ResourceData = data
	resp.DataSourceData = data
}

func (*stringFunctionsProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return []func() datasource.DataSource{
		NewCkboxEnvDataSource,
		NewGoBrightIntegrationDataSource,
	}
}
func (*stringFunctionsProvider) Functions(_ context.Context) []func() function.Function {
	return []func() function.Function{
		NewICaseAsgIdFunction,
		NewCustomersConfigFunction,
		NewCustomersSecretsFunction,
		NewJwtSignedFunction,
	}
}
func (p *stringFunctionsProvider) Resources(_ context.Context) []func() resource.Resource {
	return []func() resource.Resource{
		NewCkboxEnvResource,
		NewCkboxAccessKeyResource,
		NewGoBrightIntegrationResource,
	}
}
