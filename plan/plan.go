package plan

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/Azure/aztfpreflight/placeholder"
	"github.com/Azure/aztfpreflight/tfclient"
	"github.com/Azure/aztfpreflight/types"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	tfjson "github.com/hashicorp/terraform-json"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

type ApplyRequest struct {
	AfterV       interface{}
	Config       *tfjson.Expression
	ResourceType string
	Address      string
	DependsOn    []string
}

func ExportAzurePayload(tfplan *tfjson.Plan) []types.RequestModel {
	out := make([]types.RequestModel, 0)
	client := tfclient.NewTerraformClient()

	requests := make([]ApplyRequest, 0)
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
		config := FindConfigModule(tfplan.Config.RootModule, address)

		requests = append(requests, ApplyRequest{
			AfterV:       change.Change.After,
			Config:       config,
			ResourceType: change.Type,
			Address:      change.Address,
			DependsOn:    listDependsOn(config),
		})
	}

	requests = TopoSortRequests(requests)

	for i, request := range requests {
		valueType := client.ValueType(request.ResourceType)
		plannedValue := PlannedValue(request.AfterV, request.Config, valueType, request.ResourceType)

		err := client.ApplyResource(request.ResourceType, plannedValue)
		errMsg := ""
		if err != nil {
			errMsg = err.Error()
		}

		models := types.NewRequestModelsFromError(errMsg)
		if len(models) == 0 {
			model := types.RequestModel{
				Address: request.Address,
				Failed: &types.FailedCase{
					Detail: errMsg,
				},
			}
			out = append(out, model)
			continue
		} else {
			for index := range models {
				models[index].Address = request.Address
			}
			out = append(out, models...)
		}

		refValue := make(map[string]string)
		model := models[0]
		parsedUrl, err := url.Parse(model.URL)
		if err != nil {
			continue
		}

		resourceId := parsedUrl.Path
		armId, err := arm.ParseResourceID(resourceId)
		if err != nil {
			continue
		}

		armResourceId := armId.String()
		// fix resource ID format for Spring Cloud
		armResourceId = strings.ReplaceAll(armResourceId, "/Microsoft.AppPlatform/Spring", "/Microsoft.AppPlatform/spring")
		refValue[fmt.Sprintf("%s.id", request.Address)] = armResourceId

		for j := i + 1; j < len(requests); j++ {
			requests[j].Config = UpdateConfigWithKnownValues(requests[j].Config, refValue, client.ValueType(requests[j].ResourceType))
		}
	}

	return out
}

func UpdateConfigWithKnownValues(config *tfjson.Expression, refValue map[string]string, valueType tftypes.Type) *tfjson.Expression {
	if config == nil {
		return nil
	}
	if config.ConstantValue != nil && config.ConstantValue != tfjson.UnknownConstantValue {
		return config
	}
	if len(config.References) > 0 {
		for _, ref := range config.References {
			if val, ok := refValue[ref]; ok {
				isValList := false
				if valueType != nil {
					switch valueType.(type) {
					case tftypes.List, tftypes.Tuple, tftypes.Set:
						isValList = true
					default:
						isValList = false
					}
				}
				config.ConstantValue = val
				if isValList {
					config.ConstantValue = []string{val}
				}
				config.References = nil
				break
			}
		}
		return config
	}

	for index, block := range config.NestedBlocks {
		var objectType *tftypes.Object
		if valueType != nil {
			switch v := valueType.(type) {
			case tftypes.Object:
				objectType = &v
			case tftypes.List:
				if objType, ok := v.ElementType.(tftypes.Object); ok {
					objectType = &objType
				}
			case tftypes.Tuple:
				if index < len(v.ElementTypes) {
					if objType, ok := v.ElementTypes[index].(tftypes.Object); ok {
						objectType = &objType
					}
				}
			case tftypes.Set:
				if objType, ok := v.ElementType.(tftypes.Object); ok {
					objectType = &objType
				}
			}
		}

		for key, expr := range block {
			var vType tftypes.Type
			if objectType != nil {
				vType = objectType.AttributeTypes[key]
			}
			config.NestedBlocks[index][key] = UpdateConfigWithKnownValues(expr, refValue, vType)
		}
	}
	return config
}

func listDependsOn(config *tfjson.Expression) []string {
	if config == nil {
		return nil
	}

	if config.ConstantValue != nil && config.ConstantValue != tfjson.UnknownConstantValue {
		return nil
	}

	if len(config.References) > 0 {
		return config.References
	}

	var dependsOn []string
	for _, block := range config.ExpressionData.NestedBlocks {
		for _, expr := range block {
			dependsOn = append(dependsOn, listDependsOn(expr)...)
		}
	}
	return dependsOn
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

func FindConfigModule(input *tfjson.ConfigModule, address string) *tfjson.Expression {
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
			return &tfjson.Expression{
				ExpressionData: &tfjson.ExpressionData{
					NestedBlocks: []map[string]*tfjson.Expression{
						resource.Expressions,
					},
				},
			}
		}
	}

	return nil
}

func TopoSortRequests(requests []ApplyRequest) []ApplyRequest {
	inDegree := make(map[string]int)
	graph := make(map[string][]string)
	for _, request := range requests {
		inDegree[request.Address] = 0
		graph[request.Address] = make([]string, 0)
	}
	for _, request := range requests {
		for _, dep := range request.DependsOn {
			if _, ok := inDegree[dep]; ok {
				inDegree[request.Address]++
				graph[dep] = append(graph[dep], request.Address)
			}
		}
	}
	queue := make([]string, 0)
	for address, count := range inDegree {
		if count == 0 {
			queue = append(queue, address)
		}
	}
	sortedRequests := make([]ApplyRequest, 0)
	for len(queue) > 0 {
		address := queue[0]
		queue = queue[1:]

		for _, request := range graph[address] {
			inDegree[request]--
			if inDegree[request] == 0 {
				queue = append(queue, request)
			}
		}

		for _, request := range requests {
			if request.Address == address {
				sortedRequests = append(sortedRequests, request)
				break
			}
		}
	}
	return sortedRequests
}
