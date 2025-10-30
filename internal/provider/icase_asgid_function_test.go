// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
	"testing"
)

func TestICaseAsgId(t *testing.T) {
	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{
			tfversion.SkipBelow(version.Must(version.NewVersion("1.8.0"))),
		},
		ProtoV6ProviderFactories: testAccProtoV6ProviderFactories,
		Steps: []resource.TestStep{
			{
				Config: `
				locals {
					input = "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-name/providers/Microsoft.Network/applicationSecurityGroups/asg-name"
				}
				output "icaseasg_false" {
					value = provider::saasutils::icase_asgid(false, local.input)
				}
				output "icaseasg_true" {
					value = provider::saasutils::icase_asgid(true, local.input)
				}
				`,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownOutputValue("icaseasg_false", knownvalue.StringExact("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/rg-name/providers/Microsoft.Network/applicationSecurityGroups/asg-name")),
					statecheck.ExpectKnownOutputValue("icaseasg_true", knownvalue.StringExact("/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/RG-NAME/providers/Microsoft.Network/applicationSecurityGroups/ASG-NAME")),
				},
			},
		},
	})
}
