// Copyright (c) HashiCorp, Inc.

package ckboxapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (c *APIClient) ReadCkboxEnv(ctx context.Context, name string) (*CkboxEnv, error) {
	tflog.Debug(ctx, "ReadCkboxEnv called", map[string]any{"name": name})

	var respBody CkboxEnvRespBody
	_, err := c.CallInto(ctx, "GET", "/subscriptions/"+c.GetSubscriptionId()+"/environments", nil, &respBody)
	if err != nil {
		tflog.Error(ctx, "CallInto failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	b, _ := json.Marshal(respBody)
	tflog.Debug(ctx, "env response parsed", map[string]any{"resp": string(b)})

	for i := range respBody.Items {
		env := respBody.Items[i]
		if env.Name == name {
			return &respBody.Items[i], nil
		}
	}

	return nil, fmt.Errorf("environment %q not found", name)
}

func (c *APIClient) CreateCkboxEnv(ctx context.Context, name string, region string) (*CkboxEnv, error) {
	tflog.Debug(ctx, "CreateCkboxEnv called", map[string]any{"name": name})

	_, err := c.CallInto(
		ctx,
		"POST",
		"/subscriptions/"+c.GetSubscriptionId()+"/environments",
		CkboxEnvCreateReqBody{Name: name, Region: region},
		nil,
	)

	if err != nil {
		return nil, err
	}

	env, err := c.ReadCkboxEnv(ctx, name)

	if err != nil {
		return nil, err
	}

	return env, nil
}

func (c *APIClient) DeleteCkboxEnv(ctx context.Context, id string) error {
	tflog.Debug(ctx, "DeleteCkboxEnv called", map[string]any{"id": id, "url": "/subscriptions/" + c.GetSubscriptionId() + "/environments/" + id})

	_, err := c.CallInto(
		ctx,
		"DELETE",
		"/subscriptions/"+c.GetSubscriptionId()+"/environments/"+id,
		nil,
		nil,
	)

	return err
}
