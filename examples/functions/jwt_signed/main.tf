# Copyright (c) HashiCorp, Inc.

terraform {
  required_version = ">= 1.8.0"
  required_providers {
    saasutils = {
      source = "registry.terraform.io/flex-o-sas/saasutils"
    }
  }
}

provider "saasutils" {}

resource "time_static" "jwt_iat" {}

# This function is specially for creating ckbox auth token
locals {
  jwt = provider::saasutils::jwt_signed(
    var.environment_id, # aud
    var.access_key,     # HS256 secret
    "example-user-id",  # sub (optionnel)
    "admin",            # ckbox_role (optionnel)
    86400,              # ttl_seconds = 24h (optionnel)
    time_static.jwt_iat.unix
  )
}