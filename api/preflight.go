package api

import (
	"context"
	"encoding/json"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/sirupsen/logrus"
	"net/http"
	"net/url"
	"strings"
)

type PreflightRequestModel struct {
	Provider  string                   `json:"provider"`
	Type      string                   `json:"type"`
	Location  string                   `json:"location"`
	Scope     string                   `json:"scope"`
	Resources []map[string]interface{} `json:"resources"`
}

type PreflightResponseModel struct {
	Properties PropertiesModel `json:"properties"`
}

type PropertiesModel struct {
	ValidatedResources []string `json:"validatedResources"`
}

func Preflight(ctx context.Context, requestUrl string, payloadJson string) (interface{}, error) {
	parsedUrl, err := url.Parse(requestUrl)
	if err != nil {
		return nil, err
	}

	armId, err := arm.ParseResourceID(parsedUrl.Path)
	if err != nil {
		return nil, err
	}

	if !strings.EqualFold(armId.Parent.ResourceType.String(), arm.ResourceGroupResourceType.String()) {
		logrus.Debug("preflight request is not supported for resource type %s", armId.ResourceType.String())
		return nil, nil
	}

	var payloadMap map[string]interface{}
	if err := json.Unmarshal([]byte(payloadJson), &payloadMap); err != nil {
		return nil, err
	}

	location := ""
	if loc, ok := payloadMap["location"]; ok {
		location = loc.(string)
	}

	payloadMap["apiVersion"] = parsedUrl.Query().Get("api-version")
	payloadMap["name"] = armId.Name
	preflightRequestModel := PreflightRequestModel{
		Provider: armId.ResourceType.Namespace,
		Type:     armId.ResourceType.Type,
		Location: location,
		Scope:    armId.Parent.String(),
		Resources: []map[string]interface{}{
			payloadMap,
		},
	}

	client, err := DefaultSharedClient()
	if err != nil {
		return nil, err
	}
	resp, err := Execute[PreflightResponseModel](ctx, client, http.MethodPost, "/providers/Microsoft.Resources/validateResources", "2020-10-01", preflightRequestModel)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
