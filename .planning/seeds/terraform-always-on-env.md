---
title: Promote ephemeral Terraform env to a long-lived environment
trigger_condition: Ephemeral apply/test/destroy cycle is stable in Actions and there is appetite to practice operating persistent infra
planted_date: 2026-10-17
---

Practice the operate-it side of IaC: scheduled `terraform plan` drift detection, a state-recovery drill (lock break, import, state mv), provider/version upgrades, and a separate prod-like workspace or state key. Cost note (unverified): an always-on B1ms Flexible Server is the dominant monthly cost. See `.planning/notes/2026-10-17-terraform-azure-ephemeral-deploy.md`.
