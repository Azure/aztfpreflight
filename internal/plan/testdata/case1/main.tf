

variable "primary_location" {
  default = "West Europe"
}
variable "random_integer" {
  default = 250421160029165309
}
variable "random_string" {
  default = "2mrdi"
}

resource "azurerm_dev_center" "test" {
  name                = "acctestdc-${var.random_string}"
  resource_group_name = azurerm_resource_group.test.name
  location            = azurerm_resource_group.test.location

  identity {
    type         = "UserAssigned"
    identity_ids = [azurerm_user_assigned_identity.test.id]
  }
}


resource "azurerm_resource_group" "test" {
  name     = "acctestrg-${var.random_integer}"
  location = var.primary_location
}

resource "azurerm_user_assigned_identity" "test" {
  name                = "acctestuami-${var.random_string}"
  location            = azurerm_resource_group.test.location
  resource_group_name = azurerm_resource_group.test.name
}


resource "azurerm_shared_image_gallery" "test" {
  name                = "acctestsig${var.random_string}"
  location            = azurerm_resource_group.test.location
  resource_group_name = azurerm_resource_group.test.name
}


provider "azurerm" {
  features {}
}

resource "azurerm_dev_center_gallery" "test" {
  dev_center_id     = azurerm_dev_center.test.id
  shared_gallery_id = azurerm_shared_image_gallery.test.id
  name              = "acctestdcg${var.random_string}"
}
