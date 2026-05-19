package provider

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"terraform-provider-saasutils/internal/gobrightapi"
)

var (
	_ resource.Resource                = &resourceGoBrightIntegration{}
	_ resource.ResourceWithConfigure   = &resourceGoBrightIntegration{}
	_ resource.ResourceWithImportState = &resourceGoBrightIntegration{}
)

func NewGoBrightIntegrationResource() resource.Resource {
	return &resourceGoBrightIntegration{}
}

type resourceGoBrightIntegration struct {
	client *gobrightapi.APIClient
}

// goBrightOidcModel mirrors the nested `oidc` block. Shared by the resource
// (Required inputs except jwks_endpoint) and the data source (all Computed).
// validation_mode and token_replay_prevention_mode are projected to bool;
// fromModel / applyResponse handle the int↔bool translation at the API
// boundary.
type goBrightOidcModel struct {
	Audience                  types.String `tfsdk:"audience"`
	Issuer                    types.String `tfsdk:"issuer"`
	ValidationMode            types.Bool   `tfsdk:"validation_mode"`
	PublicKey                 types.String `tfsdk:"public_key"`
	JwksEndpoint              types.String `tfsdk:"jwks_endpoint"`
	UserIdentifierClaimName   types.String `tfsdk:"user_identifier_claim_name"`
	RelatedUserIntegrationId  types.Int64  `tfsdk:"related_user_integration_id"`
	TokenReplayPreventionMode types.Bool   `tfsdk:"token_replay_prevention_mode"`
}

type goBrightIntegrationModel struct {
	ID             types.String       `tfsdk:"id"`
	NewID          types.String       `tfsdk:"new_id"`
	Name           types.String       `tfsdk:"name"`
	ExternalSystem types.String       `tfsdk:"external_system"`
	Oidc           *goBrightOidcModel `tfsdk:"oidc"`
}

func (r *resourceGoBrightIntegration) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gobright_integration"
}

func (r *resourceGoBrightIntegration) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":     schema.StringAttribute{Computed: true},
			"new_id": schema.StringAttribute{Computed: true},
			"name":   schema.StringAttribute{Required: true},
			"external_system": schema.StringAttribute{
				Required:    true,
				Description: "Integration type. One of `openid` (GoBright externalSystem=14) or `office365` (=1).",
				Validators: []validator.String{
					stringvalidator.OneOf(supportedExternalSystemNames()...),
				},
			},
		},
		Blocks: map[string]schema.Block{
			"oidc": schema.SingleNestedBlock{
				Description: "OIDC configuration. The block is required; `fromModel` returns an error if it is omitted.",
				Attributes: map[string]schema.Attribute{
					"audience": schema.StringAttribute{Required: true},
					"issuer":   schema.StringAttribute{Required: true},
					"validation_mode": schema.BoolAttribute{
						Required:    true,
						Description: "GoBright `oidcValidationMode` projected to a boolean (false=0, true=1).",
					},
					"public_key":                  schema.StringAttribute{Required: true},
					"jwks_endpoint":               schema.StringAttribute{Computed: true},
					"user_identifier_claim_name":  schema.StringAttribute{Required: true},
					"related_user_integration_id": schema.Int64Attribute{Required: true},
					"token_replay_prevention_mode": schema.BoolAttribute{
						Required:    true,
						Description: "GoBright `oidcTokenReplayPreventionMode` projected to a boolean (false=0, true=1).",
					},
				},
			},
		},
	}
}

func (r *resourceGoBrightIntegration) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(
		resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...,
	)
}

func (r *resourceGoBrightIntegration) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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
			"The saasutils_gobright_integration resource requires the provider's gobright { ... } block to be set with all fields.",
		)
		return
	}

	r.client = data.GoBright
}

