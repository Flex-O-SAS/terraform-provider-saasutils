package gobrightapi

import "encoding/json"

// LoginReqBody is the JSON body sent to /api/users/login during stage 1 of
// auth. authFlowId must serialize as JSON null, so we use json.RawMessage.
type LoginReqBody struct {
	ClientType           int             `json:"clientType"`
	TenantType           int             `json:"tenantType"`
	EmailAddress         string          `json:"emailAddress"`
	OrganisationCode     string          `json:"organisationCode"`
	Password             string          `json:"password"`
	ExternalAuthProvider string          `json:"externalAuthProvider"`
	AuthFlowId           json.RawMessage `json:"authFlowId"`
}

// TokenResp is the response of stage 2 (/token).
type TokenResp struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	Expires      string `json:"expires"`
}

// Integration is the typed view of the fields the saasutils_gobright_integration
// resource and data source expose. The full GoBright body has many more fields
// (SAML, Office365/Exchange, locker, sensoring, ...); those round-trip through
// Update via IntegrationRaw so unmodeled state isn't nulled out.
//
// id and newId are server-managed and omitted from request bodies when zero.
//
// MicrosoftPermissionMode is a pointer because GoBright returns it as JSON
// `null` for integrations that don't apply it (e.g. OIDC = externalSystem 14)
// and as an integer for Microsoft-flavoured integrations. The data source
// uses it as a filter; the resource does not surface it but round-trips the
// existing server-side value via IntegrationRaw on Update.
type Integration struct {
	Id                            int64  `json:"id,omitempty"`
	NewId                         string `json:"newId,omitempty"`
	Name                          string `json:"name"`
	ExternalSystem                int64  `json:"externalSystem"`
	MicrosoftPermissionMode       *int64 `json:"microsoftPermissionMode"`
	OidcAudience                  string `json:"oidcAudience"`
	OidcIssuer                    string `json:"oidcIssuer"`
	OidcValidationMode            int64  `json:"oidcValidationMode"`
	OidcPublicKey                 string `json:"oidcPublicKey"`
	OidcJwksEndpoint              string `json:"oidcJwksEndpoint"`
	OidcUserIdentifierClaimName   string `json:"oidcUserIdentifierClaimName"`
	OidcRelatedUserIntegrationId  int64  `json:"oidcRelatedUserIntegrationId"`
	OidcTokenReplayPreventionMode int64  `json:"oidcTokenReplayPreventionMode"`
}

// IntegrationRaw is the full server-side Integration body, kept opaque as a
// JSON object. Used during Update so fields outside the Terraform schema
// (samlEntityId, lockerVecos*, sensoringIntegration, microsoftPermissionMode,
// ...) survive the round-trip without being nulled out.
type IntegrationRaw map[string]any
