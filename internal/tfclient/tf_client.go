package tfclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/Azure/aztfpreflight/internal/account"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/hashicorp/terraform-provider-azurerm/helpers"
	"github.com/sirupsen/logrus"
)

type TerraformClient struct {
	v5Client        tfprotov5.ProviderServer
	ResourceSchemas map[string]*tfprotov5.Schema
}

func NewTerraformClient() *TerraformClient {
	os.Setenv("ARM_PROVIDER_ENHANCED_VALIDATION", "false")
	os.Setenv("ARM_SKIP_PROVIDER_REGISTRATION", "true")
	v5Client, err := helpers.ProtoV5Provider()
	if err != nil {
		logrus.Fatal(err)
	}

	ctx := context.TODO()
	// Disable logging for the provider
	log.SetOutput(io.Discard)
	providerSchemaResponse, err := v5Client.GetProviderSchema(ctx, nil)
	log.SetOutput(os.Stdout)
	if err != nil {
		logrus.Fatal(err)
	}

	resourceManagerAccount := account.DefaultSharedAccount()
	subscriptionId := "00000000-0000-0000-0000-000000000000"
	if v := resourceManagerAccount.GetSubscriptionId(); v != "" {
		subscriptionId = v
	}

	providerCfg := fmt.Sprintf(`
{
  "features": [{}],
  "use_cli" : false,
  "subscription_id" : "%s",
  "tenant_id"       : "00000000-0000-0000-0000-000000000000",
  "client_id"       : "00000000-0000-0000-0000-000000000000",
  "client_secret"   : "00000000-0000-0000-0000-000000000000"
}
`, subscriptionId)

	providerConfigType := providerSchemaResponse.Provider.Block.ValueType()
	providerConfigVal, err := tftypes.ValueFromJSONWithOpts([]byte(providerCfg), providerConfigType, tftypes.ValueFromJSONOpts{})
	if err != nil {
		logrus.Fatal(err)
	}
	providerConfig, err := tfprotov5.NewDynamicValue(providerConfigType, providerConfigVal)
	if err != nil {
		logrus.Fatal(err)
	}

	// disable logging for the provider
	log.SetOutput(io.Discard)
	_, err = v5Client.ConfigureProvider(ctx, &tfprotov5.ConfigureProviderRequest{
		Config: &providerConfig,
	})
	log.SetOutput(os.Stdout)
	if err != nil {
		logrus.Fatal(err)
	}
	return &TerraformClient{
		v5Client:        v5Client,
		ResourceSchemas: providerSchemaResponse.ResourceSchemas,
	}
}

func (client *TerraformClient) ApplyResource(resourceType string, input interface{}) error {
	convertedJson, _ := json.Marshal(input)

	schemaType := client.ValueType(resourceType)
	if schemaType == nil {
		return fmt.Errorf("resource type %s not found", resourceType)
	}

	plannedState, err := tftypes.ValueFromJSONWithOpts(convertedJson, schemaType, tftypes.ValueFromJSONOpts{})
	if err != nil {
		logrus.Debugf("failed to convert json to value: %v", err)
		return err
	}
	plannedStateVal, err := tfprotov5.NewDynamicValue(schemaType, plannedState)
	if err != nil {
		logrus.Debugf("failed to convert value to dynamic value: %v", err)
		return err
	}
	priorState, err := tfprotov5.NewDynamicValue(schemaType, tftypes.NewValue(schemaType, nil))
	if err != nil {
		logrus.Debugf("failed to create prior state: %v", err)
		return err
	}

	ctx, cancel := context.WithTimeout(context.TODO(), time.Second*5)
	defer cancel()

	// disable logging for the provider
	log.SetOutput(io.Discard)

	defer func() {
		if r := recover(); r != nil {
			logrus.Debugf("recovered from panic: %v", r)
		}
	}()
	change, err := client.v5Client.ApplyResourceChange(ctx, &tfprotov5.ApplyResourceChangeRequest{
		TypeName:       resourceType,
		PriorState:     &priorState,
		PlannedState:   &plannedStateVal,
		Config:         &plannedStateVal,
		PlannedPrivate: nil,
		ProviderMeta:   nil,
	})
	log.SetOutput(os.Stdout)
	if err != nil {
		logrus.Debugf("failed to apply resource change: %v", err)
		return err
	}
	errMsg := ""
	if change != nil && change.Diagnostics != nil {
		for _, diag := range change.Diagnostics {
			errMsg += fmt.Sprintf("%s\n", diag.Summary)
			if diag.Detail != diag.Summary {
				errMsg += fmt.Sprintf("%s\n", diag.Detail)
			}
		}
	}

	return fmt.Errorf("error applying resource change: %s", errMsg)
}

func (client *TerraformClient) ValueType(resourceType string) tftypes.Type {
	if _, ok := client.ResourceSchemas[resourceType]; !ok {
		return nil
	}
	return client.ResourceSchemas[resourceType].Block.ValueType()
}
