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
	"terraform-provider-saasutils/internal/zitadelapi"
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

// providerData bundles each sub-block's authenticated client. Any field may be
// nil if the corresponding sub-block is absent from the provider HCL.
type providerData struct {
	CKBox    *ckboxapi.APIClient
	GoBright *gobrightapi.APIClient
	Zitadel  *zitadelapi.Client
}

type providerConfigModel struct {
	CKBox    *ckboxConfigModel    `tfsdk:"ckbox"`
	GoBright *gobrightConfigModel `tfsdk:"gobright"`
	Zitadel  *zitadelConfigModel  `tfsdk:"zitadel"`
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

type zitadelConfigModel struct {
	Issuer   types.String `tfsdk:"issuer"`
	API      types.String `tfsdk:"api"`
	UserID   types.String `tfsdk:"user_id"`
	Key      types.String `tfsdk:"key"`
	Insecure types.Bool   `tfsdk:"insecure"`
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
			"zitadel": schema.SingleNestedBlock{
				Description: "Zitadel SystemAPI access for managing instances on a Zitadel tenant. " +
					"Authenticates as a configured SystemAPIUser via JWT profile.",
				Attributes: map[string]schema.Attribute{
					"issuer":   schema.StringAttribute{Optional: true, Description: "OIDC issuer, e.g. https://example.zitadel.cloud"},
					"api":      schema.StringAttribute{Optional: true, Description: "gRPC endpoint host:port, e.g. example.zitadel.cloud:443"},
					"user_id":  schema.StringAttribute{Optional: true, Description: "SystemAPIUser id configured in Zitadel"},
					"key":      schema.StringAttribute{Optional: true, Sensitive: true, Description: "PEM-encoded RSA private key whose public counterpart is registered for user_id"},
					"insecure": schema.BoolAttribute{Optional: true, Description: "Disable TLS on the gRPC connection (local dev only)"},
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

// configureZitadel builds an authenticated Zitadel system client when the
// zitadel sub-block is fully populated. It treats partial configuration as a
// hard error.
func configureZitadel(ctx context.Context, cfg *zitadelConfigModel, resp *provider.ConfigureResponse) *zitadelapi.Client {
	if cfg == nil {
		return nil
	}

	type pair struct {
		name string
		val  types.String
	}
	fields := []pair{
		{"issuer", cfg.Issuer},
		{"api", cfg.API},
		{"user_id", cfg.UserID},
		{"key", cfg.Key},
	}

	var missing []string
	setCount := 0
	unknownCount := 0
	for _, f := range fields {
		switch {
		case f.val.IsUnknown():
			unknownCount++
		case isSet(f.val):
			setCount++
		default:
			missing = append(missing, f.name)
		}
	}
	if cfg.Insecure.IsUnknown() {
		unknownCount++
	}
	// Values that are only known during apply (e.g. derived from another
	// resource) must not be treated as missing. Configure runs again at apply
	// time with concrete values; until then, just skip building the client.
	if unknownCount > 0 {
		tflog.Debug(ctx, "Zitadel configuration has unknown values, deferring client setup to apply")
		return nil
	}
	if setCount == 0 {
		return nil
	}
	if setCount < len(fields) {
		resp.Diagnostics.AddError(
			"Invalid zitadel provider configuration",
			"All zitadel fields (issuer, api, user_id, key) must be set together. Missing: "+strings.Join(missing, ", "),
		)
		return nil
	}

	tflog.Debug(ctx, "Building Zitadel system client")
	client, err := zitadelapi.NewClient(ctx, zitadelapi.Config{
		Issuer:   cfg.Issuer.ValueString(),
		API:      cfg.API.ValueString(),
		UserID:   cfg.UserID.ValueString(),
		Key:      []byte(cfg.Key.ValueString()),
		Insecure: cfg.Insecure.ValueBool(),
	})
	if err != nil {
		resp.Diagnostics.AddError("Zitadel client initialization failed", err.Error())
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

	data.Zitadel = configureZitadel(ctx, config.Zitadel, resp)
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
		NewZitadelInstanceResource,
	}
}
