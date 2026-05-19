package gobrightapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

// Hard-coded vendor magic constants. See Design Notes in
// _bmad-output/implementation-artifacts/spec-gobright-integration.md.
const (
	loginClientType           = 6
	loginTenantType           = 1
	loginExternalAuthProvider = "0"

	tokenGrantType                = "password"
	tokenClientID                 = "AngularWebApp"
	tokenClientSecret             = "c1ef6678-e47e-487b-935c-e9507992cee4"
	tokenPasswordUseRefreshTokens = "true"

	headerAuth     = "authorization"
	headerAuthType = "x-authentication-type"

	authTypePassword = "Password"
	authTypeToken    = "Token"

	defaultUserAgent = "Bright 2"
)

// Authenticate performs the 2-step GoBright auth flow:
//
//  1. POST /api/users/login (JSON) returning a bare JWT string.
//  2. POST /token (form-encoded) returning an access_token.
//
// On success, sets Authorization: Bearer <access_token> and User-agent: Bright 2
// on the client's default headers so subsequent requests inherit them.
func (c *APIClient) Authenticate(ctx context.Context, organisationCode, login, password string) (string, error) {
	tflog.Debug(ctx, "GoBright Authenticate stage 1: /api/users/login")

	stage1Body := LoginReqBody{
		ClientType:           loginClientType,
		TenantType:           loginTenantType,
		EmailAddress:         login,
		OrganisationCode:     organisationCode,
		Password:             password,
		ExternalAuthProvider: loginExternalAuthProvider,
		AuthFlowId:           json.RawMessage("null"),
	}

	stage1Bytes, err := json.Marshal(stage1Body)
	if err != nil {
		return "", fmt.Errorf("GoBright login failed: %w", err)
	}

	stage1Headers := map[string]string{
		headerAuth:     "1",
		"Content-Type": "application/json",
		headerAuthType: authTypePassword,
	}

	respBody, _, err := c.doWithHeaders(ctx, http.MethodPost, "/api/users/login", stage1Headers, stage1Bytes)
	if err != nil {
		return "", fmt.Errorf("GoBright login failed: %w", err)
	}

	// The response is a bare JSON string (e.g. "eyJhbG..."). Unmarshal into a
	// string.
	var jwt string
	if err := json.Unmarshal(respBody, &jwt); err != nil {
		return "", fmt.Errorf("GoBright login failed: decode response: %w (body=%s)", err, string(respBody))
	}

	tflog.Debug(ctx, "GoBright Authenticate stage 2: /token")

	form := url.Values{}
	form.Set("grant_type", tokenGrantType)
	form.Set("client_id", tokenClientID)
	form.Set("client_secret", tokenClientSecret)
	form.Set("passwordUseRefreshTokens", tokenPasswordUseRefreshTokens)
	form.Set("password", jwt)

	stage2Headers := map[string]string{
		headerAuth:     "1",
		"Content-Type": "application/x-www-form-urlencoded",
		headerAuthType: authTypeToken,
		"expires":      "Sat, 01 Jan 2000 00:00:00 GMT",
	}

	tokenBytes, _, err := c.doForm(ctx, http.MethodPost, "/token", stage2Headers, form)
	if err != nil {
		return "", fmt.Errorf("GoBright token exchange failed: %w", err)
	}

	var tokenResp TokenResp
	if err := json.Unmarshal(tokenBytes, &tokenResp); err != nil {
		return "", fmt.Errorf("GoBright token exchange failed: decode response: %w (body=%s)", err, string(tokenBytes))
	}

	if tokenResp.AccessToken == "" {
		return "", fmt.Errorf("GoBright token exchange failed: empty access_token (body=%s)", string(tokenBytes))
	}

	c.SetHeader("Authorization", "Bearer "+tokenResp.AccessToken)
	c.SetHeader("User-agent", defaultUserAgent)

	tflog.Debug(ctx, "GoBright Authenticate succeeded", map[string]any{
		"accessToken_present": tokenResp.AccessToken != "",
	})

	return tokenResp.AccessToken, nil
}
