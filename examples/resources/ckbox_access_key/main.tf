# Copyright (c) HashiCorp, Inc.

terraform {
  required_version = ">= 1.8.0"
  required_providers {
    saasutils = {
      source = "registry.terraform.io/flex-o-sas/saasutils"
    }
  }
}

provider "saasutils" {
  email    = "example@example.com"
  password = "password"
}

resource "saasutils_ckbox_env" "example" {
  name = "example"
}

resource "saasutils_ckbox_access_key" "example" {
  env_id = saasutils_ckbox_env.example.id
  name   = "example"
}