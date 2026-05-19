terraform {
  required_version = ">= 1.8.0"
  required_providers {
    saasutils = {
      source = "registry.terraform.io/flex-o-sas/saasutils"
    }
  }
}

provider "saasutils" {
  gobright {
    base_url          = "https://example.gobright.cloud/"
    organization_code = "example"
    login             = "admin@example.com"
    password          = "password"
  }
}

resource "saasutils_gobright_integration" "example" {
  name            = "example-oidc"
  external_system = "openid"
  oidc {
    audience                     = "https://example.gobright.cloud"
    issuer                       = "https://issuer.example.com/"
    validation_mode              = false
    public_key                   = ""
    user_identifier_claim_name   = "sub"
    related_user_integration_id  = 0
    token_replay_prevention_mode = false
  }
}
