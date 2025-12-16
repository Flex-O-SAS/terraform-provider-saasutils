# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

terraform {
  required_version = ">= 1.8.0"
  required_providers {
    saasutils = {
      source = "registry.terraform.io/flex-o-sas/saasutils"
    }
  }
}

provider "saasutils" {}

# Example customers data
locals {
  customers = {
    customer1 = {
      name        = "customer1"
      index       = 0
      products    = ["product_a"]
      secretsFrom = null
      kind        = "customer"
      brand_name  = "Customer One"
      product_config = {
        product_a = {
          features = {
            feature1 = true
            feature2 = false
          }
          backend = {
            dns = { record = "api.customer1.com" }
          }
        }
      }
    }
    customer2 = {
      name        = "customer2"
      index       = 1
      products    = ["product_a"]
      secretsFrom = "customer1" # Inherits secrets from customer1
      kind        = "customer"
      brand_name  = "Customer Two"
      product_config = {
        product_a = {
          features = {
            feature1 = true
            feature2 = true # Override
          }
        }
      }
    }
  }

  products = {
    product_a = {
      features = {
        feature1 = false # Default: disabled
        feature2 = false # Default: disabled
        feature3 = true  # Default: enabled
      }
    }
  }

  features = {
    feature1 = {
      subfeatures = {
        subfeature_a = true
        subfeature_b = false
      }
      involved_secrets = {
        secret1 = {
          depends_on = ["subfeature_a"] # Only needed if subfeature_a is enabled
          fields     = ["api_key", "api_secret"]
        }
      }
    }
    feature2 = {
      subfeatures = {}
      involved_secrets = {
        secret2 = {
          depends_on = [] # No dependencies
          fields     = ["token"]
        }
      }
    }
    feature3 = {
      subfeatures = {
        subfeature_x = true
      }
      involved_secrets = {}
    }
  }

  customer_specific_components_secrets = {
    backend = {
      payment-salt = {
        fields = ["password"]
      }
    }
    shared = {
      api-key = {
        fields = ["key", "secret"]
      }
    }
  }
}

# Step 1: Process customers, products, and features
locals {
  customers_processed = provider::saasutils::customers_config(
    local.customers,
    local.products,
    local.features
  )
}

# Step 2: Process secrets based on customer/product configuration
locals {
  secrets_data = provider::saasutils::secrets_config(
    local.customers_processed,
    local.features,
    local.customer_specific_components_secrets
  )
}

# Output the secrets results
output "products_secrets" {
  description = "List of product/feature secrets to fetch from 1Password"
  value       = local.secrets_data.products_secrets
}

output "components_secrets" {
  description = "List of component secrets to fetch from 1Password"
  value       = local.secrets_data.components_secrets
}

output "customers_list" {
  description = "List of unique customers with secrets (for 1Password module iteration)"
  value       = local.secrets_data.customers_list
}

output "components_secrets_map_grouped" {
  description = "Component secrets grouped by customer and key (intermediate format)"
  value       = local.secrets_data.components_secrets_map_grouped
}

output "components_secrets_map" {
  description = "Component secrets formatted for 1Password module input"
  value       = local.secrets_data.components_secrets_map
}

output "products_secrets_map_grouped" {
  description = "Product secrets grouped by customer and key (intermediate format)"
  value       = local.secrets_data.products_secrets_map_grouped
}

output "products_secrets_map" {
  description = "Product secrets formatted for 1Password module input"
  value       = local.secrets_data.products_secrets_map
}
