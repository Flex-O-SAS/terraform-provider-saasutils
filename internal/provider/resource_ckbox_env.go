// Copyright (c) HashiCorp, Inc.

package provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"terraform-provider-saasutils/internal/ckboxapi"
)

var (
	_ resource.Resource                = &resourceCkboxEnv{}
	_ resource.ResourceWithConfigure   = &resourceCkboxEnv{}
	_ resource.ResourceWithImportState = &resourceCkboxEnv{}
)

func NewCkboxEnvResource() resource.Resource {
	return &resourceCkboxEnv{}
}

type resourceCkboxEnv struct {
	client *ckboxapi.APIClient
}

type ckboxEnvModel struct {
	ID   types.String `tfsdk:"id"`
	Name types.String `tfsdk:"name"`
}

func (r *resourceCkboxEnv) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ckbox_env"
}

func (r *resourceCkboxEnv) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed: true,
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

func (r *resourceCkboxEnv) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(
		resp.State.SetAttribute(ctx, path.Root("name"), req.ID)...,
	)
}

func (r *resourceCkboxEnv) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
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

func (r *resourceCkboxEnv) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan ckboxEnvModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	region := "us-east-1"
	env, err := r.client.CreateCkboxEnv(
		ctx,
		plan.Name.ValueString(),
		region,
	)

	if err != nil {
		resp.Diagnostics.AddError("Unable to create CKBox env", err.Error())
	}

	state := ckboxEnvModel{
		ID:   types.StringValue(env.Id),
		Name: types.StringValue(env.Name),
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceCkboxEnv) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state ckboxEnvModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := r.client.ReadCkboxEnv(ctx, state.Name.ValueString())
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			resp.State.RemoveResource(ctx)
			return
		}

		resp.Diagnostics.AddError("Unable to read CKBox env", err.Error())
		return
	}

	state.ID = types.StringValue(env.Id)
	state.Name = types.StringValue(env.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *resourceCkboxEnv) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {

}

func (r *resourceCkboxEnv) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state ckboxEnvModel
	diags := req.State.Get(ctx, &state)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}

	err := r.client.DeleteCkboxEnv(ctx, state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to delete CKBox env", err.Error())
		return
	}
	resp.State.RemoveResource(ctx)
}
