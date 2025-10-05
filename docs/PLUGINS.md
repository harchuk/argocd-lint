# Plugin Roadmap

`pkg/plugin` defines a minimal interface for future policy extensions. A plugin:

- exposes metadata (`types.RuleMetadata`)
- provides an applicability matcher
- returns findings from `Check`

Next steps:

1. Add registry wiring in `internal/lint/runner` so plugins participate alongside built-in rules.
2. Provide a Rego adapter that evaluates OPA policies packaged with the binary.
3. Surface CLI flags (`--plugin-dir`, `--plugin`), and document how repo-server plugins can reuse the same rules.
