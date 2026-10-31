variable "subscription_id" {
  type        = string
  description = "Azure subscription that owns the state storage."
}

variable "location" {
  type        = string
  description = "Azure region; the student policy only allows a handful."
  default     = "eastus2"
}

variable "github_sub_prefix" {
  type        = string
  description = "Start of GitHub's OIDC subject for this repo; see `gh api repos/{owner}/{repo}/actions/oidc/customization/sub`."
  default     = "repo:danielrpof@127344630/drop-tracker@1323351282"
}
