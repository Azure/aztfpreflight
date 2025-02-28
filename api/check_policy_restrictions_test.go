package api_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/ms-henglu/aztfpreflight/api"
)

func Test_CheckPolicyRestrictions(t *testing.T) {
	if os.Getenv("TF_ACC") == "" {
		t.Skip("Skipping test in non-acceptance test mode")
	}

	subscriptionId := os.Getenv("ARM_SUBSCRIPTION_ID")

	cases := []struct {
		Name          string
		RequestUrl    string
		PayloadJson   string
		ExpectedError string
	}{
		{
			Name:       "No Policy Restrictions",
			RequestUrl: fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/myResourceGroup/providers/Microsoft.AppPlatform/Spring/springName/apps/appsName?api-version=2023-05-01-preview", subscriptionId),
			PayloadJson: `
			{
				"location": "eastus",	
				"properties": {	
					"appResourceId": "/subscriptions/12345678-1234-1234-1234-123456789012/resourceGroups/myResourceGroup/providers/Microsoft.AppPlatform/spring/springName/apps/appsName",
					"protocol": "HTTPS",	
					"routes": [],	
					"ssoEnabled": false	
				}	
			}`,
			ExpectedError: "",
		},
		{
			Name:       "Deny NSG Internet Inbound Policy Restriction",
			RequestUrl: fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/myResourceGroup/providers/Microsoft.Network/networkSecurityGroups/mysg?api-version=2024-01-01", subscriptionId),
			PayloadJson: `
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
			ExpectedError: `"policyEffect": "Deny"`,
		},
	}

	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			_, err := api.CheckPolicyRestrictions(context.TODO(), tc.RequestUrl, tc.PayloadJson)
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
