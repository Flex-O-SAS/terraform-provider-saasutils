package provider
import (
	"context"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"terraform-provider-saasutils/internal/ckboxapi"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"fmt"
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

	client, ok := req.ProviderData.(*ckboxapi.APIClient)
	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Provider Data Type",
			fmt.Sprintf("Expected *ckboxapi.APIClient, got: %T", req.ProviderData),
		)
		return
	}

	d.client = client
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
		resp.Diagnostics.AddError("Unable to read Ckbox Access key for env : " + state.EnvId.ValueString(), err.Error())
		return
	}

	state.ID	= types.StringValue(accessKey.Value)
	state.EnvId	= types.StringValue(state.EnvId.ValueString())
	state.Name	= types.StringValue(accessKey.Name)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}


