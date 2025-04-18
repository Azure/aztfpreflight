package main

import (
	"context"
	"flag"
	"fmt"
	"path"

	"github.com/Azure/aztfpreflight/api"
	"github.com/Azure/aztfpreflight/plan"
	"github.com/Azure/aztfpreflight/utils"
	install "github.com/hashicorp/hc-install"
	"github.com/hashicorp/hc-install/fs"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/hc-install/src"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/sirupsen/logrus"
)

// FindTerraform finds the path to the terraform executable.
func FindTerraform(ctx context.Context) (string, error) {
	i := install.NewInstaller()
	return i.Ensure(ctx, []src.Source{
		&fs.AnyVersion{
			Product: &product.Terraform,
		},
	})
}

const helpMessage = `
Usage: aztfpreflight [options]
Options:
	-i <file>   file path to terraform plan file
	-v          enable verbose logging
	-h          show help`

func main() {
	logrus.SetLevel(logrus.InfoLevel)

	planfilepath := flag.String("i", "", "file path to terraform plan file")
	verbose := flag.Bool("v", false, "enable verbose logging")
	help := flag.Bool("h", false, "show help")
	flag.Parse()

	if *help {
		fmt.Println(helpMessage)
		fmt.Printf("version: %s\n", VersionString())
		return
	}
	if *planfilepath == "" {
		fmt.Println(helpMessage)
		fmt.Printf("version: %s\n", VersionString())
		logrus.Fatal("plan file path is required")
		return
	}

	if verbose != nil && *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	}

	execPath, err := FindTerraform(context.TODO())
	if err != nil {
		logrus.Fatalf("failed to find terraform executable: %v", err)
	}
	logrus.Infof("terraform executable path: %s\n", execPath)

	tf, err := tfexec.NewTerraform(path.Dir(*planfilepath), execPath)
	if err != nil {
		logrus.Fatalf("failed to create terraform client: %v", err)
	}

	logrus.Infof("reading terraform plan file: %s\n", *planfilepath)
	tfplan, err := tf.ShowPlanFile(context.TODO(), *planfilepath)
	if err != nil {
		logrus.Fatalf("failed to show plan file: %v\n", err)
	}

	logrus.Infof("generating request body...\n")
	models := plan.ExportAzurePayload(tfplan)
	failedAddrs := make([]string, 0)
	for _, model := range models {
		if model.Failed != nil {
			failedAddrs = append(failedAddrs, model.Address)
			logrus.Infof("%s: failed\n", model.Address)
			logrus.Debugf("failed to generate request model for address: %s, error: %s\n", model.Address, model.Failed.Detail)
			continue
		}
		logrus.Infof("%s: success\n", model.Address)
		logrus.Debugf("request model for address: %s, url: %s\nBody: %s\n", model.Address, model.URL, utils.FormatJson(model.Body))
	}
	logrus.Infof("total terraform resources: %d, success: %d, failed: %d\n", len(models), len(models)-len(failedAddrs), len(failedAddrs))

	logrus.Infof("sending preflight request...\n")
	preflightErrors := make([]error, 0)
	for _, model := range models {
		if model.Failed != nil {
			continue
		}

		_, err := api.Preflight(context.Background(), model.URL, model.Body)
		if err != nil {
			preflightErrors = append(preflightErrors, fmt.Errorf("address: %s, error: %w", model.Address, err))
			continue
		}
	}
	if len(preflightErrors) > 0 {
		logrus.Infof("preflight errors: %d\n", len(preflightErrors))
		for _, err := range preflightErrors {
			logrus.Errorf("%s\n", err)
		}
	} else {
		logrus.Infof("preflight check passed\n")
	}
}
