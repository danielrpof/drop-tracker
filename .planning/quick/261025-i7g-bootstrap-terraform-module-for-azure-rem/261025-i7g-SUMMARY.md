---
quick_id: 261025-i7g
status: complete
commit: 84bc8a8
---

# Summary: Terraform bootstrap module (Phase 999.1 slice a)

Built `infra/bootstrap` pair-style: Claude wrote the scaffold and role assignments, the user wrote the storage resources, the federated credential and the backend block.

## Delivered

- Resource group, versioned private storage account, `tfstate` container (all `prevent_destroy`).
- User-assigned managed identity + federated credential for GitHub Actions on `main`, with Contributor (subscription) and Blob Data Contributor (state account) roles.
- Bootstrap state migrated to the `azurerm` backend (`bootstrap/terraform.tfstate`); `plan` reports no changes.

## Deviations and findings

- **Managed identity, not an app registration.** The tenant returned 403 on service-principal and federated-credential writes. The app was created with no owner, so the cause may have been ownership rather than a tenant policy; untested because the managed identity avoids directory permissions entirely.
- **Orphan app registration** `droptracker-github-actions` remains in the unal.edu.co tenant (no owner, undeletable by the user, no credentials). Removed from state with `terraform state rm`.
- **OIDC subject uses immutable IDs** (`repo:danielrpof@127344630/drop-tracker@1323351282:...`) per the repo's OIDC customization; confirm on the first real login (Azure prints the received subject on mismatch).
- Free-tier B-series VMs are unavailable in the allowed regions; plan the app on Container Apps.

## Next

Slice (b): root module (Container App, Postgres Flexible Server B1ms, budget alert) using the `root/terraform.tfstate` key.
