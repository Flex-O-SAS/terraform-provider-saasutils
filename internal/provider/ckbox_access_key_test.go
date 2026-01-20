package provider

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAccCkboxAccessKey_basic_mock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/auth/signin":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"accessToken":"mock-token"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/v1/subscriptions/2d681144861c/environments":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/v1/subscriptions/2d681144861c/environments":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{
				"items": [
					{"id":"abc123","name":"acc-test-env"},
					{"id":"123abc","name":"acc-test-env2"}
				]
			}`))
			return

		case r.Method == http.MethodDelete &&
			strings.HasPrefix(r.URL.Path, "/v1/subscriptions/2d681144861c/environments/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return

		// GET credentials (liste des clés)
		case r.Method == http.MethodGet &&
			strings.HasPrefix(r.URL.Path, "/v1/subscriptions/2d681144861c/environments/") &&
			strings.HasSuffix(r.URL.Path, "/credentials"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{
				"items": [
					{"name": "key1", "value": "123456789"},
					{"name": "key2", "value": "987654321"}
				]
			}`))
			return

		// POST credentials (création)
		case r.Method == http.MethodPost &&
			strings.HasPrefix(r.URL.Path, "/v1/subscriptions/2d681144861c/environments/") &&
			strings.HasSuffix(r.URL.Path, "/credentials"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return

		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	resourceNameKey := "saasutils_ckbox_access_key.example"
	envName := "acc-test-env"

	cfg := fmt.Sprintf(`
provider "saasutils" {
  email    = "fake"
  password = "fake"
  base_url = "%s/v1"
}

resource "saasutils_ckbox_env" "example" {
  name = "%s"
}

resource "saasutils_ckbox_access_key" "example" {
  env_id = "abc123"
  name   = "key1"
}
`, srv.URL, envName)

	resource.Test(t, resource.TestCase{
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"saasutils": providerserver.NewProtocol6WithError(New("test")()),
		},
		Steps: []resource.TestStep{
			{
				Config: cfg,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(resourceNameKey, "name", "key1"),
					resource.TestCheckResourceAttr(resourceNameKey, "env_id", "abc123"),
					resource.TestCheckResourceAttr(resourceNameKey, "id", "123456789"),
				),
			},
		},
	})
}
