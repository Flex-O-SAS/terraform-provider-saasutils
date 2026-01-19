package provider

import (
	"context"
	"fmt"
	"strings"
	"terraform-provider-saasutils/internal/ckboxapi"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

var (
	_ resource.Resource              = &resourceCkboxAccessKey{}
	_ resource.ResourceWithConfigure = &resourceCkboxAccessKey{}
)

func NewCkboxAccessKeyResource() resource.Resource {
	return &resourceCkboxAccessKey{}
}

type resourceCkboxAccessKey struct {
	client *ckboxapi.APIClient
}

type CkboxAccesKeyModel struct {
	ID   	types.String `tfsdk:"id"`
	Name 	types.String `tfsdk:"name"`
	EnvId	types.String `tfsdk:"env_id"`
}

func (r *resourceCkboxAccessKey) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ckbox_access_key"
}

func (r *resourceCkboxAccessKey) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
			},
			"env_id": schema.StringAttribute{
				Required: true,
			},
			"name": schema.StringAttribute{
				Required: true,
				PlanModifiers: []planmodifier.String{
					stringplanmodifier.RequiresReplace(),
				},
			},
		},
	}
}

func (r *resourceCkboxAccessKey) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	client, ok := req.ProviderData.(*ckboxapi.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *ckboxapi.APIClient, got: %T", req.ProviderData),
		)
		return
	}

	r.client = client
}

func (r *resourceCkboxAccessKey) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan CkboxAccesKeyModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	accessKey, err := r.client.CreateCkboxAccessKey(
		ctx, 
		plan.Name.ValueString(), 
		plan.EnvId.ValueString(),
	)

	if err != nil {
		resp.Diagnostics.AddError("Unable to create CKBox access key", err.Error())
		return
	}

	state := CkboxAccesKeyModel{
		ID:     types.StringValue(accessKey.Value),
		Name:   types.StringValue(accessKey.Name),
		EnvId:	types.StringValue(plan.EnvId.ValueString()),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}


func (r *resourceCkboxAccessKey) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state CkboxAccesKeyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	accessKey, err := r.client.ReadCkboxAccessKey(
		ctx,
		state.Name.ValueString(),
		state.EnvId.ValueString(),
	)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Unable to read CKBox access key", err.Error())
		return
	}

	state.ID = types.StringValue(accessKey.Value)
	state.Name = types.StringValue(accessKey.Name)
	state.EnvId = types.StringValue(state.EnvId.ValueString())

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}


func (r *resourceCkboxAccessKey) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {

}

func (r *resourceCkboxAccessKey) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state CkboxAccesKeyModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)

	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteCkboxAccessKey(
		ctx,
		state.Name.ValueString(),
		state.EnvId.ValueString(),
		state.ID.ValueString(),
	)

	if err != nil {
		resp.Diagnostics.AddError("Unable to delete Ckbox Access Key for env : " + state.EnvId.ValueString(), err.Error())
		return
	}

	resp.State.RemoveResource(ctx)
}