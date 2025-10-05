package argocd_lint.core.require_owner_label

metadata := {
  "id": "RGC002",
  "description": "Applications must declare the app.kubernetes.io/managed-by label",
  "severity": "warn",
  "applies_to": ["Application", "ApplicationSet"],
  "category": "observability",
  "help_url": "https://github.com/argocd-lint/argocd-lint/blob/main/docs/PLUGINS.md#core-bundle",
}

applies {
  input.kind == "Application"
}

applies {
  input.kind == "ApplicationSet"
}

deny[f] {
  not label_present

  f := {
    "message": "app.kubernetes.io/managed-by label is required",
    "resource_name": input.name,
    "severity": "error",
  }
}

label_present {
  labels := input.object.metadata.labels
  labels["app.kubernetes.io/managed-by"] != ""
}
