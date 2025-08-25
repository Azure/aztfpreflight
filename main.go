package main

import (
	"context"
	"flag"
	"fmt"
	"path"
	"sync"

	"github.com/Azure/aztfpreflight/internal/api"
	"github.com/Azure/aztfpreflight/internal/plan"
	"github.com/Azure/aztfpreflight/internal/tfclient"
	"github.com/Azure/aztfpreflight/internal/utils"
	"github.com/hashicorp/terraform-exec/tfexec"
	"github.com/sirupsen/logrus"
)

const helpMessage = `
Usage: aztfpreflight [options]
Options:
	-i <file>   		file path to terraform plan file
	-v          		enable verbose logging
	-h          		show help
	-j          		json output
	-skip-preflight		skip preflight check
	-c <n>      		max concurrent preflight requests (default 8)`

func main() {
	logrus.SetLevel(logrus.InfoLevel)

	planfilepath := flag.String("i", "", "file path to terraform plan file")
	verbose := flag.Bool("v", false, "enable verbose logging")
	help := flag.Bool("h", false, "show help")
	jsonOutput := flag.Bool("j", false, "json output")
	skipPreflight := flag.Bool("skip-preflight", false, "skip preflight check")
	preflightConcurrency := flag.Int("c", 8, "max concurrent preflight requests")
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
	if jsonOutput != nil && *jsonOutput {
		logrus.SetFormatter(&logrus.JSONFormatter{})
	}

	execPath, err := tfclient.FindTerraform(context.TODO())
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
		logrus.Debugf("request model json: %s\n", utils.ToCompactJson(model))
	}
	logrus.Infof("total terraform resources: %d, success: %d, failed: %d\n", len(models), len(models)-len(failedAddrs), len(failedAddrs))

	if *skipPreflight {
		logrus.Infof("skipping preflight check...\n")
		return
	}
	if *preflightConcurrency <= 0 {
        *preflightConcurrency = 1
    }
	logrus.Infof("sending preflight requests with concurrency: %d...\n", *preflightConcurrency)
	preflightErrors := make([]error, 0)
	var mu sync.Mutex
	sem := make(chan struct{}, *preflightConcurrency)
	var wg sync.WaitGroup
	for _, model := range models {
		if model.Failed != nil {
			continue
		}
		m := model // capture loop variable
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if _, err := api.Preflight(context.Background(), m.URL, m.Body); err != nil {
				mu.Lock()
				preflightErrors = append(preflightErrors, fmt.Errorf("address: %s, error: %w", m.Address, err))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if len(preflightErrors) > 0 {
		logrus.Infof("preflight errors: %d\n", len(preflightErrors))
		for _, err := range preflightErrors {
			logrus.Errorf("%s\n", err)
		}
	} else {
		logrus.Infof("preflight check passed\n")
	}
}
