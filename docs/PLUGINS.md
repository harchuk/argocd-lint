# Plugin Roadmap

`pkg/plugin` now ships with a Rego adapter so custom policy bundles can participate in lint runs.

## Rego plugin format

Create a `.rego` module that exposes three entry points:

- `metadata` (object, required) – describes the rule and default configuration.
- `deny` (set or array of objects, required) – returns findings for the manifest under evaluation.
- `applies` (boolean rule, optional) – when present, determines whether the plugin should run for the current manifest.

Example (`examples/plugins/require-prefix.rego`):

```rego
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
```

### Metadata contract

`metadata` must include:

- `id` (string) – unique rule identifier.
- `description` (string) – short summary.
- `severity` (string) – `info`, `warn`, or `error` used as the default.

Optional keys:

- `applies_to` – array of resource kinds (`Application`, `ApplicationSet`).
- `help_url` – additional documentation link.
- `category` – reporting category string.
- `enabled` – set to `false` to disable by default.

### Finding schema

Each entry emitted by `deny` must be an object. All fields are optional; argocd-lint fills in sensible defaults (file path, resource name/kind, rule id, severity) when they are omitted.

Supported keys include:

- `message` – human-friendly explanation.
- `severity` – override configured severity (`info|warn|error`).
- `rule_id` – override the reported rule ID.
- `file`, `line`, `column` – location metadata.
- `resource_name`, `resource_kind` – override resource metadata.
- `category`, `help_url` – override defaults from metadata.

### CLI usage

Pass individual modules or whole directories using the new flags:

```bash
argocd-lint ./manifests \
  --plugin examples/plugins/require-prefix.rego \
  --plugin-dir ./more-policies
```

The loader recursively discovers `.rego` files in the supplied directories. Plugins participate in configuration overrides just like built-in rules, so you can tweak severities through the standard `rules` and `overrides` sections.

### Curated bundles

Maintained bundles live in `bundles/`. Package them for distribution with:

```bash
./scripts/package-plugin-bundles.sh dist
```

The archives can be attached to GitHub releases or baked into container images
for offline environments. The core bundle includes opinionated naming and
metadata checks that complement the built-in rule set.

#### Core bundle

Policies inside `bundles/core/` enforce naming prefixes and required ownership
labels. Update the bundled Rego modules to match your organisation before
shipping them broadly.

#### Security bundle

Located at `bundles/security/`, these rules focus on HTTPS destinations and Git
repositories served over secure transports. Combine them with the core bundle or
extend them with organisation-specific controls.

### Repo-server integration

To run the same policies inside Argo CD, follow the starter kit in
`examples/repo-server-plugin/` and the detailed walkthrough in
`docs/REPO_SERVER.md`. The Config Management Plugin publishes lint findings as
part of the repo-server sync, blocking promotion when violations are detected.

### Roadmap

1. Add conformance tests that validate third-party plugin bundles.
2. Expand curated bundles with security and policy-as-code rules contributed by the community.
