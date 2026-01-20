// Copyright (c) HashiCorp, Inc.

package ckboxapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func (c *APIClient) ReadCkboxAccessKey(ctx context.Context, name, envId string) (*CkboxAccesKey, error) {
	tflog.Debug(ctx, "ReadCkboxAccessKey called", map[string]any{"name": name, "envId": envId})

	var respBody CkboxReadAccesKeyRespBody
	c.SetHeader("organizationid", "b9ee06c380fb")
	defer c.UnsetHeader("organizationid")

	_, err := c.CallInto(
		ctx,
		"GET",
		"/subscriptions/2d681144861c/environments/"+envId+"/credentials",
		nil,
		&respBody,
	)

	if err != nil {
		tflog.Error(ctx, "CallInto failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	b, _ := json.Marshal(respBody)
	tflog.Debug(ctx, "access key response parsed", map[string]any{"resp": string(b)})

	for i := range respBody.Items {
		accessKey := respBody.Items[i]
		if accessKey.Name == name {
			return &respBody.Items[i], nil
		}
	}

	return nil, fmt.Errorf("access key %q not found", name)
}

func (c *APIClient) CreateCkboxAccessKey(ctx context.Context, name, envId string) (*CkboxAccesKey, error) {
	tflog.Debug(ctx, "CreateCkboxAccessKey called", map[string]any{"name": name, "envId": envId})

	c.SetHeader("organizationid", "b9ee06c380fb")
	defer c.UnsetHeader("organizationid")

	_, err := c.CallInto(
		ctx,
		"POST",
		"/subscriptions/2d681144861c/environments/"+envId+"/credentials",
		CkboxCreateAccessKeyReqBody{Name: name},
		nil,
	)

	if err != nil {
		tflog.Error(ctx, "CallInto failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	accessKey, err := c.ReadCkboxAccessKey(ctx, name, envId)

	if err != nil {
		tflog.Error(ctx, "CallInto failed", map[string]any{"error": err.Error()})
		return nil, err
	}

	return accessKey, nil
}

func (c *APIClient) DeleteCkboxAccessKey(ctx context.Context, name, envId, token string) error {
	tflog.Debug(ctx, "CreateCkboxAccessKey called", map[string]any{"name": name})

	c.SetHeader("organizationid", "b9ee06c380fb")
	defer c.UnsetHeader("organizationid")

	_, err := c.CallInto(
		ctx,
		"DELETE",
		"/subscriptions/2d681144861c/environments/"+envId+"/credentials/"+token,
		nil,
		nil,
	)

	if err != nil {
		tflog.Error(ctx, "CallInto failed", map[string]any{"error": err.Error()})
		return err
	}

	return nil
}
