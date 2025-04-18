package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/Azure/aztfpreflight/account"
	"github.com/Azure/aztfpreflight/utils"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
)

type CheckPolicyRestrictionsRequestModel struct {
	ResourceDetails    ResourceDetailsModel `json:"resourceDetails"`
	IncludeAuditEffect bool                 `json:"includeAuditEffect"`
}

type ResourceDetailsModel struct {
	ResourceContent map[string]interface{} `json:"resourceContent"`
	ApiVersion      string                 `json:"apiVersion"`
}

type CheckPolicyRestrictionsResponseModel struct {
	FieldRestrictions       []FieldRestriction           `json:"fieldRestrictions"`
	ContentEvaluationResult ContentEvaluationResultModel `json:"contentEvaluationResult"`
}

type ContentEvaluationResultModel struct {
	PolicyEvaluations []PolicyEvaluationModel `json:"policyEvaluations"`
}

type PolicyEvaluationModel struct {
	PolicyInfo        map[string]interface{} `json:"policyInfo"`
	EvaluationResult  string                 `json:"evaluationResult"`
	EvaluationDetails map[string]interface{} `json:"evaluationDetails"`
	EffectDetails     map[string]interface{} `json:"effectDetails"`
}

type FieldRestriction struct {
	Field        string                   `json:"field"`
	Restrictions []map[string]interface{} `json:"restrictions"`
}

func CheckPolicyRestrictions(ctx context.Context, requestUrl string, payloadJson string) (interface{}, error) {
	var payloadMap map[string]interface{}
	if err := json.Unmarshal([]byte(payloadJson), &payloadMap); err != nil {
		return nil, err
	}

	parsedUrl, err := url.Parse(requestUrl)
	if err != nil {
		return nil, err
	}

	armId, err := arm.ParseResourceID(parsedUrl.Path)
	if err != nil {
		return nil, err
	}

	payloadMap["type"] = armId.ResourceType.String()
	payloadMap["name"] = armId.Name

	model := CheckPolicyRestrictionsRequestModel{
		ResourceDetails: ResourceDetailsModel{
			ResourceContent: payloadMap,
			ApiVersion:      parsedUrl.Query().Get("api-version"),
		},
		IncludeAuditEffect: false,
	}

	client, err := DefaultSharedClient()
	if err != nil {
		return nil, err
	}

	resourceManagerAccount := account.DefaultSharedAccount()

	scope := fmt.Sprintf("/subscriptions/%s", resourceManagerAccount.GetSubscriptionId())
	if armId.Parent.ResourceType.String() == arm.ResourceGroupResourceType.String() {
		scope = armId.Parent.String()
	}
	CheckPolicyRestrictionsUrl := fmt.Sprintf("%s/providers/Microsoft.PolicyInsights/checkPolicyRestrictions", scope)

	resp, err := Execute[CheckPolicyRestrictionsResponseModel](ctx, client, http.MethodPost, CheckPolicyRestrictionsUrl, "2023-03-01", model)
	if err != nil {
		return nil, err
	}
	if resp != nil {
		evaluationResult := ""
		for _, policyEvaluation := range resp.ContentEvaluationResult.PolicyEvaluations {
			if policyEvaluation.EvaluationResult == "NotApplicable" {
				continue
			}
			evaluationResult = policyEvaluation.EvaluationResult
			break
		}
		if evaluationResult != "" {
			return resp, fmt.Errorf("resource is not compliant with policy: %s", utils.ToJson(resp))
		}
	}
	return resp, nil
}
