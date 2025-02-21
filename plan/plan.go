package plan

import (
	"fmt"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/ms-henglu/aztfpreflight/placeholder"
	"strings"
)

func PlannedValue(input interface{}, config *tfjson.Expression, valueType tftypes.Type, path string) interface{} {
	if input == nil {
		if config == nil {
			if pathPlaceholder := placeholder.ForPath(path); pathPlaceholder != nil {
				return pathPlaceholder
			}
		} else {
			if config.ExpressionData.ConstantValue != nil && config.ExpressionData.ConstantValue != tfjson.UnknownConstantValue {
				return config.ExpressionData.ConstantValue
			} else if pathPlaceholder := placeholder.ForPath(path); pathPlaceholder != nil {
				return pathPlaceholder
			} else if refPlaceholder := placeholder.ForUnknownReference(config.References, valueType); refPlaceholder != nil {
				return refPlaceholder
			}
			return fmt.Sprintf("%s-%s", path, "unknown")
		}
	}
	switch v := input.(type) {
	case map[string]interface{}:
		var nestedBlock map[string]*tfjson.Expression
		if config != nil && len(config.NestedBlocks) > 0 {
			nestedBlock = config.NestedBlocks[0]
		}
		var objectType *tftypes.Object
		if valueType != nil {
			if objType, ok := valueType.(tftypes.Object); ok {
				objectType = &objType
			}
		}
		for key, value := range v {
			var vType tftypes.Type
			if objectType != nil {
				vType = objectType.AttributeTypes[key]
			}
			v[key] = PlannedValue(value, nestedBlock[key], vType, fmt.Sprintf("%s.%s", path, key))
		}
		for key, value := range nestedBlock {
			if v[key] == nil {
				var vType tftypes.Type
				if objectType != nil {
					vType = objectType.AttributeTypes[key]
				}
				v[key] = PlannedValue(nil, value, vType, fmt.Sprintf("%s.%s", path, key))
			}
		}
		return v
	case []interface{}:
		if config == nil {
			return v
		}
		if len(config.NestedBlocks) > 0 {
			var elementType tftypes.Type
			if valueType != nil {
				if listType, ok := valueType.(tftypes.List); ok {
					elementType = listType.ElementType
				}
				if tupleType, ok := valueType.(tftypes.Tuple); ok {
					elementType = tupleType.ElementTypes[0]
				}
				if setType, ok := valueType.(tftypes.Set); ok {
					elementType = setType.ElementType
				}
			}

			for index, value := range v {
				nestedBlock := config.NestedBlocks[0]
				if index < len(config.NestedBlocks) {
					nestedBlock = config.NestedBlocks[index]
				}

				v[index] = PlannedValue(value, &tfjson.Expression{
					ExpressionData: &tfjson.ExpressionData{
						NestedBlocks: []map[string]*tfjson.Expression{nestedBlock},
					},
				}, elementType, fmt.Sprintf("%s.%d", path, 0))
			}
		}
		return v
	default:
		return input
	}
}

func FindConfigModule(input *tfjson.ConfigModule, address string) *tfjson.ConfigResource {
	parts := strings.Split(address, ".")
	if parts[0] == "module" {
		for moduleName, moduleCall := range input.ModuleCalls {
			if moduleName == parts[1] {
				return FindConfigModule(moduleCall.Module, strings.Join(parts[2:], "."))
			}
		}

		return nil
	}

	for _, resource := range input.Resources {
		if resource.Address == address {
			return resource
		}
	}

	return nil
}
