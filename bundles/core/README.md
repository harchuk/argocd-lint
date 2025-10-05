# Core bundle

Baseline policies that complement the built-in rule set. These focus on
consistent naming and metadata hygiene.

Included plugins:

- `require-team-prefix.rego` – enforce that Application names include the
  expected team prefix.
- `require-owner-label.rego` – ensure Applications declare the
  `app.kubernetes.io/managed-by` label so ownership is explicit.

Bundle usage examples are available in `docs/PLUGINS.md`.