// fromModel populates a gobrightapi.Integration from the plan model. Computed
// fields (Id, NewId, OidcJwksEndpoint) are left as their zero value. Returns
// an error if the `oidc` block is missing or if external_system can't be
// mapped (schema-level validation should already catch the latter).
func fromModel(m *goBrightIntegrationModel) (*gobrightapi.Integration, error) {
	if m.Oidc == nil {
		return nil, fmt.Errorf("the `oidc { ... }` block is required on saasutils_gobright_integration")
	}
	es, err := externalSystemFromString(m.ExternalSystem.ValueString())
	if err != nil {
		return nil, err
	}
	return &gobrightapi.Integration{
		Name:                          m.Name.ValueString(),
		ExternalSystem:                es,
		OidcAudience:                  m.Oidc.Audience.ValueString(),
		OidcIssuer:                    m.Oidc.Issuer.ValueString(),
		OidcValidationMode:            boolToInt(m.Oidc.ValidationMode.ValueBool()),
		OidcPublicKey:                 m.Oidc.PublicKey.ValueString(),
		OidcUserIdentifierClaimName:   m.Oidc.UserIdentifierClaimName.ValueString(),
		OidcRelatedUserIntegrationId:  m.Oidc.RelatedUserIntegrationId.ValueInt64(),
		OidcTokenReplayPreventionMode: boolToInt(m.Oidc.TokenReplayPreventionMode.ValueBool()),
	}, nil
}

// applyResponse merges the server-side Integration response into the given
// state model. Allocates / overwrites the nested oidc block. Returns an
// error if external_system is outside the set this provider models.
func applyResponse(state *goBrightIntegrationModel, resp *gobrightapi.Integration) error {
	es, err := externalSystemToString(resp.ExternalSystem)
	if err != nil {
		return err
	}
	state.ID = types.StringValue(strconv.FormatInt(resp.Id, 10))
	state.NewID = types.StringValue(resp.NewId)
	state.Name = types.StringValue(resp.Name)
	state.ExternalSystem = types.StringValue(es)
	state.Oidc = &goBrightOidcModel{
		Audience:                  types.StringValue(resp.OidcAudience),
		Issuer:                    types.StringValue(resp.OidcIssuer),
		ValidationMode:            types.BoolValue(intToBool(resp.OidcValidationMode)),
		PublicKey:                 types.StringValue(resp.OidcPublicKey),
		JwksEndpoint:              types.StringValue(resp.OidcJwksEndpoint),
		UserIdentifierClaimName:   types.StringValue(resp.OidcUserIdentifierClaimName),
		RelatedUserIntegrationId:  types.Int64Value(resp.OidcRelatedUserIntegrationId),
		TokenReplayPreventionMode: types.BoolValue(intToBool(resp.OidcTokenReplayPreventionMode)),
	}
	return nil
}

func (r *resourceGoBrightIntegration) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan goBrightIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, err := fromModel(&plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid GoBright integration plan", err.Error())
		return
	}

	created, err := r.client.CreateIntegration(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create GoBright integration", err.Error())
		return
	}

	if err := applyResponse(&plan, created); err != nil {
		resp.Diagnostics.AddError("Unable to apply GoBright integration response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceGoBrightIntegration) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state goBrightIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid GoBright integration id in state", err.Error())
		return
	}

	got, err := r.client.ReadIntegration(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Unable to read GoBright integration", err.Error())
		return
	}

	if err := applyResponse(&state, got); err != nil {
		resp.Diagnostics.AddError("Unable to apply GoBright integration response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceGoBrightIntegration) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan goBrightIntegrationModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state goBrightIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid GoBright integration id in state", err.Error())
		return
	}

	body, err := fromModel(&plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid GoBright integration plan", err.Error())
		return
	}
	body.Id = id

	updated, err := r.client.UpdateIntegration(ctx, id, body)
	if err != nil {
		resp.Diagnostics.AddError("Unable to update GoBright integration", err.Error())
		return
	}

	if err := applyResponse(&plan, updated); err != nil {
		resp.Diagnostics.AddError("Unable to apply GoBright integration response", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceGoBrightIntegration) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state goBrightIntegrationModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	id, err := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err != nil {
		resp.Diagnostics.AddError("Invalid GoBright integration id in state", err.Error())
		return
	}

	if err := r.client.DeleteIntegration(ctx, id); err != nil {
		resp.Diagnostics.AddError("Unable to delete GoBright integration", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}
