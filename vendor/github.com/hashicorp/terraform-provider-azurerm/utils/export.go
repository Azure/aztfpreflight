package utils

import (
	"context"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-provider-azurerm/internal/provider/framework"
)

func ProtoV5Provider() (tfprotov5.ProviderServer, error) {
	providerServer, _, err := framework.ProtoV5ProviderServerFactory(context.Background())
	if err != nil {
		return nil, err
	}
	return providerServer(), nil
}
