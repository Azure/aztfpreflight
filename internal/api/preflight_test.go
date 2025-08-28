package api

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/Azure/aztfpreflight/internal/types"
)

func Test_Preflight(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Skipping test in non-acceptance test mode")
	}

	subscriptionId := os.Getenv("ARM_SUBSCRIPTION_ID")

	cases := []struct {
		Name          string
		Model         types.RequestModel
		ExpectedError string
	}{
		{
			Name: "No issue",
			Model: types.RequestModel{
				URL: fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/myResourceGroup/providers/Microsoft.AppPlatform/Spring/springName/apps/appsName?api-version=2023-05-01-preview", subscriptionId),
				Body: `
			{
				"location": "eastus",	
				"properties": {	
					"appResourceId": "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/myResourceGroup/providers/Microsoft.AppPlatform/spring/springName/apps/appsName",
					"protocol": "HTTPS",	
					"routes": [],	
					"ssoEnabled": false	
				}	
			}`,
			},
			ExpectedError: "",
		},
		{
			Name: "Blocked by preflight check",
			Model: types.RequestModel{
				URL: fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/myResourceGroup/providers/Microsoft.Storage/storageAccounts/example?api-version=2023-01-01", subscriptionId),
				Body: `
			{
				"location": "eastus",	
				"kind": "StorageV2",
				"sku": {
					"name": "Standard_LRS",
					"tier": "Standard"
				}
			}`,
			},
			ExpectedError: "StorageAccountAlreadyTaken",
		},
		{
			Name: "Blocked by policy",
			Model: types.RequestModel{
				URL: fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/myResourceGroup/providers/Microsoft.Network/networkSecurityGroups/mysg?api-version=2024-01-01", subscriptionId),
				Body: `
			{
				"location": "eastus",	
				"properties": {
					"securityRules": [
						{
							"properties": {	
								"access": "Allow",	
								"direction": "Inbound",	
								"priority": 100,
								"protocol": "*",	
								"sourceAddressPrefix": "*",	
								"sourcePortRange": "*",	
								"destinationAddressPrefix": "*",	
								"destinationPortRange": "*"	
							}	
						}	
					]
				}
			}`,
			},
			ExpectedError: "RequestDisallowedByPolicy",
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			preflightRequestModel, _ := BuildPreflightRequestBody(tc.Model)
			_, err := Preflight(context.TODO(), preflightRequestModel)
			if tc.ExpectedError == "" {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				return
			}
			if err == nil {
				t.Errorf("Expected error: %s, got none", tc.ExpectedError)
				return
			}
			if !strings.Contains(err.Error(), tc.ExpectedError) {
				t.Errorf("Expected error to contain: %s, got: %v", tc.ExpectedError, err)
			}
		})
	}
}

func Test_PreflightInBatch(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Skipping test in non-acceptance test mode")
	}

	subscriptionId := os.Getenv("ARM_SUBSCRIPTION_ID")

	testcases := []struct {
		requests       []types.RequestModel
		expectedErrLen int
	}{
		{
			requests: []types.RequestModel{
				{
					URL: fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/myResourceGroup/providers/Microsoft.AppPlatform/Spring/springName/apps/appsName?api-version=2023-05-01-preview", subscriptionId),
					Body: `
			{
				"location": "eastus",	
				"properties": {	
					"appResourceId": "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/myResourceGroup/providers/Microsoft.AppPlatform/spring/springName/apps/appsName",
					"protocol": "HTTPS",	
					"routes": [],	
					"ssoEnabled": false	
				}	
			}`,
				},
				{
					URL: fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/myResourceGroup/providers/Microsoft.AppPlatform/Spring/springName/apps/appsName?api-version=2023-05-01-preview", subscriptionId),
					Body: `
			{
				"location": "eastus",	
				"properties": {	
					"appResourceId": "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/myResourceGroup/providers/Microsoft.AppPlatform/spring/springName/apps/appsName",
					"protocol": "HTTPS",	
					"routes": [],	
					"ssoEnabled": false	
				}	
			}`,
				},
			},
			expectedErrLen: 0,
		},
	}

	for _, tc := range testcases {
		errs := PreflightInBatch(context.TODO(), tc.requests, 5)
		if len(errs) != tc.expectedErrLen {
			t.Fatalf("expected %d errors, got %d", tc.expectedErrLen, len(errs))
		}
		for _, err := range errs {
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}
	}

}

func Test_preflightRequestKey(t *testing.T) {
	testcases := []struct {
		r        PreflightRequestModel
		expected string
	}{
		{
			r:        PreflightRequestModel{Provider: "Microsoft.Storage", Type: "storageAccounts", Scope: "/subscriptions/000/resourceGroups/rg", Location: "West US"},
			expected: "Microsoft.Storage|storageAccounts|westus|/subscriptions/000/resourceGroups/rg",
		},
	}

	for _, tc := range testcases {
		key := preflightRequestKey(tc.r)
		if key != tc.expected {
			t.Fatalf("expected key %q, got %q", tc.expected, key)
		}
	}
}

func Test_normalizeLocation(t *testing.T) {
	testcases := []struct {
		input string
		want  string
	}{
		{"East US", "eastus"},
		{"eastus", "eastus"},
	}

	for _, tc := range testcases {
		got := normalizeLocation(tc.input)
		if got != tc.want {
			t.Fatalf("normalizeLocation(%q) = %q; want %q", tc.input, got, tc.want)
		}
	}
}

func Test_BuildPreflightRequestBody(t *testing.T) {
	req := types.RequestModel{
		URL:  "https://management.azure.com/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myrg/providers/Microsoft.Storage/storageAccounts/sa1?api-version=2023-01-01",
		Body: `{"location":"East US","properties":{"kind":"StorageV2"}}`,
	}
	body, err := BuildPreflightRequestBody(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body.Provider != "Microsoft.Storage" || body.Type != "storageAccounts" {
		t.Fatalf("unexpected provider/type: %s/%s", body.Provider, body.Type)
	}
	if body.Location != "eastus" {
		t.Fatalf("expected normalized location eastus, got %s", body.Location)
	}
	if body.Scope != "/subscriptions/00000000-0000-0000-0000-000000000000/resourceGroups/myrg" {
		t.Fatalf("unexpected scope: %s", body.Scope)
	}
	if len(body.Resources) != 1 {
		t.Fatalf("expected 1 resource in payload, got %d", len(body.Resources))
	}
	if apiv, ok := body.Resources[0]["apiVersion"].(string); !ok || apiv != "2023-01-01" {
		t.Fatalf("expected apiVersion propagated, got %v", body.Resources[0]["apiVersion"])
	}
}
