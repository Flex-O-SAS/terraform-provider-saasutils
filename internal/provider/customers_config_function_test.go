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

func TestCustomersConfig(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.8.0"))),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					customers = {
						customer1 = {
							name     = "Customer One"
							products = ["theproduct"]
							product_config = {
								theproduct = {
									features = {
										feat1 = true
									}
									feature_config = {
										feat1 = {
											subfeatures = {
												sub1 = false  # Override to false
											}
										}
									}
								}
							}
						}
					}

					products = {
						theproduct = {
							features = {
								feat1 = false  # Default disabled
							}
						}
					}

					features = {
						feat1 = {
							subfeatures = {
								sub1 = true   # Default enabled
								sub2 = false  # Default disabled
							}
						}
					}

					result = provider::saasutils::customers_config(local.customers, local.products, local.features)
				}

				output "inherit_customer" {
					value = local.result.inherit_customer
				}
				output "inherit_product" {
					value = local.result.inherit_product
				}
				output "inherit_products_subfeatures" {
					value = local.result.inherit_products_subfeatures
				}
				`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownOutputValue("inherit_customer",
						knownvalue.MapPartial(map[string]knownvalue.Check{
							"customer1": knownvalue.MapPartial(map[string]knownvalue.Check{
								"name": knownvalue.StringExact("Customer One"),
							}),
						}),
					),
					statecheck.ExpectKnownOutputValue("inherit_product",
						knownvalue.MapPartial(map[string]knownvalue.Check{
							"customer1": knownvalue.MapPartial(map[string]knownvalue.Check{
								"theproduct": knownvalue.MapPartial(map[string]knownvalue.Check{
									"features": knownvalue.MapPartial(map[string]knownvalue.Check{
										"feat1": knownvalue.Bool(true),
									}),
								}),
							}),
						}),
					),
					statecheck.ExpectKnownOutputValue("inherit_products_subfeatures",
						knownvalue.MapPartial(map[string]knownvalue.Check{
							"customer1": knownvalue.MapPartial(map[string]knownvalue.Check{
								"theproduct": knownvalue.MapPartial(map[string]knownvalue.Check{
									"feature_config": knownvalue.MapPartial(map[string]knownvalue.Check{
										"feat1": knownvalue.MapPartial(map[string]knownvalue.Check{
											"subfeatures": knownvalue.MapPartial(map[string]knownvalue.Check{
												"sub1": knownvalue.Bool(false), // Customer override
												"sub2": knownvalue.Bool(false),
											}),
										}),
									}),
								}),
							}),
						}),
					),
				},
			},
		},
	})
}

func TestCustomersConfigWithSecretsFrom(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.8.0"))),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					customers = {
						base = {
							name     = "Base Customer"
							products = ["theproduct"]
							product_config = {
								theproduct = {
									features = {
										feat1 = true
									}
								}
							}
						}
						derived = {
							name        = "Derived Customer"
							products    = ["theproduct"]
							secretsFrom = "base"  # Points to base for secrets only
							product_config = {
								theproduct = {
									features = {
										feat2 = true  # Only feat2, NOT feat1
									}
								}
							}
						}
					}

					products = {
						theproduct = {
							features = {
								feat1 = false
								feat2 = false
							}
						}
					}

					features = {
						feat1 = {}
						feat2 = {}
					}

					result = provider::saasutils::customers_config(local.customers, local.products, local.features)
				}

				output "derived_customer" {
					value = local.result.inherit_customer.derived
				}
				output "derived_product" {
					value = local.result.inherit_product.derived
				}
				`,
				ConfigStateChecks: []statecheck.StateCheck{
					// secretsFrom sets secret_module_name to point to base
					statecheck.ExpectKnownOutputValue("derived_customer",
						knownvalue.MapPartial(map[string]knownvalue.Check{
							"secret_module_name": knownvalue.StringExact("base"),
						}),
					),
					// secretsFrom does NOT inherit features - only feat2 should be enabled
					statecheck.ExpectKnownOutputValue("derived_product",
						knownvalue.MapPartial(map[string]knownvalue.Check{
							"theproduct": knownvalue.MapPartial(map[string]knownvalue.Check{
								"features": knownvalue.MapPartial(map[string]knownvalue.Check{
									"feat1": knownvalue.Bool(false), // NOT inherited
									"feat2": knownvalue.Bool(true),  // Only this one
								}),
							}),
						}),
					),
				},
			},
		},
	})
}

func TestCustomersConfigEmptyInputs(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.8.0"))),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					result = provider::saasutils::customers_config({}, {}, {})
				}

				output "inherit_customer" {
					value = local.result.inherit_customer
				}
				output "inherit_product" {
					value = local.result.inherit_product
				}
				output "inherit_products_subfeatures" {
					value = local.result.inherit_products_subfeatures
				}
				`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownOutputValue("inherit_customer", knownvalue.MapExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownOutputValue("inherit_product", knownvalue.MapExact(map[string]knownvalue.Check{})),
					statecheck.ExpectKnownOutputValue("inherit_products_subfeatures", knownvalue.MapExact(map[string]knownvalue.Check{})),
				},
			},
		},
	})
}
