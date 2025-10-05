package argocd_lint.require_prefix

metadata := {
  "id": "RG100",
  "description": "Application names must include the team prefix",
  "severity": "warn",
  "applies_to": ["Application"],
  "category": "Consistency",
  "help_url": "https://example.com/argocd-lint/plugins#prefix",
}

required_prefix := "team-"

applies {
  input.kind == "Application"
}

deny[f] {
  not startswith(input.name, required_prefix)
  f := {
    "message": sprintf("%s is missing required prefix %s", [input.name, required_prefix]),
    "resource_name": input.name,
    "severity": "error",
  }
}
