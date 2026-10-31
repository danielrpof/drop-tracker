data "azurerm_subscription" "current" {}

# A managed identity instead of an app registration: the student tenant blocks service principals.
resource "azurerm_user_assigned_identity" "github" {
  name                = "id-droptracker-github"
  resource_group_name = azurerm_resource_group.tfstate.name
  location            = azurerm_resource_group.tfstate.location
}

resource "azurerm_federated_identity_credential" "drop-tracker" {
  name      = "drop-tracker-cred"
  parent_id = azurerm_user_assigned_identity.github.id
  audience  = ["api://AzureADTokenExchange"]
  issuer    = "https://token.actions.githubusercontent.com"
  subject   = "${var.github_sub_prefix}:ref:refs/heads/main"
}

# Contributor creates and destroys the root module's resources; it cannot grant roles.
resource "azurerm_role_assignment" "github_contributor" {
  scope                = data.azurerm_subscription.current.id
  role_definition_name = "Contributor"
  principal_id         = azurerm_user_assigned_identity.github.principal_id
}

# Lets the pipeline read and write state through Entra auth instead of storage keys.
resource "azurerm_role_assignment" "github_state" {
  scope                = azurerm_storage_account.tfstate.id
  role_definition_name = "Storage Blob Data Contributor"
  principal_id         = azurerm_user_assigned_identity.github.principal_id
}
