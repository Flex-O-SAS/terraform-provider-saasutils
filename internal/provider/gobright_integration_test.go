package provider

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"terraform-provider-saasutils/internal/gobrightapi"
)

// mockedIntegration is the canonical server-side representation returned by
// the mock from GET /api/integrations/{id}. Keys are camelCase to match the
// real GoBright API. A handful of unmodeled fields (samlEntityId, autoCreateUser,
// ...) are included to exercise the IntegrationRaw round-trip in Update.
func mockedIntegration() map[string]any {
	return map[string]any{
		"id":                            220,
		"newId":                         "11111111-2222-3333-4444-555555555555",
		"name":                          "acc-test-integration",
		"externalSystem":                14,
		"oidcAudience":                  "https://example.gobright.cloud",
		"oidcIssuer":                    "https://issuer.example.com/",
		"oidcValidationMode":            1,
		"oidcPublicKey":                 "",
		"oidcJwksEndpoint":              "https://issuer.example.com/.well-known/jwks.json",
		"oidcUserIdentifierClaimName":   "sub",
		"oidcRelatedUserIntegrationId":  0,
		"oidcTokenReplayPreventionMode": 0,
		// Fields outside the Terraform schema — must round-trip on Update.
		"samlEntityId":   "https://www.gobright.com/sso/11111111-2222-3333-4444-555555555555",
		"autoCreateUser": false,
		"exchangeAuthenticationDifferentUsername": false,
		"microsoftPermissionMode":                 nil,
	}
}

func newGoBrightMock(t *testing.T, deleted *atomic.Bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		// Stage 1: login returns a bare JSON string (the JWT).
		case r.Method == http.MethodPost && r.URL.Path == "/api/users/login":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`"mock-jwt-token"`))
			return

		// Stage 2: token returns access_token et al.
		case r.Method == http.MethodPost && r.URL.Path == "/token":
			// Drain the form body so the request is well-formed.
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

		// Create: GoBright responds with 201 and no body. The client must
		// discover the new id via the list endpoint below.
		case r.Method == http.MethodPost && r.URL.Path == "/api/integrations":
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusCreated)
			return

		// List: GET /api/integrations/ (trailing slash). Returns a few rows
		// including two with the target name so the max-id heuristic in
		// CreateIntegration is actually exercised.
		case r.Method == http.MethodGet && r.URL.Path == "/api/integrations/":
			if deleted != nil && deleted.Load() {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				_, _ = w.Write([]byte(`[]`))
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"id": 50, "name": "different-name", "externalSystem": 1},
				{"id": 100, "name": "acc-test-integration", "externalSystem": 14},
				{"id": 220, "name": "acc-test-integration", "externalSystem": 14},
			})
			return

		// Update: PUT lives at /api/integrations/ (no id in path; id is in body).
		case r.Method == http.MethodPut && r.URL.Path == "/api/integrations/":
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(http.StatusNoContent)
			return

		// Read / Delete by id.
		case strings.HasPrefix(r.URL.Path, "/api/integrations/") && r.URL.Path != "/api/integrations/":
			switch r.Method {
			case http.MethodGet:
				if deleted != nil && deleted.Load() {
					w.WriteHeader(http.StatusNotFound)
					_, _ = w.Write([]byte(`{"error":"not found"}`))
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(200)
				_ = json.NewEncoder(w).Encode(mockedIntegration())
				return
			case http.MethodDelete:
				if deleted != nil {
					deleted.Store(true)
				}
				w.WriteHeader(http.StatusNoContent)
				return
			}
		}

		t.Logf("unexpected request: %s %s", r.Method, r.URL.Path)
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
}

func TestAccGoBrightIntegration_basic_mock(t *testing.T) {
	var deleted atomic.Bool
	srv := newGoBrightMock(t, &deleted)
	defer srv.Close()

	resourceName := "saasutils_gobright_integration.example"

	cfg := fmt.Sprintf(`
provider "saasutils" {
  gobright {
    base_url          = "%s/"
    organization_code = "acc-test"
    login             = "fake"
    password          = "fake"
  }
}

resource "saasutils_gobright_integration" "example" {
  name            = "acc-test-integration"
  external_system = "openid"
  oidc {
    audience                     = "https://example.gobright.cloud"
    issuer                       = "https://issuer.example.com/"
    validation_mode              = true
    public_key                   = ""
    user_identifier_claim_name   = "sub"
    related_user_integration_id  = 0
    token_replay_prevention_mode = false
  }
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
					resource.TestCheckResourceAttr(resourceName, "id", "220"),
					resource.TestCheckResourceAttr(resourceName, "new_id", "11111111-2222-3333-4444-555555555555"),
					resource.TestCheckResourceAttr(resourceName, "oidc.jwks_endpoint", "https://issuer.example.com/.well-known/jwks.json"),
					resource.TestCheckResourceAttr(resourceName, "name", "acc-test-integration"),
				),
			},
		},
	})
}

// TestGoBrightReadIntegrationNotFound verifies that a server-side 404 during
// Read surfaces an error whose message contains "not found", which the
// resource layer matches via strings.Contains to drive drift detection.
func TestGoBrightReadIntegrationNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"gone"}`))
	}))
	defer srv.Close()

	// Talk to the gobrightapi client directly — no terraform involved.
	client := gobrightapi.NewClient(srv.URL, "acc-test", 10*time.Second)
	_, err := client.ReadIntegration(t.Context(), 220)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' substring, got %q", err.Error())
	}
}
