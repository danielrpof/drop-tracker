terraform {
  backend "azurerm" {
    resource_group_name  = "rg-droptracker-tfstate"
    storage_account_name = "storagedroptrackerazure"
    container_name       = "tfstate"
    key                  = "bootstrap/terraform.tfstate"
  }
}