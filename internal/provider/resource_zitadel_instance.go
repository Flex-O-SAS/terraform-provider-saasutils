package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"

	systempb "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/system"
	"google.golang.org/protobuf/types/known/timestamppb"

	"terraform-provider-saasutils/internal/zitadelapi"
)

var (
	_ resource.Resource                = &resourceZitadelInstance{}
	_ resource.ResourceWithConfigure   = &resourceZitadelInstance{}
	_ resource.ResourceWithImportState = &resourceZitadelInstance{}
)

func NewZitadelInstanceResource() resource.Resource {
	return &resourceZitadelInstance{}
}

type resourceZitadelInstance struct {
	client *zitadelapi.Client
}

// zitadelInstanceHumanModel is the create-only block describing the first
// human owner of the new instance. All fields force replacement when changed.
type zitadelInstanceHumanModel struct {
	UserName               types.String `tfsdk:"user_name"`
	Email                  types.String `tfsdk:"email"`
	IsEmailVerified        types.Bool   `tfsdk:"is_email_verified"`
	FirstName              types.String `tfsdk:"first_name"`
	LastName               types.String `tfsdk:"last_name"`
	PreferredLanguage      types.String `tfsdk:"preferred_language"`
	Password               types.String `tfsdk:"password"`
	PasswordChangeRequired types.Bool   `tfsdk:"password_change_required"`
}

// zitadelInstancePATModel is the optional create-only block under `machine`
// that requests a Personal Access Token to be issued for the machine user.
type zitadelInstancePATModel struct {
	ExpirationDate types.String `tfsdk:"expiration_date"`
}

// zitadelInstanceMachineModel is the create-only block describing a machine
// (service account) owner of the new instance. Mutually exclusive with the
// `human` block.
type zitadelInstanceMachineModel struct {
	UserName            types.String              `tfsdk:"user_name"`
	Name                types.String              `tfsdk:"name"`
	PersonalAccessToken []zitadelInstancePATModel `tfsdk:"personal_access_token"`
}

type zitadelInstanceModel struct {
	ID              types.String                  `tfsdk:"id"`
	InstanceName    types.String                  `tfsdk:"instance_name"`
	FirstOrgName    types.String                  `tfsdk:"first_org_name"`
	CustomDomain    types.String                  `tfsdk:"custom_domain"`
	DefaultLanguage types.String                  `tfsdk:"default_language"`
	Human           []zitadelInstanceHumanModel   `tfsdk:"human"`
	Machine         []zitadelInstanceMachineModel `tfsdk:"machine"`
	PAT             types.String                  `tfsdk:"pat"`
}

func (r *resourceZitadelInstance) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_zitadel_instance"
}

