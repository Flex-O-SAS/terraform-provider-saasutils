package provider

import (
	"context"
	"fmt"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"terraform-provider-saasutils/internal/ckboxapi"
)

var _ datasource.DataSource = &dataCkboxAccessKey{}

func NewCkboxAccessKeyResourceDataSource() datasource.DataSource {
	return &dataCkboxAccessKey{}
}

type dataCkboxAccessKey struct {
	client *ckboxapi.APIClient
}

func (d *dataCkboxAccessKey) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ckbox_access_key"
}

func (d *dataCkboxAccessKey) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"name": schema.StringAttribute{
				Required: true,
			},
			"env_id": schema.StringAttribute{
				Required: true,
			},
			"id": schema.StringAttribute{
				Computed: true,
			},
		},
	}
}

func (d *dataCkboxAccessKey) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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
			"The saasutils_ckbox_access_key data source requires the provider's ckbox { ... } block to be set with all fields.",
		)
		return
	}

	d.client = data.CKBox
}

func (d *dataCkboxAccessKey) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state CkboxAccesKeyModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)

	if resp.Diagnostics.HasError() {
		return
	}

	accessKey, err := d.client.ReadCkboxAccessKey(
		ctx,
		state.Name.ValueString(),
		state.EnvId.ValueString(),
	)

	if err != nil {
		resp.Diagnostics.AddError("Unable to read Ckbox Access key for env : "+state.EnvId.ValueString(), err.Error())
		return
	}

	state.ID = types.StringValue(accessKey.Value)
	state.EnvId = types.StringValue(state.EnvId.ValueString())
	state.Name = types.StringValue(accessKey.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
