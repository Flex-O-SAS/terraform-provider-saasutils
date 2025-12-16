// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
	"maps"
)

// build customers config with inherited from customer "secretsFrom" settings.
func computeCustomerInheritance(customers map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for key, value := range customers {
		customerMap, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("customer '%s' is not a map", key)
		}

		name := getString(customerMap, "name")
		if name == "" {
			return nil, fmt.Errorf("customer '%s' missing 'name' field", key)
		}

		secretsFrom := getString(customerMap, "secretsFrom")
		productConfig := getMap(customerMap, "product_config")

		secretModuleName := name
		if secretsFrom != "" {
			secretModuleName = secretsFrom
		}

		var mergedProductConfig map[string]interface{}
		if secretsFrom != "" {
			parentCustomer, exists := customers[secretsFrom]
			if !exists {
				return nil, fmt.Errorf("customer '%s' references non-existent secretsFrom customer '%s'", key, secretsFrom)
			}
			parentMap, ok := parentCustomer.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("parent customer '%s' is not a map", secretsFrom)
			}
			parentProductConfig := getMap(parentMap, "product_config")
			mergedProductConfig = mergeMaps(parentProductConfig, productConfig)
		} else {
			mergedProductConfig = mergeMaps(make(map[string]interface{}), productConfig)
		}

		resultCustomer := maps.Clone(customerMap)
		resultCustomer["secret_module_name"] = secretModuleName
		resultCustomer["product_config"] = mergedProductConfig

		result[key] = resultCustomer
	}

	return result, nil
}

// build customers config inherited with default product settings.
func computeProductInheritance(
	inheritCustomer map[string]interface{},
	products map[string]interface{},
	features map[string]interface{},
) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for customerKey, customerValue := range inheritCustomer {
		customerMap, ok := customerValue.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("customer '%s' is not a map", customerKey)
		}

		customerProductsList := getList(customerMap, "products")
		customerProductsSet := make(map[string]bool)
		for _, p := range customerProductsList {
			if pStr, ok := p.(string); ok {
				customerProductsSet[pStr] = true
			}
		}

		customerProductConfig := getMap(customerMap, "product_config")

		customerProducts := make(map[string]interface{})
		for productKey, productValue := range products {
			// Only process if this customer uses this product
			if !customerProductsSet[productKey] {
				continue
			}

			productMap, ok := productValue.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("product '%s' is not a map", productKey)
			}

			productFeatures := getMap(productMap, "features")

			customerOverride := getMap(customerProductConfig, productKey)
			customerOverrideFeatures := getMap(customerOverride, "features")

			mergedFeatures := mergeMaps(productFeatures, customerOverrideFeatures)

			completeFeatures := make(map[string]interface{})
			for featureKey := range features {
				if val, exists := mergedFeatures[featureKey]; exists {
					completeFeatures[featureKey] = val
				} else {
					completeFeatures[featureKey] = false
				}
			}

			resultProduct := mergeMaps(productMap, customerOverride)
			resultProduct["name"] = productKey
			resultProduct["features"] = completeFeatures

			customerProducts[productKey] = resultProduct
		}

		result[customerKey] = customerProducts
	}

	return result, nil
}

// build customers config inherited with subfeatures.
func computeSubfeaturesInheritance(
	inheritProduct map[string]interface{},
	features map[string]interface{},
) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	for customerKey, customerValue := range inheritProduct {
		customerProducts, ok := customerValue.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("customer '%s' products is not a map", customerKey)
		}

		resultCustomerProducts := make(map[string]interface{})

		for productKey, productValue := range customerProducts {
			productMap, ok := productValue.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("product '%s' for customer '%s' is not a map", productKey, customerKey)
			}

			existingFeatureConfig := getMap(productMap, "feature_config")

			productFeatures := getMap(productMap, "features")

			newFeatureConfigEntries := make(map[string]interface{})

			for featureName, featureEnabled := range productFeatures {
				enabled, ok := featureEnabled.(bool)
				if !ok || !enabled {
					continue
				}

				featureDefMap, featureExists := features[featureName]
				if !featureExists {
					continue
				}

				featureDef, ok := featureDefMap.(map[string]interface{})
				if !ok {
					continue
				}

				defaultSubfeatures := getMap(featureDef, "subfeatures")

				if len(defaultSubfeatures) == 0 {
					continue
				}
				if !anyTrue(defaultSubfeatures) {
					continue
				}

				existingFeatureEntry := getMap(existingFeatureConfig, featureName)
				customerOverrideSubfeatures := getMap(existingFeatureEntry, "subfeatures")

				mergedSubfeatures := mergeMaps(defaultSubfeatures, customerOverrideSubfeatures)

				newFeatureEntry := mergeMaps(existingFeatureEntry, map[string]interface{}{
					"subfeatures": mergedSubfeatures,
				})

				newFeatureConfigEntries[featureName] = newFeatureEntry
			}

			mergedFeatureConfig := mergeMaps(existingFeatureConfig, newFeatureConfigEntries)

			resultProduct := mergeMaps(productMap, map[string]interface{}{
				"feature_config": mergedFeatureConfig,
			})

			resultCustomerProducts[productKey] = resultProduct
		}

		result[customerKey] = resultCustomerProducts
	}

	return result, nil
}
