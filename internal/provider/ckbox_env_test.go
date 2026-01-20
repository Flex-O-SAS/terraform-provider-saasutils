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

func TestAccCkboxEnv_basic_mock(t *testing.T) {
	// 1) Mock HTTP server
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/auth/signin":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"accessToken":"mock-token"}`))
			return

		case r.Method == http.MethodPost && r.URL.Path == "/v1/subscriptions/2d681144861c/environments":
			// Create env
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
			return

		case r.Method == http.MethodGet && r.URL.Path == "/v1/subscriptions/2d681144861c/environments":
			// Read/list envs => doit contenir "acc-test-env"
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{
				"items": [
					{"id":"abc123","name":"acc-test-env"},
					{"id":"123abc","name":"acc-test-env2"}
				]
			}`))
			return
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/v1/subscriptions/2d681144861c/environments/"):
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			_, _ = w.Write([]byte(`{"ok":true}`))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer srv.Close()

	// 2) Terraform config (base_url pointe sur le mock)
	envName := "acc-test-env"
	resourceName := "saasutils_ckbox_env.example"

	cfg := fmt.Sprintf(`
provider "saasutils" {
  email    = "fake"
  password = "fake"
  base_url = "%s/v1"
}

resource "saasutils_ckbox_env" "example" {
  name = "%s"
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
					resource.TestCheckResourceAttr(resourceName, "name", envName),
					resource.TestCheckResourceAttrSet(resourceName, "id"),
				),
			},
		},
	})
}
