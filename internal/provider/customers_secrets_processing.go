// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"fmt"
)

// builds the list of secrets to get from onepassword for each customer/feature.
func computeFeatureSecretsList(
	inheritProductsSubfeatures map[string]interface{},
	features map[string]interface{},
	inheritCustomer map[string]interface{},
) ([]interface{}, error) {
	result := []interface{}{}

	for customerKey, customerValue := range inheritProductsSubfeatures {
		customerProducts, ok := customerValue.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("customer '%s' products is not a map", customerKey)
		}

		origCustomerValue, exists := inheritCustomer[customerKey]
		if !exists {
			return nil, fmt.Errorf("customer '%s' not found in inheritCustomer", customerKey)
		}
		origCustomer, ok := origCustomerValue.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("original customer '%s' is not a map", customerKey)
		}

		customerName := getString(origCustomer, "name")
		secretsFrom := getString(origCustomer, "secretsFrom")

		secretCustomer := customerName
		if secretsFrom != "" {
			secretCustomer = secretsFrom
		}

		for _, productValue := range customerProducts {
			productMap, ok := productValue.(map[string]interface{})
			if !ok {
				continue
			}

			productFeatures := getMap(productMap, "features")
			productFeatureConfig := getMap(productMap, "feature_config")

			for featureName, featureEnabled := range productFeatures {
				enabled, ok := featureEnabled.(bool)
				if !ok || !enabled {
					continue
				}

				featureDefValue, featureExists := features[featureName]
				if !featureExists {
					continue
				}
				featureDef, ok := featureDefValue.(map[string]interface{})
				if !ok {
					continue
				}

				involvedSecrets := getMap(featureDef, "involved_secrets")

				for secretName, secretConfigValue := range involvedSecrets {
					secretConfig, ok := secretConfigValue.(map[string]interface{})
					if !ok {
						continue
					}

					fields := getList(secretConfig, "fields")
					dependsOn := getList(secretConfig, "depends_on")

					allDependenciesSatisfied := true
					for _, depValue := range dependsOn {
						depStr, ok := depValue.(string)
						if !ok {
							allDependenciesSatisfied = false
							break
						}

						var subfeatureEnabled bool

						if featureConfigEntry, hasFeatureConfig := productFeatureConfig[featureName]; hasFeatureConfig {
							featureConfigMap, ok := featureConfigEntry.(map[string]interface{})
							if !ok {
								allDependenciesSatisfied = false
								break
							}
							subfeatures := getMap(featureConfigMap, "subfeatures")
							if val, exists := subfeatures[depStr]; exists {
								subfeatureEnabled, _ = val.(bool)
							} else {
								defaultSubfeatures := getMap(featureDef, "subfeatures")
								subfeatureEnabled = getBool(defaultSubfeatures, depStr)
							}
						} else {
							allDependenciesSatisfied = false
							break
						}

						if !subfeatureEnabled {
							allDependenciesSatisfied = false
							break
						}
					}

					if !allDependenciesSatisfied {
						continue
					}

					secretEntry := map[string]interface{}{
						"customer":        customerName,
						"feature":         featureName,
						"secret":          secretName,
						"fields":          fields,
						"secret_customer": secretCustomer,
					}

					result = append(result, secretEntry)
				}
			}
		}
	}

	result = distinct(result)

	return result, nil
}

// builds the list of secrets to get from onepassword for each customer/component.
func computeComponentSecretsList(
	inheritCustomer map[string]interface{},
	componentSecrets map[string]interface{},
) []interface{} {
	result := []interface{}{}

	for _, customerValue := range inheritCustomer {
		customerMap, ok := customerValue.(map[string]interface{})
		if !ok {
			continue
		}

		customerName := getString(customerMap, "name")
		secretsFrom := getString(customerMap, "secretsFrom")

		secretCustomer := customerName
		if secretsFrom != "" {
			secretCustomer = secretsFrom
		}

		for componentName, componentConfigValue := range componentSecrets {
			componentConfig, ok := componentConfigValue.(map[string]interface{})
			if !ok {
				continue
			}

			for secretName, secretConfigValue := range componentConfig {
				secretConfig, ok := secretConfigValue.(map[string]interface{})
				if !ok {
					continue
				}

				fields := getList(secretConfig, "fields")

				secretEntry := map[string]interface{}{
					"customer":        customerName,
					"component":       componentName,
					"secret":          secretName,
					"fields":          fields,
					"secret_customer": secretCustomer,
				}

				result = append(result, secretEntry)
			}
		}
	}

	result = distinct(result)

	return result
}

