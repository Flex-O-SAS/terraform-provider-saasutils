// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"testing"

	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

func TestCustomersSecrets(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.8.0"))),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					# Simulate output from customers_config
					customers_processed = {
						inherit_customer = {
							customer1 = {
								name               = "Customer One"
								secret_module_name = "customer1"
							}
						}
						inherit_product = {
							customer1 = {
								theproduct = {
									features = {
										feat1 = true
									}
								}
							}
						}
						inherit_products_subfeatures = {
							customer1 = {
								theproduct = {
									features = {
										feat1 = true
									}
									feature_config = {}
								}
							}
						}
					}

					features = {
						feat1 = {
							involved_secrets = {
								secret1 = {
									fields     = ["api_key", "api_secret"]
									depends_on = []
								}
							}
						}
					}

					component_customers_secrets = {
						component1 = {
							secret_comp1 = {
								fields = ["comp_field1"]
							}
						}
					}

					result = provider::saasutils::secrets_config(
						local.customers_processed,
						local.features,
						local.component_customers_secrets
					)
				}

				output "products_secrets" {
					value = local.result.products_secrets
				}
				output "components_secrets" {
					value = local.result.components_secrets
				}
				output "customers_list" {
					value = local.result.customers_list
				}
				`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownOutputValue("products_secrets",
						knownvalue.ListPartial(map[int]knownvalue.Check{
							0: knownvalue.MapPartial(map[string]knownvalue.Check{
								"customer": knownvalue.StringExact("Customer One"),
								"feature":  knownvalue.StringExact("feat1"),
								"secret":   knownvalue.StringExact("secret1"),
							}),
						}),
					),
					statecheck.ExpectKnownOutputValue("components_secrets",
						knownvalue.ListPartial(map[int]knownvalue.Check{
							0: knownvalue.MapPartial(map[string]knownvalue.Check{
								"customer":  knownvalue.StringExact("Customer One"),
								"component": knownvalue.StringExact("component1"),
							}),
						}),
					),
					statecheck.ExpectKnownOutputValue("customers_list",
						knownvalue.ListExact([]knownvalue.Check{
							knownvalue.StringExact("Customer One"),
						}),
					),
				},
			},
		},
	})
}

func TestCustomersSecretsEmptySecrets(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.8.0"))),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					customers_processed = {
						inherit_customer = {
							customer1 = {
								name               = "Customer One"
								secret_module_name = "customer1"
							}
						}
						inherit_product = {
							customer1 = {
								theproduct = {
									features = { feat1 = true }
								}
							}
						}
						inherit_products_subfeatures = {
							customer1 = {
								theproduct = {
									features       = { feat1 = true }
									feature_config = {}
								}
							}
						}
					}

					features = {
						feat1 = {}  # No involved_secrets
					}

					result = provider::saasutils::secrets_config(
						local.customers_processed,
						local.features,
						{}
					)
				}

				output "products_secrets" {
					value = local.result.products_secrets
				}
				output "components_secrets" {
					value = local.result.components_secrets
				}
				output "customers_list" {
					value = local.result.customers_list
				}
				`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownOutputValue("products_secrets", knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownOutputValue("components_secrets", knownvalue.ListExact([]knownvalue.Check{})),
					statecheck.ExpectKnownOutputValue("customers_list", knownvalue.ListExact([]knownvalue.Check{})),
				},
			},
		},
	})
}
