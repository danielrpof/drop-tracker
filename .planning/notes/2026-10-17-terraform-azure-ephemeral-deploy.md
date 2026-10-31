---
title: Terraform + Azure ephemeral deploy cycle
date: 2026-10-17
context: /gsd-explore session; $100 Azure student credit, goal is Terraform/IaC + Azure experience, not a public product
---

## Decisions

- **Purpose:** DevOps practice (Terraform, Azure as second cloud, pipeline-driven deploy). No domain, no permanent public hosting.
- **Lifecycle:** ephemeral. Cycle is `terraform apply` -> check -> smoke test -> `terraform destroy`.
- **Order:** local CLI cycle first, then the same modules driven from GitHub Actions (`workflow_dispatch`, smoke test, destroy under `if: always()`).
- **Shape:** Azure Container Apps (consumption) pulling the public ghcr.io image + Azure Database for PostgreSQL Flexible Server (B1ms).
- **State:** Azure Storage blob backend in a separate bootstrap root module that is applied once and never destroyed.
- **Auth:** GitHub Actions -> Azure via OIDC federated credentials, no stored secrets.
- **Guardrails as code:** `azurerm_consumption_budget` alerts + manual destroy workflow.
- **Data:** empty DB each cycle; app migrates on boot; smoke test seeds itself (POST /watchlist + re-GET, as `n1-boot` does).

## Relationship to existing work

- Supersedes the blocker on deferred Phase 17 (automated VPS deploy, DPLY-01...08): that phase waits on a VPS + domain. This gives a deploy target without either. Phase 17's `17-CONTEXT.md` and the v1.4 `/ready` probe (health gate) are reusable inputs.
- Pipeline currently ends at `release` (ghcr push, SBOM, svu tag) in `.github/workflows/full-pipeline.yml`; a deploy job would gate on it and pass the `vX.Y.Z` tag as a Terraform variable.

## Unresolved (not stood behind)

- Flexible Server B1ms ~ $0.02/hr and "stopped server auto-restarts after 7 days" - unverifiable: prior knowledge, no primary source consulted.
- Total credit runway estimates (~100+ sessions) - derived from the unverified hourly figure above.

## Update 2026-10-30 (slice a done, 67b1bec)

- **Learning mode:** the user wants to learn Terraform. Claude writes boilerplate; the user writes core resources and predicts `plan` output; quiz via the grilling skill. Weak spot to revisit: the three-way compare (config vs state vs real Azure).
- **Region:** the student subscription's policy allows only southcentralus, canadacentral, mexicocentral, eastus2, brazilsouth. Use `eastus2`.
- **Free tier** (expires 2027-02-06): Postgres Flexible B1MS 750 h + 32 GB storage and backup. Free-tier B-series VMs are restricted in the allowed regions, so the app runs on Container Apps.
- **Identity:** a user-assigned managed identity with a federated credential replaced the app registration. The tenant returned 403 on service-principal and credential writes, and the app had been created with no owner. An orphan app `droptracker-github-actions` remains in the unal.edu.co tenant.
- **OIDC subject** uses immutable IDs (`gh api repos/{owner}/{repo}/actions/oidc/customization/sub`). Unverified until the first real login; Azure prints the received subject on mismatch.
- **Providers** `Microsoft.App`, `Microsoft.DBforPostgreSQL`, `Microsoft.OperationalInsights` were NotRegistered; slice (b) must register them (azurerm registers a core set automatically).
- Backend `key` for the root module should be `root/terraform.tfstate`; auth defaults to the `az login` key lookup (Owner role), not Entra data-plane auth.
