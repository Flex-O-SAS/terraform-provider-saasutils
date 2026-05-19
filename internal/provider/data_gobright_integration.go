package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-saasutils/internal/gobrightapi"
)

var (
	_ datasource.DataSource              = &dataGoBrightIntegration{}
	_ datasource.DataSourceWithConfigure = &dataGoBrightIntegration{}
)

func NewGoBrightIntegrationDataSource() datasource.DataSource {
	return &dataGoBrightIntegration{}
}

type dataGoBrightIntegration struct {
	client *gobrightapi.APIClient
}

type goBrightIntegrationDataSourceModel struct {
	// Filter inputs.
	Name                    types.String `tfsdk:"name"`
	ExternalSystem          types.String `tfsdk:"external_system"`
	MicrosoftPermissionMode types.Bool   `tfsdk:"microsoft_permission_mode"`

	// Identity outputs.
	ID    types.String `tfsdk:"id"`
	NewID types.String `tfsdk:"new_id"`

	// OIDC output block — populated from the matched integration's full Read
	// response.
	Oidc *goBrightOidcModel `tfsdk:"oidc"`
}

func (d *dataGoBrightIntegration) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gobright_integration"
}

func (d *dataGoBrightIntegration) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Look up an existing GoBright integration by name + external_system + microsoft_permission_mode. Errors if the filters do not identify exactly one integration.",
		Attributes: map[string]schema.Attribute{
			// Filters
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Filter: GoBright integration `name` (exact match).",
			},
			"external_system": schema.StringAttribute{
				Required:    true,
				Description: "Filter: integration type. One of `openid` (GoBright externalSystem=14) or `office365` (=1).",
				Validators: []validator.String{
					stringvalidator.OneOf(supportedExternalSystemNames()...),
				},
			},
			"microsoft_permission_mode": schema.BoolAttribute{
				Optional:    true,
				Description: "Filter: GoBright `microsoftPermissionMode` projected to a boolean (false=0, true=1). Omit (or set `null`) to match integrations where the API returns null — typical for OIDC integrations.",
			},

			// Identity outputs
			"id":     schema.StringAttribute{Computed: true, Description: "Stringified GoBright integer id."},
			"new_id": schema.StringAttribute{Computed: true, Description: "Server-generated GUID."},
		},
		Blocks: map[string]schema.Block{
			"oidc": schema.SingleNestedBlock{
				Description: "OIDC fields from the matched integration. Populated by the data source's Read.",
				Attributes: map[string]schema.Attribute{
					"audience": schema.StringAttribute{Computed: true},
					"issuer":   schema.StringAttribute{Computed: true},
					"validation_mode": schema.BoolAttribute{
						Computed:    true,
						Description: "GoBright `oidcValidationMode` projected to a boolean (false=0, true=1).",
					},
					"public_key":                  schema.StringAttribute{Computed: true},
					"jwks_endpoint":               schema.StringAttribute{Computed: true},
					"user_identifier_claim_name":  schema.StringAttribute{Computed: true},
					"related_user_integration_id": schema.Int64Attribute{Computed: true},
					"token_replay_prevention_mode": schema.BoolAttribute{
						Computed:    true,
						Description: "GoBright `oidcTokenReplayPreventionMode` projected to a boolean (false=0, true=1).",
					},
				},
			},
		},
	}
}

func (d *dataGoBrightIntegration) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}
	data, ok := req.ProviderData.(*providerData)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *providerData, got: %T", req.ProviderData),
		)
		return
	}
	if data.GoBright == nil {
		resp.Diagnostics.AddError(
			"gobright provider block not configured",
			"The saasutils_gobright_integration data source requires the `gobright { ... }` block on the provider to be fully configured.",
		)
		return
	}
	d.client = data.GoBright
}

func (d *dataGoBrightIntegration) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config goBrightIntegrationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	wantName := config.Name.ValueString()
	wantExternalSystem, err := externalSystemFromString(config.ExternalSystem.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid external_system filter", err.Error())
		return
	}
	var wantMpm *int64
	if !config.MicrosoftPermissionMode.IsNull() && !config.MicrosoftPermissionMode.IsUnknown() {
		v := boolToInt(config.MicrosoftPermissionMode.ValueBool())
		wantMpm = &v
	}

	list, err := d.client.ListIntegrations(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Unable to list GoBright integrations", err.Error())
		return
	}

	var matches []gobrightapi.Integration
	for _, item := range list {
		if item.Name != wantName {
			continue
		}
		if item.ExternalSystem != wantExternalSystem {
			continue
		}
		if !nullableInt64Equal(item.MicrosoftPermissionMode, wantMpm) {
			continue
		}
		matches = append(matches, item)
	}

	switch len(matches) {
	case 0:
		resp.Diagnostics.AddError(
			"No GoBright integration matched the filters",
			fmt.Sprintf("name=%q, external_system=%q, microsoft_permission_mode=%s — no rows returned.",
				wantName, config.ExternalSystem.ValueString(), formatNullableBool(config.MicrosoftPermissionMode)),
		)
		return
	case 1:
		// proceed below
	default:
		ids := make([]string, len(matches))
		for i, m := range matches {
			ids[i] = strconv.FormatInt(m.Id, 10)
		}
		resp.Diagnostics.AddError(
			"Multiple GoBright integrations matched the filters",
			fmt.Sprintf("name=%q, external_system=%q, microsoft_permission_mode=%s — got %d rows with ids [%s]. Filters must identify exactly one integration.",
				wantName, config.ExternalSystem.ValueString(), formatNullableBool(config.MicrosoftPermissionMode), len(matches), strings.Join(ids, ", ")),
		)
		return
	}

	matched := matches[0]
	full, err := d.client.ReadIntegration(ctx, matched.Id)
	if err != nil {
		resp.Diagnostics.AddError(
			fmt.Sprintf("Unable to read GoBright integration %d", matched.Id),
			err.Error(),
		)
		return
	}

	state := config
	state.ID = types.StringValue(strconv.FormatInt(full.Id, 10))
	state.NewID = types.StringValue(full.NewId)
	state.Oidc = &goBrightOidcModel{
		Audience:                  types.StringValue(full.OidcAudience),
		Issuer:                    types.StringValue(full.OidcIssuer),
		ValidationMode:            types.BoolValue(intToBool(full.OidcValidationMode)),
		PublicKey:                 types.StringValue(full.OidcPublicKey),
		JwksEndpoint:              types.StringValue(full.OidcJwksEndpoint),
		UserIdentifierClaimName:   types.StringValue(full.OidcUserIdentifierClaimName),
		RelatedUserIntegrationId:  types.Int64Value(full.OidcRelatedUserIntegrationId),
		TokenReplayPreventionMode: types.BoolValue(intToBool(full.OidcTokenReplayPreventionMode)),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// nullableInt64Equal compares two nullable int64s. Both nil → equal. Exactly
// one nil → not equal. Both non-nil → values must match.
func nullableInt64Equal(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

// formatNullableBool renders a types.Bool as "null", "true", or "false" for
// use in error diagnostics.
func formatNullableBool(b types.Bool) string {
	if b.IsNull() || b.IsUnknown() {
		return "null"
	}
	if b.ValueBool() {
		return "true"
	}
	return "false"
}
