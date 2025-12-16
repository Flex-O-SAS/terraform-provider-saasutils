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
      secretsFrom = "customer1" # Inherits from customer1
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
          depends_on = ["subfeature_a"]
          fields     = ["api_key", "api_secret"]
        }
      }
    }
    feature2 = {
      subfeatures      = {}
      involved_secrets = {}
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

# Call the customers_config function (processes customers, products, and features)
locals {
  config = provider::saasutils::customers_config(
    local.customers,
    local.products,
    local.features
  )
}

# Output the results (customer/product processing only)
output "inherit_customer" {
  description = "Customers with inheritance applied (secretsFrom resolved)"
  value       = local.config.inherit_customer
}

output "inherit_product" {
  description = "Customers with product features merged (product defaults + customer overrides)"
  value       = local.config.inherit_product
}

output "inherit_products_subfeatures" {
  description = "Customers with subfeatures enhanced (feature_config merged with feature defaults)"
  value       = local.config.inherit_products_subfeatures
}
