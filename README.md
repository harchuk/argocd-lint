# argocd-lint

`argocd-lint` is a Go-based linter for Argo CD `Application` and `ApplicationSet` manifests. It performs schema validation and best-practice checks locally, making it easy to gate pull requests, enforce platform guidelines, and surface configuration risks before manifests reach your cluster.

## Highlights

- 📦 Single static binary – no runtime dependencies.
- ✅ Built-in OpenAPI schema validation for Applications and ApplicationSets.
- 🔍 Pluggable rule engine with severity/enable toggles (`info`, `warn`, `error`).
- 📄 Outputs in table, JSON, and SARIF (for GitHub Code Scanning).
- 🔁 Integrations for `pre-commit` and GitHub Actions.
- 🧪 Example manifests and starter rules configuration.

## Getting started

```bash
make build               # builds ./bin/argocd-lint
./bin/argocd-lint examples/apps --format table
./bin/argocd-lint ./apps --severity-threshold=warn
```

### Configuration

Rules can be enabled/disabled and reclassified through `rules.yaml`:

```yaml
rules:
  AR001:
    severity: error   # escalate floating targetRevision to errors
  AR006:
    severity: warn    # adjust finalizer guidance

overrides:
  - pattern: "platform/**/*.yaml"
    rules:
      AR010:
        enabled: false
```

Pass the configuration via `--rules`:

```bash
argocd-lint ./manifests --rules rules.yaml --format json
```

### Formats

- `table` (default) – human readable summary.
- `json` – structured output for scripting and automation.
- `sarif` – upload to GitHub Code Scanning.

### Severity thresholds

Use `--severity-threshold` to fail the command when the highest severity meets or exceeds the threshold:

```bash
argocd-lint ./apps --severity-threshold=warn
```

## Rules shipped in v0.1.0

| ID     | Kind(s)           | Default | Summary |
| ------ | ----------------- | ------- | ------- |
| AR001  | Application, ApplicationSet | warn   | `targetRevision` must be pinned (no floating refs).
| AR002  | Application, ApplicationSet | error  | `spec.project` must not be empty or `default`.
| AR003  | Application        | error  | Namespace destinations must declare `destination.namespace`.
| AR004  | Application        | warn   | `syncPolicy` should explicitly choose automated/manual.
| AR005  | Application        | warn   | Automated sync requires `prune` and `selfHeal`.
| AR006  | Application        | info   | Finalizer usage should be intentional.
| AR007  | Application        | warn   | `ignoreDifferences` must remain tightly scoped.
| AR008  | ApplicationSet     | warn   | Enable `missingkey=error` for Go templates.
| AR009  | Application        | error  | Source definitions must be consistent.
| AR010  | Application, ApplicationSet | info | Recommend `app.kubernetes.io/name` label.
| AR011  | Application        | error  | Application names must be unique within a lint run.
| SCHEMA_APPLICATION | Application | error | Built-in CRD schema validation.
| SCHEMA_APPLICATIONSET | ApplicationSet | error | Built-in CRD schema validation.

## Integrations

### Pre-commit

```yaml
repos:
  - repo: local
    hooks:
      - id: argocd-lint
        name: argocd-lint
        entry: argocd-lint --severity-threshold=warn
        language: system
        pass_filenames: false
```

### GitHub Actions

Two workflows are provided:

- `.github/workflows/ci.yaml` – build, test, and lint sample manifests on pushes and pull requests.
- `.github/workflows/release.yaml` – publish multi-OS binaries on tagged releases (`v*`).

### SARIF upload example

```yaml
- name: Run argocd-lint
  run: |
    ./bin/argocd-lint manifests --format sarif > argocd-lint.sarif
- name: Upload SARIF results
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: argocd-lint.sarif
```

## Development

```bash
make build     # compile binary
make test      # go test ./...
make release   # cross-compile into ./dist
```

> **Note:** Run `go mod tidy` after cloning to download dependencies and regenerate `go.sum`.

## Roadmap

- Helm/Kustomize rendering before linting.
- Server-side dry-run (kubeconform / `kubectl --dry-run=server`).
- Policy plug-ins (Rego/OPA) and repo-server plug-in starter kit.
- Additional project best-practice rules (app-of-apps, repo layout validation).

## License

Apache-2.0 (pending).
