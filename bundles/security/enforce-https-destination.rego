package argocd_lint.security.enforce_https_destination

metadata := {
  "id": "RGS001",
  "description": "Destination servers must use HTTPS",
  "severity": "error",
  "applies_to": ["Application", "ApplicationSet"],
  "category": "security",
  "help_url": "https://github.com/argocd-lint/argocd-lint/tree/main/bundles/security",
}

deny[f] {
  server := insecure_server

  f := {
    "message": sprintf("destination.server %s must use https://", [server]),
    "severity": "error",
  }
}

insecure_server := server {
  input.kind == "Application"
  spec := input.object.spec
  server := spec.destination.server
  server != ""
  startswith(server, "http://")
}

insecure_server := server {
  input.kind == "ApplicationSet"
  server := application_set_server
  server != ""
  startswith(server, "http://")
}

application_set_server := server {
  spec := input.object.spec
  template := spec.template
  template.spec.destination.server != ""
  server := template.spec.destination.server
}

application_set_server := server {
  spec := input.object.spec
  g := spec.generators[_]
  template := g.template
  template.spec.destination.server != ""
  server := template.spec.destination.server
}
