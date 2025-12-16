// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// merge of two maps, with the second map taking precedence.
func mergeMaps(base, override map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	for k, v := range base {
		result[k] = v
	}

	// Override with second map
	for k, v := range override {
		if vMap, ok := v.(map[string]interface{}); ok {
			if baseMap, ok := result[k].(map[string]interface{}); ok {
				result[k] = mergeMaps(baseMap, vMap)
				continue
			}
		}
		result[k] = v
	}

	return result
}

// extract a string from a map.
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// extract a map from a map.
func getMap(m map[string]interface{}, key string) map[string]interface{} {
	if v, ok := m[key]; ok {
		if mapVal, ok := v.(map[string]interface{}); ok {
			return mapVal
		}
	}
	return make(map[string]interface{})
}

// extract a list from a map.
func getList(m map[string]interface{}, key string) []interface{} {
	if v, ok := m[key]; ok && v != nil {
		// Check if it's already a []interface{}
		if listVal, ok := v.([]interface{}); ok {
			return listVal
		}
		// It might be a different slice type, try to convert
		// This can happen with reflection
		return []interface{}{v}
	}
	return []interface{}{}
}

// extract a bool from a map.
func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// remove duplicate values from a slice.
func distinct(items []interface{}) []interface{} {
	seen := make(map[string]bool)
	result := []interface{}{}

	for _, item := range items {
		// For simple types, use the value directly
		if str, ok := item.(string); ok {
			if !seen[str] {
				seen[str] = true
				result = append(result, item)
			}
		} else if mapItem, ok := item.(map[string]interface{}); ok {
			// For maps, create a string key based on sorted fields
			// This is a simplified approach that works for our use case
			key := generateKey(mapItem)
			if !seen[key] {
				seen[key] = true
				result = append(result, item)
			}
		} else {
			// For other types, always add
			result = append(result, item)
		}
	}

	return result
}

// create a unique string key from a map for deduplication.
func generateKey(m map[string]interface{}) string {
	bytes, err := json.Marshal(m)
	if err != nil {
		panic(fmt.Sprintf("generateKey: failed to marshal map: %v", err))
	}
	return string(bytes)
}

// flatten a slice of slices into a single slice.
func flatten(items []interface{}) []interface{} {
	result := []interface{}{}

	for _, item := range items {
		if slice, ok := item.([]interface{}); ok {
			result = append(result, flatten(slice)...)
		} else {
			result = append(result, item)
		}
	}

	return result
}

// return true if any value in a map is true.
func anyTrue(m map[string]interface{}) bool {
	for _, v := range m {
		if b, ok := v.(bool); ok && b {
			return true
		}
	}
	return false
}

// convert a Terraform dynamic value to a Go map.
func dynamicToMap(dv basetypes.DynamicValue) (map[string]interface{}, error) {
	underlyingValue := dv.UnderlyingValue()

	if underlyingValue.IsNull() {
		return make(map[string]interface{}), nil
	}

	if underlyingValue.IsUnknown() {
		return nil, fmt.Errorf("cannot convert unknown value to map")
	}

	result := make(map[string]interface{})

	if objVal, ok := underlyingValue.(basetypes.ObjectValuable); ok {
		obj, diags := objVal.ToObjectValue(context.Background())
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert to object value: %v", diags.Errors())
		}
		attrs := obj.Attributes()
		for k, v := range attrs {
			converted, err := convertAttrValue(v)
			if err != nil {
				return nil, fmt.Errorf("failed to convert object attribute '%s': %w", k, err)
			}
			result[k] = converted
		}
		return result, nil
	}

	if mapVal, ok := underlyingValue.(basetypes.MapValuable); ok {
		m, diags := mapVal.ToMapValue(context.Background())
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert to map value: %v", diags.Errors())
		}
		elems := m.Elements()
		for k, v := range elems {
			converted, err := convertAttrValue(v)
			if err != nil {
				return nil, fmt.Errorf("failed to convert map element '%s': %w", k, err)
			}
			result[k] = converted
		}
		return result, nil
	}

	return nil, fmt.Errorf("unsupported dynamic value type")
}

