package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// newGoBrightDataSourceMock returns a mock GoBright server whose
// GET /api/integrations/ returns `listRows`. GET /api/integrations/{id}
// returns the row from `byId` for the matching numeric id (or 404).
func newGoBrightDataSourceMock(t *testing.T, listRows []map[string]any, byId map[int64]map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/users/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`"mock-jwt-token"`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/token":
			_, _ = io.Copy(io.Discard, r.Body)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{
				"access_token": "mock-access-token",
				"refresh_token": "mock-refresh-token",
				"token_type": "bearer",
				"expires_in": 8350,
				"expires": "2099-01-01T00:00:00Z"
			}`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/api/integrations/":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(listRows)
			return

		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/integrations/"):
			suffix := strings.TrimPrefix(r.URL.Path, "/api/integrations/")
			var wantId int64
			_, err := fmt.Sscanf(suffix, "%d", &wantId)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			row, ok := byId[wantId]
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				_, _ = w.Write([]byte(`{"error":"not found"}`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode(row)
			return
		}

		t.Logf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
}

// fullOidcIntegration is a server-side full Integration body for the
// data-source's per-id Read, modeled on the user-supplied PUT example.
func fullOidcIntegration(id int64, name string) map[string]any {
	return map[string]any{
		"id":                            id,
		"newId":                         "11111111-2222-3333-4444-555555555555",
		"name":                          name,
		"externalSystem":                14,
		"microsoftPermissionMode":       nil,
		"oidcAudience":                  "fb2a534b-e8d8-4ac2-9d81-e4b0bbb4c2aa",
		"oidcIssuer":                    "https://portal.gobright.cloud/oidc",
		"oidcValidationMode":            0,
		"oidcPublicKey":                 "-----BEGIN PUBLIC KEY-----\nMIGbMBA...\n-----END PUBLIC KEY-----",
		"oidcJwksEndpoint":              "",
		"oidcUserIdentifierClaimName":   "sub",
		"oidcRelatedUserIntegrationId":  156,
		"oidcTokenReplayPreventionMode": 0,
	}
}

const dataGoBrightProviderHCL = `
provider "saasutils" {
  gobright {
    base_url          = "%s/"
    organization_code = "acc-test"
    login             = "fake"
    password          = "fake"
  }
}
`

func TestAccGoBrightIntegrationDataSource_match(t *testing.T) {
	listRows := []map[string]any{
		{"id": 156, "name": "SO365", "externalSystem": 1, "microsoftPermissionMode": 1},
		{"id": 220, "name": "test oidc local dev", "externalSystem": 14, "microsoftPermissionMode": nil},
		{"id": 244, "name": "fds", "externalSystem": 14, "microsoftPermissionMode": nil},
	}
	byId := map[int64]map[string]any{
		220: fullOidcIntegration(220, "test oidc local dev"),
	}
	srv := newGoBrightDataSourceMock(t, listRows, byId)
	defer srv.Close()

	cfg := fmt.Sprintf(dataGoBrightProviderHCL+`
data "saasutils_gobright_integration" "match" {
  name                      = "test oidc local dev"
  external_system           = "openid"
  microsoft_permission_mode = null
}
`, srv.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"saasutils": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.saasutils_gobright_integration.match", "id", "220"),
					resource.TestCheckResourceAttr("data.saasutils_gobright_integration.match", "new_id", "11111111-2222-3333-4444-555555555555"),
					resource.TestCheckResourceAttr("data.saasutils_gobright_integration.match", "oidc.audience", "fb2a534b-e8d8-4ac2-9d81-e4b0bbb4c2aa"),
					resource.TestCheckResourceAttr("data.saasutils_gobright_integration.match", "oidc.issuer", "https://portal.gobright.cloud/oidc"),
					resource.TestCheckResourceAttr("data.saasutils_gobright_integration.match", "oidc.related_user_integration_id", "156"),
				),
			},
		},
	})
}

func TestAccGoBrightIntegrationDataSource_noMatch(t *testing.T) {
	listRows := []map[string]any{
		{"id": 156, "name": "SO365", "externalSystem": 1, "microsoftPermissionMode": 1},
	}
	srv := newGoBrightDataSourceMock(t, listRows, nil)
	defer srv.Close()

	cfg := fmt.Sprintf(dataGoBrightProviderHCL+`
data "saasutils_gobright_integration" "nope" {
  name                      = "does-not-exist"
  external_system           = "openid"
  microsoft_permission_mode = null
}
`, srv.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"saasutils": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`No GoBright integration matched the filters`),
			},
		},
	})
}

func TestAccGoBrightIntegrationDataSource_multipleMatches(t *testing.T) {
	listRows := []map[string]any{
		{"id": 244, "name": "dup", "externalSystem": 14, "microsoftPermissionMode": nil},
		{"id": 245, "name": "dup", "externalSystem": 14, "microsoftPermissionMode": nil},
	}
	srv := newGoBrightDataSourceMock(t, listRows, nil)
	defer srv.Close()

	cfg := fmt.Sprintf(dataGoBrightProviderHCL+`
data "saasutils_gobright_integration" "dup" {
  name                      = "dup"
  external_system           = "openid"
  microsoft_permission_mode = null
}
`, srv.URL)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"saasutils": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config:      cfg,
				ExpectError: regexp.MustCompile(`Multiple GoBright integrations matched the filters[\s\S]*ids \[244, 245\]`),
			},
		},
	})
}
