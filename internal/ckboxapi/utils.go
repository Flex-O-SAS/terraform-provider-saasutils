// Copyright (c) HashiCorp, Inc.

package ckboxapi

import (
	"context"
	"github.com/hashicorp/terraform-plugin-log/tflog"
	"net/http"
)

type AuthenticateRespBody struct {
	AccessToken string "json:accessToken"
}

type AuthenticateReqBody struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (c *APIClient) Authenticate(ctx context.Context, email string, password string) (string, error) {
	var resp AuthenticateRespBody

	_, err := c.CallInto(
		ctx,
		http.MethodPost,
		"/auth/signin",
		AuthenticateReqBody{Email: email, Password: password},
		&resp,
	)

	if err != nil {
		return "", err
	}

	tflog.Debug(ctx, "Authentication succeeded",
		map[string]any{
			"accessToken_present": resp.AccessToken != "",
		},
	)

	c.SetHeader("Authorization", resp.AccessToken)
	return resp.AccessToken, nil
}
