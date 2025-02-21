# AZTFPreflight - Run preflight/policy checks for Terraform AzureRM Provider configuration

## Introduction
AZTFPreflight is a tool that helps you run preflight/policy checks for Terraform AzureRM Provider configuration. It is designed to help you identify potential issues with your Terraform configuration before you apply it to your Azure environment.

## Install

#### Install from the binary:

1. Download the binary from [releases](https://github.com/ms-henglu/aztfpreflight/releases).

2. It's recommended to add the directory containing the binary to your `PATH`, so that you can run `aztfpreflight` directly.

#### Install from the source code:

1. Install [Go](https://golang.org/doc/install).

2. Run `go install github.com/ms-henglu/aztfpreflight@latest` to install the latest version.

3. It's recommended to add `$GOPATH/bin` to your `PATH`, so that you can run `aztfpreflight` directly.

## Usage
```
Usage: aztfpreflight [options]
Options:
        -i <file>   file path to terraform plan file
        -v          enable verbose logging
        -h          show help

version: 0.1.0
```

## Step-by-step

1. Install `terraform`: https://www.terraform.io/downloads.html
2. Install `aztfpreflight`, see [Install](#install).
3. Take the below hcl as an example, save it to `main.tf`:
```hcl
resource "azurerm_resource_group" "example" {
  name     = "example-resources"
  location = "West Europe"
}

resource "azurerm_virtual_network" "example" {
  name                = "example-network"
  location            = azurerm_resource_group.example.location
  resource_group_name = azurerm_resource_group.example.name
  address_space       = ["10.0.0.0/160"]
  dns_servers         = ["10.0.0.4", "10.0.0.5"]
}
```
4. Run `terraform init` to initialize the working directory.
5. Run `terraform plan -out=tfplan` to create a plan file.
6. Run `aztfpreflight -i tfplan` to run preflight/policy checks.
```bash
luheng@MacBookPro ~/g/p/f/a/basic> aztfpreflight -i ./planfile
INFO[0001] terraform executable path: /opt/homebrew/bin/terraform 
INFO[0001] reading terraform plan file: ./planfile      
INFO[0001] generating request body...                   
INFO[0002] azurerm_resource_group.example: success      
INFO[0002] azurerm_virtual_network.example: success     
INFO[0002] total terraform resources: 2, success: 2, failed: 0 
INFO[0002] sending preflight request...                 
INFO[0004] preflight errors: 1                          
ERRO[0004] address: azurerm_virtual_network.example, error: POST https://management.azure.com/providers/Microsoft.Resources/validateResources
--------------------------------------------------------------------------------
RESPONSE 400: 400 Bad Request
ERROR CODE: ResourceValidationFailed
--------------------------------------------------------------------------------
{
  "error": {
    "code": "ResourceValidationFailed",
    "message": "Resource validation failed, correlation id: 'e29f2560-139b-4b9c-af1a-e07cc047298d', see details for more information.",
    "details": [
      {
        "code": "InvalidAddressPrefixFormat",
        "target": "/subscriptions/000000/resourceGroups/example-resources/providers/Microsoft.Network/virtualNetworks/example-network",
        "message": "Address prefix 10.0.0.0/160 of resource /subscriptions/000000/resourceGroups/example-resources/providers/Microsoft.Network/virtualNetworks/example-network is not formatted correctly. It should follow CIDR notation, for example 10.0.0.0/24.",
        "details": []
      }
    ]
  }
}
--------------------------------------------------------------------------------
 
INFO[0004] sending policy request...                    
INFO[0006] check policy restrictions passed             
```

## Frequently Asked Questions

1. Which subscription is used for the preflight check?

   The subscription used for the preflight check is the one specified in `ARM_SUBSCRIPTION_ID` environment variable. If this variable is not set, the subscription used for the preflight check is the one specified in `az account show --query id -o tsv`.

## Credit

We wish to thank HashiCorp for the use of some MPLv2-licensed code from their open source project [terraform-provider-azurerm](https://github.com/hashicorp/terraform-provider-azurerm).
