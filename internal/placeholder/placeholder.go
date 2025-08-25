package placeholder

import (
	"fmt"
	"regexp"

	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var r = regexp.MustCompile(`azurerm_(\w+).[\w\[\]"\-]+\.(.+)`)

func ForUnknownReference(references []string, valueType tftypes.Type) interface{} {
	if len(references) == 0 {
		return nil
	}
	out := make([]string, 0)
	for _, reference := range references {
		matches := r.FindStringSubmatch(reference)
		if len(matches) == 0 {
			continue
		}

		if idPlaceholder := ForResourceTypePath(fmt.Sprintf("azurerm_%s", matches[1]), matches[2]); idPlaceholder != "" {
			out = append(out, idPlaceholder)
		}
	}

	if len(out) == 0 {
		return nil
	}
	if valueType != nil {
		if listType, ok := valueType.(tftypes.List); ok {
			if listType.ElementType.Is(tftypes.String) {
				return out
			}
		}
		if tupleType, ok := valueType.(tftypes.Tuple); ok {
			if len(tupleType.ElementTypes) != 0 && tupleType.ElementTypes[0].Is(tftypes.String) {
				return out
			}
		}
		if setType, ok := valueType.(tftypes.Set); ok {
			if setType.ElementType.Is(tftypes.String) {
				return out
			}
		}
	}

	return out[0]
}

func ForPath(path string) interface{} {
	return pathPlaceholderMap[path]
}

func ForResourceTypePath(resourceType string, path string) string {
	if resourceTypeMapping, ok := mapping[resourceType]; ok {
		if placeholder, ok := resourceTypeMapping[path]; ok {
			return placeholder
		}
	}
	return ""
}
