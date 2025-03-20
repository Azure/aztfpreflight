package plan

import (
	"fmt"
	"strings"

	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/ms-henglu/aztfpreflight/placeholder"
	"github.com/ms-henglu/aztfpreflight/tfclient"
	"github.com/ms-henglu/aztfpreflight/types"
)

func ExportAzurePayload(tfplan *tfjson.Plan) []types.RequestModel {
	out := make([]types.RequestModel, 0)
	client := tfclient.NewTerraformClient()
	for _, change := range tfplan.ResourceChanges {
		// Skip resources that are not from the azurerm provider
		if change.ProviderName != "registry.terraform.io/hashicorp/azurerm" {
			continue
		}

		// Skip resources that are not being created or updated
		if !change.Change.Actions.Create() && !change.Change.Actions.Update() {
			continue
		}

		address := fmt.Sprintf("%s.%s", change.Type, change.Name)
		if change.ModuleAddress != "" {
			address = fmt.Sprintf("%s.%s.%s", change.ModuleAddress, change.Type, change.Name)
		}
		configModule := FindConfigModule(tfplan.Config.RootModule, address)

		config := &tfjson.Expression{
			ExpressionData: &tfjson.ExpressionData{
				NestedBlocks: []map[string]*tfjson.Expression{
					configModule.Expressions,
				},
			},
		}
		valueType := client.ValueType(change.Type)

		plannedValue := PlannedValue(change.Change.After, config, valueType, change.Type)

		err := client.ApplyResource(change.Type, plannedValue)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}

		models := types.NewRequestModelsFromError(errMsg)
		if len(models) == 0 {
			model := types.RequestModel{
				Address: change.Address,
				Failed: &types.FailedCase{
					Detail: errMsg,
				},
			}
			out = append(out, model)
			continue
		} else {
			for index := range models {
				models[index].Address = change.Address
			}
			out = append(out, models...)
		}
	}

	return out
}

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
