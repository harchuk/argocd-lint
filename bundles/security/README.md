# Security bundle

Policies focused on transport security and Git source hygiene. Pair these rules
with the core bundle or extend them with organisation-specific checks.

Included plugins:

- `enforce-https-destination.rego` – block applications that sync to HTTP
  destinations.
- `require-secure-git.rego` – guard against repositories served over plain HTTP.

Bundle archives are built via `./scripts/package-plugin-bundles.sh`.