func (r *resourceZitadelInstance) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceOnChange := []planmodifier.String{stringplanmodifier.RequiresReplace()}

	resp.Schema = schema.Schema{
		Description: "Creates a Zitadel instance on the configured Zitadel tenant via the SystemAPI. " +
			"Only `instance_name` is updatable; changing any other field forces a replacement.",
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Computed: true, Description: "Instance ID returned by Zitadel."},
			"instance_name": schema.StringAttribute{Required: true},
			"first_org_name": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: replaceOnChange,
			},
			"custom_domain": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: replaceOnChange,
			},
			"default_language": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: replaceOnChange,
			},
			"pat": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Personal Access Token returned by Zitadel when creating the instance. Empty if not requested.",
			},
		},
		Blocks: map[string]schema.Block{
			"human": schema.ListNestedBlock{
				Description: "Human owner of the new instance. Mutually exclusive with `machine`; exactly one of the two must be set. All fields are create-only.",
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"user_name":                schema.StringAttribute{Required: true, PlanModifiers: replaceOnChange},
						"email":                    schema.StringAttribute{Required: true, PlanModifiers: replaceOnChange},
						"is_email_verified":        schema.BoolAttribute{Optional: true},
						"first_name":               schema.StringAttribute{Required: true, PlanModifiers: replaceOnChange},
						"last_name":                schema.StringAttribute{Required: true, PlanModifiers: replaceOnChange},
						"preferred_language":       schema.StringAttribute{Optional: true, PlanModifiers: replaceOnChange},
						"password":                 schema.StringAttribute{Required: true, Sensitive: true, PlanModifiers: replaceOnChange},
						"password_change_required": schema.BoolAttribute{Optional: true},
					},
				},
			},
			"machine": schema.ListNestedBlock{
				Description: "Machine (service account) owner of the new instance. Mutually exclusive with `human`; exactly one of the two must be set. All fields are create-only.",
				Validators: []validator.List{
					listvalidator.SizeAtMost(1),
				},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"user_name": schema.StringAttribute{Required: true, PlanModifiers: replaceOnChange},
						"name":      schema.StringAttribute{Required: true, PlanModifiers: replaceOnChange},
					},
					Blocks: map[string]schema.Block{
						"personal_access_token": schema.ListNestedBlock{
							Description: "If present, a Personal Access Token is issued for the machine user and returned as `pat`.",
							Validators: []validator.List{
								listvalidator.SizeAtMost(1),
							},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"expiration_date": schema.StringAttribute{
										Optional:      true,
										Description:   "RFC3339 timestamp for PAT expiration. Omit for no expiration.",
										PlanModifiers: replaceOnChange,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func (r *resourceZitadelInstance) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(
		resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...,
	)
}

func (r *resourceZitadelInstance) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

	// data.Zitadel may legitimately be nil at plan time when the provider's
	// zitadel block contains values only known during apply. requireClient
	// reports the error from CRUD methods, which only run with a real client
	// expected.
	r.client = data.Zitadel
}

// requireClient adds an error diagnostic when the zitadel client is absent,
// i.e. the provider's zitadel block was never (fully) configured.
func (r *resourceZitadelInstance) requireClient(diags *diag.Diagnostics) bool {
	if r.client == nil {
		diags.AddError(
			"zitadel provider block not configured",
			"The saasutils_zitadel_instance resource requires the provider's zitadel { ... } block to be set with all fields.",
		)
		return false
	}
	return true
}

// buildCreateRequest maps the plan into a SystemAPI CreateInstanceRequest.
// Exactly one of `human` or `machine` must be set (the API's owner field is a
// oneof). When `machine.personal_access_token` is present, the response will
// include a PAT.
func buildCreateRequest(m *zitadelInstanceModel) (*systempb.CreateInstanceRequest, error) {
	if (len(m.Human) == 0) == (len(m.Machine) == 0) {
		return nil, fmt.Errorf("exactly one of `human { ... }` or `machine { ... }` must be set on saasutils_zitadel_instance")
	}

	req := &systempb.CreateInstanceRequest{
		InstanceName:    m.InstanceName.ValueString(),
		FirstOrgName:    m.FirstOrgName.ValueString(),
		CustomDomain:    m.CustomDomain.ValueString(),
		DefaultLanguage: m.DefaultLanguage.ValueString(),
	}

	if len(m.Human) > 0 {
		h := m.Human[0]
		req.Owner = &systempb.CreateInstanceRequest_Human_{
			Human: &systempb.CreateInstanceRequest_Human{
				UserName: h.UserName.ValueString(),
				Email: &systempb.CreateInstanceRequest_Email{
					Email:           h.Email.ValueString(),
					IsEmailVerified: h.IsEmailVerified.ValueBool(),
				},
				Profile: &systempb.CreateInstanceRequest_Profile{
					FirstName:         h.FirstName.ValueString(),
					LastName:          h.LastName.ValueString(),
					PreferredLanguage: h.PreferredLanguage.ValueString(),
				},
				Password: &systempb.CreateInstanceRequest_Password{
					Password:               h.Password.ValueString(),
					PasswordChangeRequired: h.PasswordChangeRequired.ValueBool(),
				},
			},
		}
		return req, nil
	}

	mc := m.Machine[0]
	machine := &systempb.CreateInstanceRequest_Machine{
		UserName: mc.UserName.ValueString(),
		Name:     mc.Name.ValueString(),
	}
	if len(mc.PersonalAccessToken) > 0 {
		pat := &systempb.CreateInstanceRequest_PersonalAccessToken{}
		if exp := mc.PersonalAccessToken[0].ExpirationDate.ValueString(); exp != "" {
			t, err := time.Parse(time.RFC3339, exp)
			if err != nil {
				return nil, fmt.Errorf("machine.personal_access_token.expiration_date must be RFC3339: %w", err)
			}
			pat.ExpirationDate = timestamppb.New(t)
		}
		machine.PersonalAccessToken = pat
	}
	req.Owner = &systempb.CreateInstanceRequest_Machine_{Machine: machine}
	return req, nil
}

func (r *resourceZitadelInstance) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	if !r.requireClient(&resp.Diagnostics) {
		return
	}

	var plan zitadelInstanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	body, err := buildCreateRequest(&plan)
	if err != nil {
		resp.Diagnostics.AddError("Invalid zitadel instance plan", err.Error())
		return
	}

	created, err := r.client.CreateInstance(ctx, body)
	if err != nil {
		resp.Diagnostics.AddError("Unable to create Zitadel instance", err.Error())
		return
	}

	plan.ID = types.StringValue(created.GetInstanceId())
	plan.PAT = types.StringValue(created.GetPat())

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceZitadelInstance) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	if !r.requireClient(&resp.Diagnostics) {
		return
	}

	var state zitadelInstanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	detail, err := r.client.GetInstance(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read Zitadel instance", err.Error())
		return
	}
	if detail == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	// Only refresh the server-authoritative fields. Create-only inputs
	// (first_org_name, custom_domain, default_language, human, pat) are
	// preserved as-is — the SystemAPI doesn't expose them after creation.
	state.InstanceName = types.StringValue(detail.GetName())

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceZitadelInstance) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	if !r.requireClient(&resp.Diagnostics) {
		return
	}

	var plan zitadelInstanceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var state zitadelInstanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if plan.InstanceName.ValueString() != state.InstanceName.ValueString() {
		if err := r.client.UpdateInstanceName(ctx, state.ID.ValueString(), plan.InstanceName.ValueString()); err != nil {
			resp.Diagnostics.AddError("Unable to update Zitadel instance", err.Error())
			return
		}
	}

	// Preserve id and pat across updates.
	plan.ID = state.ID
	plan.PAT = state.PAT

	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *resourceZitadelInstance) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	if !r.requireClient(&resp.Diagnostics) {
		return
	}

	var state zitadelInstanceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	if err := r.client.RemoveInstance(ctx, state.ID.ValueString()); err != nil {
		resp.Diagnostics.AddError("Unable to delete Zitadel instance", err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}
