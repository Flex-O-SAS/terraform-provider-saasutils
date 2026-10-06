terraform {
  required_version = ">= 1.8.0"
  required_providers {
    saasutils = {
      source = "registry.terraform.io/flex-o-sas/saasutils"
    }
    null = {
      source  = "hashicorp/null"
      version = "3.2.4"
    }
  }
}

provider "saasutils" {}

resource "time_static" "jwt_iat" {}

# This function is specially for creating ckbox auth token
locals {
  jwt = provider::saasutils::jwt_sign(
    var.environment_id, # aud
    var.access_key,     # HS256 secret
    "example-user-id",  # sub (optionnel)
    "admin",            # ckbox_role (optionnel)
    86400,              # ttl_seconds = 24h (optionnel)
    time_static.jwt_iat.unix
  )
}

variable "environment_id" {
  type = string
}
variable "access_key" {
  type = string
}

resource "null_resource" "ckbox_files_extensions" {
  depends_on = [
    time_static.jwt_iat,
  ]

  triggers = {
    jwt = local.jwt
  }

  provisioner "local-exec" {
    interpreter = ["/bin/bash", "-c"]
    command     = <<EOT
    set -euo pipefail

    curl -s -X GET "https://api.ckbox.io/admin/categories" \
      -H "Accept: application/json" \
      -H "Content-Type: application/json" \
      -H "Authorization: ${self.triggers.jwt}" | jq '(.items[] | select(.name == "Files")).extensions'
    EOT
  }
}
