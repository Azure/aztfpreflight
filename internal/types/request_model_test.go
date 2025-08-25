package types_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/Azure/aztfpreflight/internal/types"
)

func Test_NewRequestModelsFromError(t *testing.T) {
	testcases := []struct {
		input  string
		expect []types.RequestModel
	}{
		{
			input: `		creating Automation Account (Subscription: "0b1f6471-1bf0-4dda-aec3-cb9272f09590"
				Resource Group Name: "test"
				Automation Account Name: "test"): unexpected status 400 with response: {"code":"InterceptedError","message":"Intercepted error","target":null,"details":null,"innererror":{"body":"{\"location\":\"eastus\",\"properties\":{\"disableLocalAuth\":false,\"encryption\":{\"keySource\":\"Microsoft.Automation\"},\"publicNetworkAccess\":true,\"sku\":{\"name\":\"Basic\"}},\"tags\":{}}","url":"https://management.azure.com/subscriptions/0b1f6471-1bf0-4dda-aec3-cb9272f09590/resourceGroups/test/providers/Microsoft.Automation/automationAccounts/test?api-version=2023-11-01"},"additionalInfo":null}`,
			expect: []types.RequestModel{
				{
					URL:  "https://management.azure.com/subscriptions/0b1f6471-1bf0-4dda-aec3-cb9272f09590/resourceGroups/test/providers/Microsoft.Automation/automationAccounts/test?api-version=2023-11-01",
					Body: "{\"location\":\"eastus\",\"properties\":{\"disableLocalAuth\":false,\"encryption\":{\"keySource\":\"Microsoft.Automation\"},\"publicNetworkAccess\":true,\"sku\":{\"name\":\"Basic\"}},\"tags\":{}}",
				},
			},
		},
		{
			input: `				creating Resource Group "test": resources.GroupsClient#CreateOrUpdate: Failure responding to request: StatusCode=400 -- Original Error: autorest/azure: Service returned an error. Status=400 Code="InterceptedError" Message="InterceptedError" InnerError={"body":"{\"location\":\"eastus\",\"tags\":{}}","url":"https://management.azure.com/subscriptions/0b1f6471-1bf0-4dda-aec3-cb9272f09590/resourcegroups/test?api-version=2020-06-01"}`,
			expect: []types.RequestModel{
				{
					URL:  "https://management.azure.com/subscriptions/0b1f6471-1bf0-4dda-aec3-cb9272f09590/resourcegroups/test?api-version=2020-06-01",
					Body: "{\"location\":\"eastus\",\"tags\":{}}",
				},
			},
		},
	}

	for _, tc := range testcases {
		result := types.NewRequestModelsFromError(tc.input)
		if len(result) != len(tc.expect) {
			t.Fatalf("Expected %d result(s), got %d", len(tc.expect), len(result))
		}
		for i, r := range result {
			if r.URL != tc.expect[i].URL {
				t.Fatalf("Expected URL %s, got %s", tc.expect[i].URL, r.URL)
			}

			var actualBody, expectBody interface{}
			err := json.Unmarshal([]byte(r.Body), &actualBody)
			if err != nil {
				t.Fatalf("Failed to unmarshal actual body: %v", err)
			}
			err = json.Unmarshal([]byte(tc.expect[i].Body), &expectBody)
			if err != nil {
				t.Fatalf("Failed to unmarshal expect body: %v", err)
			}
			if !reflect.DeepEqual(actualBody, expectBody) {
				t.Fatalf("Expected body %s, got %s", tc.expect[i].Body, r.Body)
			}
		}
	}
}
