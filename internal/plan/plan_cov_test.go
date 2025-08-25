package plan_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/Azure/aztfpreflight/internal/plan"
	"github.com/Azure/aztfpreflight/internal/tfclient"
	"github.com/Azure/aztfpreflight/internal/types"
	"github.com/hashicorp/terraform-exec/tfexec"
)

func Test_AzureRMResourceCoverage(t *testing.T) {
	// AZURERM_EXPORT_DIR is the directory where the exported test cases are stored
	dir := os.Getenv("AZURERM_EXPORT_DIR")
	if dir == "" {
		t.Skip("Skipping test: AZURERM_EXPORT_DIR is not set")
	}
	t.Setenv("ARM_PROVIDER_ENHANCED_VALIDATION", "false")
	t.Setenv("ARM_SKIP_PROVIDER_REGISTRATION", "true")

	testcaseMap, err := ListTestcases(dir)
	if err != nil {
		t.Fatalf("Failed to list test cases: %v", err)
	}

	t.Logf("Total test cases: %d", len(testcaseMap))

	failed := make(map[string]types.FailedCase)
	tested := make(map[string]bool)
	success := make(map[string]bool)

	tfexecPath, err := tfclient.FindTerraform(context.TODO())
	if err != nil {
		t.Fatalf("Failed to find terraform executable: %v", err)
	}
	index := 0
	for azurermType, testcases := range testcaseMap {
		index++
		t.Logf("Processing test case %d: %s", index, azurermType)

		validPlanOutputPath := ""
		for _, testcase := range testcases {
			tf, _ := tfexec.NewTerraform(testcase, tfexecPath)

			planOutputPath := path.Join(testcase, "planfile.tfplan")
			// skip if the plan file already exists
			if _, err = os.Stat(planOutputPath); err != nil {
				_, err = tf.Plan(context.TODO(), tfexec.Out(planOutputPath))
				if err != nil {
					continue
				}
				t.Logf("Plan file created: %s", planOutputPath)
			} else {
				t.Logf("Plan file already exists: %s", planOutputPath)
			}
			validPlanOutputPath = planOutputPath
			break
		}

		tf, _ := tfexec.NewTerraform(path.Dir(validPlanOutputPath), tfexecPath)
		tfplan, err := tf.ShowPlanFile(context.TODO(), validPlanOutputPath)
		if err != nil {
			failed[azurermType] = types.FailedCase{
				Detail: fmt.Sprintf("Failed to show plan for %s: %v", validPlanOutputPath, err),
			}
			continue
		}

		models := plan.ExportAzurePayload(tfplan)
		for _, model := range models {
			resourceType := strings.Split(model.Address, ".")[0]
			if model.Failed != nil {
				failed[resourceType] = *model.Failed
			} else {
				success[resourceType] = true
			}
		}
	}

	for azurermType := range failed {
		tested[azurermType] = true
	}
	for azurermType := range success {
		tested[azurermType] = true
	}

	t.Logf("Total tested resource types: %d", len(tested))
	t.Logf("Total successful resource types: %d, coverage: %.2f%%", len(success), float64(len(success))/float64(len(tested))*100)
	t.Logf("Total failed resource types: %d", len(failed))
	for azurermType, failedCase := range failed {
		t.Logf("Failed resource type: %s, detail: %s", azurermType, failedCase.Detail)
	}

	supportedResourceTypes := make([]string, 0)
	for azurermType := range success {
		supportedResourceTypes = append(supportedResourceTypes, azurermType)
	}
	markdownContent := MarkdownReport(len(tested), len(success), supportedResourceTypes)
	docOutputPath := path.Join("..", "docs", "supported_azurerm_resource_types.md")
	err = os.WriteFile(docOutputPath, []byte(markdownContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write report file: %v", err)
	}
	t.Logf("Markdown report written to: %s", docOutputPath)
}

func MarkdownReport(total int, supported int, resourceTypes []string) string {
	template := `# Supported AzureRM resource types

## Overview

Total number of AzureRM resource types: %d  
Supported AzureRM resource types: %d


## List of supported AzureRM resource types

%s
`
	list := ""
	sort.Strings(resourceTypes)
	for _, resourceType := range resourceTypes {
		list += fmt.Sprintf("- %s\n", resourceType)
	}

	return fmt.Sprintf(template, total, supported, list)
}

func ListTestcases(input string) (map[string][]string, error) {
	resourceTypeDirs, err := os.ReadDir(input)
	if err != nil {
		return nil, err
	}
	output := make(map[string][]string)
	for _, resourceTypeDir := range resourceTypeDirs {
		if !resourceTypeDir.IsDir() {
			continue
		}

		resourceType := resourceTypeDir.Name()

		tests := make([]string, 0)

		testcaseDirs, err := os.ReadDir(path.Join(input, resourceType))
		if err != nil {
			return nil, err
		}

		for _, testcaseDir := range testcaseDirs {
			if !testcaseDir.IsDir() {
				continue
			}
			testcase := testcaseDir.Name()

			stepDirs, err := os.ReadDir(path.Join(input, resourceType, testcase))
			if err != nil {
				continue
			}
			for _, stepDir := range stepDirs {
				if !stepDir.IsDir() {
					continue
				}
				step := stepDir.Name()
				tests = append(tests, path.Join(input, resourceType, testcase, step))
			}
		}

		output[resourceType] = tests
	}
	return output, nil
}
