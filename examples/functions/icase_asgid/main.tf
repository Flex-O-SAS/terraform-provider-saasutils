# Copyright (c) HashiCorp, Inc.

terraform {
  required_providers {
    saasutils = {
      source = "registry.terraform.io/flex-o-sas/saasutils"
    }
  }
  required_version = ">= 1.8.0"
}

provider "saasutils" {}

locals {
  input = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-name/providers/Microsoft.Network/applicationSecurityGroups/asg-name"
}

output "asgid" {
  value = provider::saasutils::icase_asgid(false, local.input)
  // "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-name/providers/Microsoft.Network/applicationSecurityGroups/asg-name"
}

output "asgid-with-case" {
  value = provider::saasutils::icase_asgid(true, local.input)
  // "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/RG-NAME/providers/Microsoft.Network/applicationSecurityGroups/ASG-NAME"
}