// recursively convert attr.Value to interface{}.
func convertAttrValue(v attr.Value) (interface{}, error) {
	if v.IsNull() {
		return nil, nil
	}
	if v.IsUnknown() {
		return nil, nil
	}

	switch val := v.(type) {
	case basetypes.StringValuable:
		sv, diags := val.ToStringValue(context.Background())
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert to string: %v", diags.Errors())
		}
		return sv.ValueString(), nil
	case basetypes.BoolValuable:
		bv, diags := val.ToBoolValue(context.Background())
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert to bool: %v", diags.Errors())
		}
		return bv.ValueBool(), nil
	case basetypes.NumberValuable:
		nv, diags := val.ToNumberValue(context.Background())
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert to number: %v", diags.Errors())
		}
		f, _ := nv.ValueBigFloat().Float64()
		return f, nil
	case basetypes.ListValuable:
		lv, diags := val.ToListValue(context.Background())
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert to list: %v", diags.Errors())
		}
		result := []interface{}{}
		for _, elem := range lv.Elements() {
			converted, err := convertAttrValue(elem)
			if err != nil {
				return nil, fmt.Errorf("failed to convert list element: %w", err)
			}
			result = append(result, converted)
		}
		return result, nil
	case basetypes.TupleValue:
		result := []interface{}{}
		for _, elem := range val.Elements() {
			converted, err := convertAttrValue(elem)
			if err != nil {
				return nil, fmt.Errorf("failed to convert tuple element: %w", err)
			}
			result = append(result, converted)
		}
		return result, nil
	case basetypes.MapValuable:
		mv, diags := val.ToMapValue(context.Background())
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert to map: %v", diags.Errors())
		}
		result := make(map[string]interface{})
		for k, elem := range mv.Elements() {
			converted, err := convertAttrValue(elem)
			if err != nil {
				return nil, fmt.Errorf("failed to convert map element '%s': %w", k, err)
			}
			result[k] = converted
		}
		return result, nil
	case basetypes.ObjectValuable:
		ov, diags := val.ToObjectValue(context.Background())
		if diags.HasError() {
			return nil, fmt.Errorf("failed to convert to object: %v", diags.Errors())
		}
		result := make(map[string]interface{})
		for k, attr := range ov.Attributes() {
			converted, err := convertAttrValue(attr)
			if err != nil {
				return nil, fmt.Errorf("failed to convert object attribute '%s': %w", k, err)
			}
			result[k] = converted
		}
		return result, nil
	default:
		return nil, nil
	}
}

// convert a Go map to a Terraform dynamic value.
func mapToDynamic(m map[string]interface{}) (basetypes.DynamicValue, error) {
	attrMap := make(map[string]attr.Value)
	attrTypes := make(map[string]attr.Type)

	for k, v := range m {
		attrVal, attrType := interfaceToAttrValue(v)
		attrMap[k] = attrVal
		attrTypes[k] = attrType
	}

	objVal, diags := basetypes.NewObjectValue(attrTypes, attrMap)
	if diags.HasError() {
		return basetypes.DynamicValue{}, fmt.Errorf("failed to create object value: %v", diags.Errors())
	}

	return basetypes.NewDynamicValue(objVal), nil
}

// convert interface{} to attr.Value.
func interfaceToAttrValue(v interface{}) (attr.Value, attr.Type) {
	if v == nil {
		return basetypes.NewStringNull(), basetypes.StringType{}
	}

	switch val := v.(type) {
	case string:
		return basetypes.NewStringValue(val), basetypes.StringType{}
	case bool:
		return basetypes.NewBoolValue(val), basetypes.BoolType{}
	case float64:
		bf := big.NewFloat(val)
		return basetypes.NewNumberValue(bf), basetypes.NumberType{}
	case int:
		return basetypes.NewInt64Value(int64(val)), basetypes.Int64Type{}
	case []interface{}:
		if len(val) == 0 {
			listVal, diags := basetypes.NewListValue(basetypes.StringType{}, []attr.Value{})
			if diags.HasError() {
				panic(fmt.Sprintf("Failed to create empty list: %v", diags.Errors()))
			}
			return listVal, basetypes.ListType{ElemType: basetypes.StringType{}}
		}

		elemVals := []attr.Value{}
		var elemType attr.Type
		for i, elem := range val {
			elemVal, eType := interfaceToAttrValue(elem)
			elemVals = append(elemVals, elemVal)

			if i == 0 {
				elemType = eType
			} else {
				// Verify homogeneity - all elements must have the same type
				if fmt.Sprintf("%T", elemType) != fmt.Sprintf("%T", eType) {
					panic(fmt.Sprintf("Element 0 has type %T, element %d has type %T. Terraform lists must be homogeneous.", elemType, i, eType))
				}
			}
		}

		listVal, diags := basetypes.NewListValue(elemType, elemVals)
		if diags.HasError() {
			panic(fmt.Sprintf("Failed to create list value with element type %T: %v", elemType, diags.Errors()))
		}
		return listVal, basetypes.ListType{ElemType: elemType}
	case map[string]interface{}:
		attrMap := make(map[string]attr.Value)
		attrTypes := make(map[string]attr.Type)
		for k, v := range val {
			attrVal, attrType := interfaceToAttrValue(v)
			attrMap[k] = attrVal
			attrTypes[k] = attrType
		}
		objVal, diags := basetypes.NewObjectValue(attrTypes, attrMap)
		if diags.HasError() {
			panic(fmt.Sprintf("Failed to create object value: %v", diags.Errors()))
		}
		return objVal, basetypes.ObjectType{AttrTypes: attrTypes}
	default:
		return basetypes.NewStringValue(fmt.Sprintf("%v", val)), basetypes.StringType{}
	}
}