// Collects unique secret customers, groups and deduplicates secrets.
func computeSecretsAggregation(
	productSecrets []interface{},
	componentSecrets []interface{},
) map[string]interface{} {

	// Build the objects for the customer secrets v2.
	// Has been added on top of the previous objects to avoid breaking stuff,
	// but it would be cleaner to update the previous objects.
	// NB : We can get the secret_customer of any secret because it's inherited from the customer, but it's not that clean.
	customersSet := make(map[string]bool)

	for _, sec := range componentSecrets {
		if secMap, ok := sec.(map[string]interface{}); ok {
			if secretCustomer := getString(secMap, "secret_customer"); secretCustomer != "" {
				customersSet[secretCustomer] = true
			}
		}
	}

	for _, sec := range productSecrets {
		if secMap, ok := sec.(map[string]interface{}); ok {
			if secretCustomer := getString(secMap, "secret_customer"); secretCustomer != "" {
				customersSet[secretCustomer] = true
			}
		}
	}

	customersWithNonInheritedSecrets := []interface{}{}
	for customer := range customersSet {
		customersWithNonInheritedSecrets = append(customersWithNonInheritedSecrets, customer)
	}

	// Poor man's mapReduce. We do this to handle
	// a) inheritance of secrets from other customers, where both the inheriting and the inherited-from customers are in the same list of customers_components_secrets
	// b) When a feature may be deactivated from the inherited-from customer, but not from the inheriting customer
	// c) where the secretfields may not be the same between the inheriting and the inherited-from customers (idk if it's possible, but just in case)
	customersComponentsSecretsMapGrouped := make(map[string]interface{})

	componentKeysSet := make(map[string]bool)
	for _, sec := range componentSecrets {
		if secMap, ok := sec.(map[string]interface{}); ok {
			component := getString(secMap, "component")
			secret := getString(secMap, "secret")
			key := component + "-" + secret
			componentKeysSet[key] = true
		}
	}

	for customer := range customersSet {
		customerGrouped := make(map[string]interface{})

		for key := range componentKeysSet {
			fieldsForKey := []interface{}{}

			for _, sec := range componentSecrets {
				if secMap, ok := sec.(map[string]interface{}); ok {
					component := getString(secMap, "component")
					secret := getString(secMap, "secret")
					secretCustomer := getString(secMap, "secret_customer")
					currentKey := component + "-" + secret

					if currentKey == key && secretCustomer == customer {
						fields := secMap["fields"]
						fieldsForKey = append(fieldsForKey, fields)
					}
				}
			}

			customerGrouped[key] = fieldsForKey
		}

		customersComponentsSecretsMapGrouped[customer] = customerGrouped
	}

	customersComponentsSecretsMap := make(map[string]interface{})

	for customer, groupedValue := range customersComponentsSecretsMapGrouped {
		grouped, ok := groupedValue.(map[string]interface{})
		if !ok {
			continue
		}

		customerSecrets := []interface{}{}

		for key, fieldsValue := range grouped {
			fieldsList, ok := fieldsValue.([]interface{})
			if !ok {
				continue
			}

			if len(fieldsList) == 0 {
				continue
			}

			flattenedFields := flatten(fieldsList)
			distinctFields := distinct(flattenedFields)

			secretEntry := map[string]interface{}{
				"section_name": key,
				"field_names":  distinctFields,
			}

			customerSecrets = append(customerSecrets, secretEntry)
		}

		customersComponentsSecretsMap[customer] = customerSecrets
	}

	customersProductsSecretsMapGrouped := make(map[string]interface{})

	productKeysSet := make(map[string]bool)
	for _, sec := range productSecrets {
		if secMap, ok := sec.(map[string]interface{}); ok {
			feature := getString(secMap, "feature")
			secret := getString(secMap, "secret")
			key := feature + "-" + secret
			productKeysSet[key] = true
		}
	}

	for customer := range customersSet {
		customerGrouped := make(map[string]interface{})

		for key := range productKeysSet {
			fieldsForKey := []interface{}{}

			for _, sec := range productSecrets {
				if secMap, ok := sec.(map[string]interface{}); ok {
					feature := getString(secMap, "feature")
					secret := getString(secMap, "secret")
					secretCustomer := getString(secMap, "secret_customer")
					currentKey := feature + "-" + secret

					if currentKey == key && secretCustomer == customer {
						fields := secMap["fields"]
						fieldsForKey = append(fieldsForKey, fields)
					}
				}
			}
			customerGrouped[key] = fieldsForKey
		}
		customersProductsSecretsMapGrouped[customer] = customerGrouped
	}

	customersProductsSecretsMap := make(map[string]interface{})

	for customer, groupedValue := range customersProductsSecretsMapGrouped {
		grouped, ok := groupedValue.(map[string]interface{})
		if !ok {
			continue
		}

		customerSecrets := []interface{}{}

		for key, fieldsValue := range grouped {
			fieldsList, ok := fieldsValue.([]interface{})
			if !ok {
				continue
			}
			if len(fieldsList) == 0 {
				continue
			}

			flattenedFields := flatten(fieldsList)
			distinctFields := distinct(flattenedFields)

			secretEntry := map[string]interface{}{
				"section_name": key,
				"field_names":  distinctFields,
			}

			customerSecrets = append(customerSecrets, secretEntry)
		}
		customersProductsSecretsMap[customer] = customerSecrets
	}

	result := map[string]interface{}{
		"customersWithNonInheritedSecrets":     customersWithNonInheritedSecrets,
		"customersComponentsSecretsMapGrouped": customersComponentsSecretsMapGrouped,
		"customersComponentsSecretsMap":        customersComponentsSecretsMap,
		"customersProductsSecretsMapGrouped":   customersProductsSecretsMapGrouped,
		"customersProductsSecretsMap":          customersProductsSecretsMap,
	}

	return result
}
