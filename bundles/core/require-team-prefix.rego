package argocd_lint.core.require_team_prefix

metadata := {
  "id": "RGC001",
  "description": "Application names must start with an approved team prefix",
  "severity": "warn",
  "applies_to": ["Application"],
  "category": "governance",
  "help_url": "https://github.com/argocd-lint/argocd-lint/blob/main/docs/PLUGINS.md#core-bundle",
}

# adjust the prefixes to your organisation's naming conventions
approved_prefixes := [
  "team-",
  "platform-",
]

applies {
  input.kind == "Application"
}

deny[f] {
  not has_prefix(input.name)

  f := {
    "message": sprintf("%s must start with one of %v", [input.name, approved_prefixes]),
    "resource_name": input.name,
  }
}

has_prefix(name) {
  prefix := approved_prefixes[_]
  startswith(name, prefix)
}
