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
    base_url          = "https://t4b.gobright.cloud/"
    organization_code = "example"
    login             = "example@example.com"
    password          = "password"
  }
}

# Look up an existing OIDC integration by name + external_system.
# microsoft_permission_mode = null matches the GoBright API's null value for
# OIDC integrations.
data "saasutils_gobright_integration" "existing_oidc" {
  name                      = "test oidc local dev"
  external_system           = "openid"
  microsoft_permission_mode = null
}

output "integration_id" {
  value = data.saasutils_gobright_integration.existing_oidc.id
}
