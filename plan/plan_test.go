package plan_test

import (
	"context"
	"os"
	"path"
	"testing"

	"github.com/Azure/aztfpreflight/plan"
	"github.com/Azure/aztfpreflight/tfclient"
	"github.com/hashicorp/terraform-exec/tfexec"
)

func Test_TopoSortRequests(t *testing.T) {
	testcases := []struct {
		Input  []plan.ApplyRequest
		Output []plan.ApplyRequest
	}{
		{
			Input: []plan.ApplyRequest{
				{
					Address:   "azurerm_storage_account.test",
					DependsOn: []string{"azurerm_resource_group.test"},
				},
				{
					Address:   "azurerm_resource_group.test",
					DependsOn: nil,
				},
			},
			Output: []plan.ApplyRequest{
				{
					Address:   "azurerm_resource_group.test",
					DependsOn: nil,
				},
				{
					Address:   "azurerm_storage_account.test",
					DependsOn: []string{"azurerm_resource_group.test"},
				},
			},
		},

		{
			Input: []plan.ApplyRequest{
				{
					Address:   "azurerm_storage_account.test",
					DependsOn: []string{"azurerm_resource_group.test"},
				},
				{
					Address:   "azurerm_resource_group.test",
					DependsOn: nil,
				},
				{
					Address:   "azurerm_synapse_workspace.test",
					DependsOn: []string{"azurerm_storage_account.test"},
				},
			},
			Output: []plan.ApplyRequest{
				{
					Address:   "azurerm_resource_group.test",
					DependsOn: nil,
				},
				{
					Address:   "azurerm_storage_account.test",
					DependsOn: []string{"azurerm_resource_group.test"},
				},
				{
					Address:   "azurerm_synapse_workspace.test",
					DependsOn: []string{"azurerm_storage_account.test"},
				},
			},
		},
	}

	for _, testcase := range testcases {
		expected := testcase.Output
		actual := plan.TopoSortRequests(testcase.Input)
		if len(actual) != len(expected) {
			t.Fatalf("Expected %d items, got %d", len(testcase.Output), len(actual))
		}

		for i, item := range actual {
			if item.Address != expected[i].Address {
				t.Fatalf("Expected address %s, got %s", expected[i].Address, item.Address)
			}
		}
	}
}

func Test_ExportAzurePayload(t *testing.T) {
	os.Setenv("ARM_SUBSCRIPTION_ID", "00000000-0000-0000-0000-000000000000")
	testcases := []struct {
		PlanFilePath string
		ModelCount   int
	}{
		{
			PlanFilePath: path.Join("testdata", "case1", "planfile"),
			ModelCount:   5,
		},
	}

	tfexecPath, err := tfclient.FindTerraform(context.TODO())
	if err != nil {
		t.Fatalf("Failed to find terraform: %v", err)
	}

	wd, _ := os.Getwd()

	for _, testcase := range testcases {
		planFilePath := path.Join(wd, testcase.PlanFilePath)
		t.Logf("Processing test case: %s", planFilePath)

		tf, err := tfexec.NewTerraform(path.Dir(planFilePath), tfexecPath)
		if err != nil {
			t.Error(err)
			continue
		}

		err = tf.Init(context.TODO())
		if err != nil {
			t.Error(err)
			continue
		}

		tfplan, err := tf.ShowPlanFile(context.TODO(), planFilePath)
		if err != nil {
			t.Fatal(err)
		}

		models := plan.ExportAzurePayload(tfplan)
		if len(models) != testcase.ModelCount {
			t.Fatalf("Expected %d models, got %d", testcase.ModelCount, len(models))
		}
		for _, model := range models {
			t.Logf("Address: %s", model.Address)
			if model.Failed != nil {
				t.Errorf("Failed: %v", *model.Failed)
			} else {
				t.Logf("URL: %s", model.URL)
				t.Logf("Body: %s", model.Body)
			}
		}
	}
}
