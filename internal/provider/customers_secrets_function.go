// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ function.Function = &CustomersSecretsFunction{}

type CustomersSecretsFunction struct{}

func NewCustomersSecretsFunction() function.Function {
	return &CustomersSecretsFunction{}
}

func (f *CustomersSecretsFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "secrets_config"
}

func (f *CustomersSecretsFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Compute secrets lists for customer features and components",
		Description: "This function handles the secrets processing domain, taking processed customer/product data as input.",
		Parameters: []function.Parameter{
			function.DynamicParameter{
				Name: "customers_config",
				Description: "Output from customers_config function. Contains: inherit_customer, " +
					"inherit_product, and inherit_products_subfeatures maps.",
			},
			function.DynamicParameter{
				Name: "features",
				Description: "Map of feature definitions. Each feature can have: subfeatures (map of bools), " +
					"required_config_params (list of strings), and involved_secrets (map with fields and depends_on).",
			},
			function.DynamicParameter{
				Name:        "component_secrets",
				Description: "Map of component-specific secrets configuration. Structure: component -> secret -> {fields: []}",
			},
		},
		Return: function.DynamicReturn{},
	}
}

func (f *CustomersSecretsFunction) Run(
	ctx context.Context,
	req function.RunRequest,
	resp *function.RunResponse,
) {
	var customersConfig, features, componentCustomersSecrets basetypes.DynamicValue

	resp.Error = function.ConcatFuncErrors(
		req.Arguments.Get(ctx, &customersConfig, &features, &componentCustomersSecrets),
	)
	if resp.Error != nil {
		return
	}

	// Convert Terraform dynamic values to Go maps
	customersConfigData, err := dynamicToMap(customersConfig)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(
			resp.Error,
			function.NewFuncError(fmt.Sprintf("failed to parse customers_config: %v", err)),
		)
		return
	}

	featuresData, err := dynamicToMap(features)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(
			resp.Error,
			function.NewFuncError(fmt.Sprintf("failed to parse features: %v", err)),
		)
		return
	}

	componentCustomersSecretsData, err := dynamicToMap(componentCustomersSecrets)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(
			resp.Error,
			function.NewFuncError(fmt.Sprintf("failed to parse component_secrets: %v", err)),
		)
		return
	}

	result, err := executeCustomersSecrets(
		customersConfigData,
		featuresData,
		componentCustomersSecretsData,
	)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(
			resp.Error,
			function.NewFuncError(fmt.Sprintf("secrets processing failed: %v", err)),
		)
		return
	}

	// Convert result to dynamic value and set response
	resultDynamic, err := mapToDynamic(result)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(
			resp.Error,
			function.NewFuncError(fmt.Sprintf("failed to convert result: %v", err)),
		)
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, resultDynamic))
}

// executeCustomersSecrets processes customer secrets.
func executeCustomersSecrets(
	customersProcessed map[string]interface{},
	features map[string]interface{},
	componentCustomersSecrets map[string]interface{},
) (map[string]interface{}, error) {
	inheritCustomer := getMap(customersProcessed, "inherit_customer")
	inheritProductsSubfeatures := getMap(customersProcessed, "inherit_products_subfeatures")

	productsSecrets, err := computeFeatureSecretsList(inheritProductsSubfeatures, features, inheritCustomer)
	if err != nil {
		return nil, fmt.Errorf("feature_secrets: %w", err)
	}

	componentsSecrets := computeComponentSecretsList(inheritCustomer, componentCustomersSecrets)

	aggregatedSecrets := computeSecretsAggregation(productsSecrets, componentsSecrets)

	result := map[string]interface{}{
		"products_secrets":               productsSecrets,
		"components_secrets":             componentsSecrets,
		"customers_list":                 aggregatedSecrets["customersWithNonInheritedSecrets"],
		"components_secrets_map_grouped": aggregatedSecrets["customersComponentsSecretsMapGrouped"],
		"components_secrets_map":         aggregatedSecrets["customersComponentsSecretsMap"],
		"products_secrets_map_grouped":   aggregatedSecrets["customersProductsSecretsMapGrouped"],
		"products_secrets_map":           aggregatedSecrets["customersProductsSecretsMap"],
	}

	return result, nil
}
