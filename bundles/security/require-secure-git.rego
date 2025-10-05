package argocd_lint.security.require_secure_git

metadata := {
  "id": "RGS002",
  "description": "Git repositories must use SSH or HTTPS",
  "severity": "error",
  "applies_to": ["Application", "ApplicationSet"],
  "category": "security",
  "help_url": "https://github.com/argocd-lint/argocd-lint/tree/main/bundles/security",
}

deny[f] {
  repo := insecure_repo

  f := {
    "message": sprintf("repoURL %s must not use http://", [repo]),
    "severity": "error",
  }
}

insecure_repo := repo {
  repo := application_repo
  startswith(repo, "http://")
}

insecure_repo := repo {
  repo := application_set_repo
  startswith(repo, "http://")
}

application_repo := repo {
  spec := input.object.spec
  repo := spec.source.repoURL
  repo != ""
}

application_repo := repo {
  spec := input.object.spec
  repos := spec.sources
  repos != null
  repo := repos[_].repoURL
  repo != ""
}

application_set_repo := repo {
  spec := input.object.spec
  template := spec.template
  repo := template.spec.source.repoURL
  repo != ""
}

application_set_repo := repo {
  spec := input.object.spec
  template := spec.template
  repos := template.spec.sources
  repos != null
  repo := repos[_].repoURL
  repo != ""
}

application_set_repo := repo {
  spec := input.object.spec
  generators := spec.generators
  generators != null
  template := generators[_].template
  repo := template.spec.source.repoURL
  repo != ""
}

application_set_repo := repo {
  spec := input.object.spec
  generators := spec.generators
  generators != null
  template := generators[_].template
  repos := template.spec.sources
  repos != null
  repo := repos[_].repoURL
  repo != ""
}
