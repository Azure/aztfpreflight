package tfclient_test

import (
	"strings"
	"testing"

	"github.com/Azure/aztfpreflight/internal/tfclient"
)

func Test_NewTerraformClient(t *testing.T) {
	client := tfclient.NewTerraformClient()
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	if len(client.ResourceSchemas) == 0 {
		t.Fatal("Expected non-empty ResourceSchemas")
	}
}

func Test_ApplyResource(t *testing.T) {
	testcases := []struct {
		resourceType string
		body         interface{}
		expectError  string
	}{
		{
			resourceType: "azurerm_automation_account",
			body: map[string]interface{}{
				"name":                "test",
				"location":            "East US",
				"resource_group_name": "test",
				"sku_name":            "Basic",
			},
			expectError: "unexpected status 400 with response:",
		},
		{
			resourceType: "azurerm_resource_group",
			body: map[string]interface{}{
				"name":     "test",
				"location": "East US",
			},
			expectError: `Status=400 Code="InterceptedError"`,
		},
	}

	client := tfclient.NewTerraformClient()
	for _, tc := range testcases {
		err := client.ApplyResource(tc.resourceType, tc.body)
		if err == nil {
			t.Fatalf("Expected error for resource type %s, got nil", tc.resourceType)
		}
		if tc.expectError != "" && !strings.Contains(err.Error(), tc.expectError) {
			t.Fatalf("Expected error to contain %q, got %q", tc.expectError, err.Error())
		}
	}
}
