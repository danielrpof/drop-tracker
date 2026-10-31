# Terraform bootstrap

Remote-state storage and the GitHub OIDC identity. Applied once by hand and never destroyed: every `prevent_destroy` here protects the state that other modules depend on.

| Resource | Purpose |
|---|---|
| Resource group, storage account, `tfstate` container | Remote state (blob versioning on, no public access) |
| User-assigned managed identity + federated credential | Lets GitHub Actions on `main` log in with no stored secret |
| Role assignments | Contributor on the subscription; Blob Data Contributor on the state account |

## Apply

```
az login
cp terraform.tfvars.example terraform.tfvars   # set subscription_id
terraform init
terraform plan -out=tfplan && terraform apply tfplan
```

State lives in blob `bootstrap/terraform.tfstate`; other modules use their own `key`.

## Notes

- Region is `eastus2`: the student subscription's policy only allows five regions.
- A managed identity replaces an app registration because the university tenant denied service-principal and credential writes.
- The OIDC subject uses GitHub's immutable-ID prefix (`gh api repos/{owner}/{repo}/actions/oidc/customization/sub`). If a login fails, Azure prints the subject it received.
- State holds secrets (storage keys). Never commit `*.tfstate*`, `tfplan` or `*.tfvars`.
