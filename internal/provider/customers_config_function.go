// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

var _ function.Function = &CustomersConfigFunction{}

type CustomersConfigFunction struct{}

func NewCustomersConfigFunction() function.Function {
	return &CustomersConfigFunction{}
}

func (f *CustomersConfigFunction) Metadata(ctx context.Context, req function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "customers_config"
}

func (f *CustomersConfigFunction) Definition(ctx context.Context, req function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Compute customer configurations with product and feature inheritance",
		Description: "Compute customer configurations with product and feature inheritance",
		Parameters: []function.Parameter{
			function.DynamicParameter{
				Name:        "customers",
				Description: "Map of customer configurations",
			},
			function.DynamicParameter{
				Name:        "products",
				Description: "Map of product definitions",
			},
			function.DynamicParameter{
				Name:        "features",
				Description: "Map of feature definitions",
			},
		},
		Return: function.DynamicReturn{},
	}
}

func (f *CustomersConfigFunction) Run(
	ctx context.Context,
	req function.RunRequest,
	resp *function.RunResponse,
) {
	var customers, products, features basetypes.DynamicValue

	resp.Error = function.ConcatFuncErrors(
		req.Arguments.Get(ctx, &customers, &products, &features),
	)
	if resp.Error != nil {
		return
	}

	// Convert Terraform dynamic values to Go maps
	customersData, err := dynamicToMap(customers)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(
			resp.Error,
			function.NewFuncError(fmt.Sprintf("failed to parse customers: %v", err)),
		)
		return
	}

	productsData, err := dynamicToMap(products)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(
			resp.Error,
			function.NewFuncError(fmt.Sprintf("failed to parse products: %v", err)),
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

	result, err := executeCustomerConfig(
		customersData,
		productsData,
		featuresData,
	)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(
			resp.Error,
			function.NewFuncError(fmt.Sprintf("customer/product processing failed: %v", err)),
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

// executeCustomerConfig processes customer configurations.
func executeCustomerConfig(
	customers map[string]interface{},
	products map[string]interface{},
	features map[string]interface{},
) (map[string]interface{}, error) {
	inheritCustomer, err := computeCustomerInheritance(customers)
	if err != nil {
		return nil, fmt.Errorf("customer_inheritance: %w", err)
	}

	inheritProduct, err := computeProductInheritance(inheritCustomer, products, features)
	if err != nil {
		return nil, fmt.Errorf("product_inheritance: %w", err)
	}

	inheritProductsSubfeatures, err := computeSubfeaturesInheritance(inheritProduct, features)
	if err != nil {
		return nil, fmt.Errorf("subfeatures_inheritance: %w", err)
	}

	result := map[string]interface{}{
		"inherit_customer":             inheritCustomer,
		"inherit_product":              inheritProduct,
		"inherit_products_subfeatures": inheritProductsSubfeatures,
	}

	return result, nil
}
