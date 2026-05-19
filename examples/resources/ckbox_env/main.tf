terraform {
  required_version = ">= 1.8.0"
  required_providers {
    saasutils = {
      source = "registry.terraform.io/flex-o-sas/saasutils"
    }
  }
}

provider "saasutils" {
  ckbox {
    email           = "example@example.com"
    password        = "password"
    organization_id = "example"
    subscription_id = "example"
    base_url        = "https://example.com/"
  }
}

resource "saasutils_ckbox_env" "example" {
  name = "example"
}
