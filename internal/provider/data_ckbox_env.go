package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"terraform-provider-saasutils/internal/ckboxapi"
)

// Ensure the implementation satisfies the desired interfaces.
var _ datasource.DataSource = &dataCkboxEnv{}

func NewCkboxEnvDataSource() datasource.DataSource {
	return &dataCkboxEnv{}
}

type dataCkboxEnv struct {
	client *ckboxapi.APIClient
}

func (d *dataCkboxEnv) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ckbox_env"
}

func (d *dataCkboxEnv) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
			},
			"id": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *dataCkboxEnv) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

	if data.CKBox == nil {
		resp.Diagnostics.AddError(
			"ckbox provider block not configured",
			"The saasutils_ckbox_env data source requires the provider's ckbox { ... } block to be set with all fields.",
		)
		return
	}

	d.client = data.CKBox
}

func (d *dataCkboxEnv) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state ckboxEnvModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	env, err := d.client.ReadCkboxEnv(ctx, state.Name.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Unable to read CKBox env", err.Error())
		return
	}

	state.ID = types.StringValue(env.Id)
	state.Name = types.StringValue(env.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
