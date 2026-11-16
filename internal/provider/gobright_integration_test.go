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
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"terraform-provider-saasutils/internal/gobrightapi"
)

// mockedIntegration is the canonical server-side representation returned by
// the mock from GET /api/integrations/{id}. Keys are PascalCase and the
// public key is masked, as the real GoBright API returns them. A handful of
// unmodeled fields (SamlEntityId, AutoCreateUser, ...) are included to
// exercise the IntegrationRaw round-trip in Update.
func mockedIntegration() map[string]any {
	return map[string]any{
		"Id":                            220,
		"NewId":                         "11111111-2222-3333-4444-555555555555",
		"Name":                          "acc-test-integration",
		"ExternalSystem":                14,
		"OidcAudience":                  "https://example.gobright.cloud",
		"OidcIssuer":                    "https://issuer.example.com/",
		"OidcValidationMode":            1,
		"OidcPublicKey":                 "******",
		"OidcJwksEndpoint":              "https://issuer.example.com/.well-known/jwks.json",
		"OidcUserIdentifierClaimName":   "sub",
		"OidcRelatedUserIntegrationId":  0,
		"OidcTokenReplayPreventionMode": 0,
		// Fields outside the Terraform schema — must round-trip on Update.
		"SamlEntityId":   "https://www.gobright.com/sso/11111111-2222-3333-4444-555555555555",
		"AutoCreateUser": false,
		"ExchangeAuthenticationDifferentUsername": false,
		"MicrosoftPermissionMode":                 nil,
	}
}

// newGoBrightMock starts a fake GoBright API. onPut, when not nil, receives
// the decoded body of every PUT /api/integrations/.
func newGoBrightMock(t *testing.T, deleted *atomic.Bool, onPut func(map[string]any)) *httptest.Server {
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
			body := map[string]any{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode PUT body: %v", err)
			}
			if onPut != nil {
				onPut(body)
			}
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
	var lastPut atomic.Pointer[map[string]any]
	srv := newGoBrightMock(t, &deleted, func(body map[string]any) { lastPut.Store(&body) })
	defer srv.Close()

	resourceName := "saasutils_gobright_integration.example"

	cfg := func(publicKey string) string {
		return fmt.Sprintf(`
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
    public_key                   = %q
    user_identifier_claim_name   = "sub"
    related_user_integration_id  = 0
    token_replay_prevention_mode = false
  }
}
`, srv.URL, publicKey)
	}

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"saasutils": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				// The API masks the public key: the state keeps the configured one.
				Config: cfg("public-key-1"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "id", "220"),
					resource.TestCheckResourceAttr(resourceName, "new_id", "11111111-2222-3333-4444-555555555555"),
					resource.TestCheckResourceAttr(resourceName, "oidc.jwks_endpoint", "https://issuer.example.com/.well-known/jwks.json"),
					resource.TestCheckResourceAttr(resourceName, "name", "acc-test-integration"),
					resource.TestCheckResourceAttr(resourceName, "oidc.public_key", "public-key-1"),
				),
			},
			{
				// A key rotation is pushed with a PUT that carries each field once.
				Config: cfg("public-key-2"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceName, "oidc.public_key", "public-key-2"),
					func(_ *terraform.State) error {
						body := lastPut.Load()
						if body == nil {
							return fmt.Errorf("no PUT received")
						}
						return checkPutBody(*body, "public-key-2")
					},
				),
			},
		},
	})
}

// checkPutBody fails when a key appears twice with a different case (the
// server reads one of them at random) or when the public key sent is not the
// configured one. Unmodeled fields must still be there.
func checkPutBody(body map[string]any, publicKey string) error {
	seen := map[string]string{}
	for k := range body {
		lk := strings.ToLower(k)
		if prev, ok := seen[lk]; ok {
			return fmt.Errorf("PUT body has duplicate keys %q and %q", prev, k)
		}
		seen[lk] = k
	}
	if got := body[seen["oidcpublickey"]]; got != publicKey {
		return fmt.Errorf("PUT body public key = %v, want %q", got, publicKey)
	}
	if _, ok := seen["samlentityid"]; !ok {
		return fmt.Errorf("PUT body lost unmodeled field SamlEntityId")
	}
	return nil
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
