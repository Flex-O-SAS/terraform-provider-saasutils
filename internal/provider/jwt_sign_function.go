package provider

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ function.Function = &JwtSignFunction{}

type JwtSignFunction struct{}

func NewJwtSignedFunction() function.Function {
	return &JwtSignFunction{}
}

func (f *JwtSignFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "jwt_sign"
}

func (f *JwtSignFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Generate and sign CKEditor Cloud JWT (HS256)",
		Description: "Builds a JWT with aud=Environment ID and signs it with HS256 using the Access key. Optionally includes sub, CKBox role, and exp via ttl_seconds.",

		Parameters: []function.Parameter{
			function.StringParameter{
				Name:        "environment_id",
				Description: "Cloud Environment ID (maps to aud).",
			},
			function.StringParameter{
				Name:        "access_key",
				Description: "Access key used as HS256 secret (signature key).",
			},
			function.StringParameter{
				Name:        "sub",
				Description: "Optional user id (sub). If empty or null, omitted.",
			},
			function.StringParameter{
				Name:        "ckbox_role",
				Description: "Optional CKBox role (auth.ckbox.role), e.g. admin. If empty or null, auth is omitted.",
			},
			function.Int64Parameter{
				Name:        "ttl_seconds",
				Description: "Optional token TTL in seconds. If > 0, exp = iat + ttl_seconds is added. Default: 0 (no exp).",
			},
			function.Int64Parameter{
				Name:        "iat_unix",
				Description: "Optional issued-at unix seconds. If set, used instead of current time.",
			},
		},

		Return: function.StringReturn{},
	}
}

func (f *JwtSignFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var environmentID, accessKey types.String
	var sub, ckboxRole types.String
	var ttlSeconds types.Int64
	var iatUnix types.Int64

	resp.Error = function.ConcatFuncErrors(resp.Error,
		req.Arguments.Get(ctx, &environmentID, &accessKey, &sub, &ckboxRole, &ttlSeconds, &iatUnix),
	)
	if resp.Error != nil {
		return
	}

	// Validate required inputs
	if environmentID.IsUnknown() || accessKey.IsUnknown() {
		resp.Error = function.NewFuncError("environment_id and access_key must be known")
		return
	}
	if environmentID.IsNull() || accessKey.IsNull() || environmentID.ValueString() == "" || accessKey.ValueString() == "" {
		resp.Error = function.NewFuncError("environment_id and access_key are required and cannot be empty")
		return
	}

	iat := time.Now().UTC().Unix()
	if !iatUnix.IsNull() && !iatUnix.IsUnknown() {
		iat = iatUnix.ValueInt64()
	}

	// Build payload
	payload := map[string]any{
		"aud": environmentID.ValueString(),
		"iat": iat,
	}

	// Optional sub
	if !sub.IsNull() && !sub.IsUnknown() {
		if s := sub.ValueString(); s != "" {
			payload["sub"] = s
		}
	}

	// Optional auth.ckbox.role
	if !ckboxRole.IsNull() && !ckboxRole.IsUnknown() {
		if r := ckboxRole.ValueString(); r != "" {
			payload["auth"] = map[string]any{
				"ckbox": map[string]any{
					"role": r,
				},
			}
		}
	}

	// Optional exp
	if !ttlSeconds.IsNull() && !ttlSeconds.IsUnknown() {
		ttl := ttlSeconds.ValueInt64()
		if ttl > 0 {
			payload["exp"] = iat + ttl
		} else if ttl < 0 {
			resp.Error = function.NewFuncError("ttl_seconds cannot be negative")
			return
		}
	}

	// Header
	header := map[string]any{
		"alg": "HS256",
		"typ": "JWT",
	}

	headerJSON, err := json.Marshal(header)
	if err != nil {
		resp.Error = function.NewFuncError(fmt.Sprintf("marshal header: %v", err))
		return
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		resp.Error = function.NewFuncError(fmt.Sprintf("marshal payload: %v", err))
		return
	}

	encHeader := base64.RawURLEncoding.EncodeToString(headerJSON)
	encPayload := base64.RawURLEncoding.EncodeToString(payloadJSON)

	signingInput := encHeader + "." + encPayload
	sig := hmacSHA256Base64URL(signingInput, accessKey.ValueString())

	token := signingInput + "." + sig

	resp.Error = function.ConcatFuncErrors(resp.Error,
		resp.Result.Set(ctx, types.StringValue(token)),
	)
}

func hmacSHA256Base64URL(message, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
