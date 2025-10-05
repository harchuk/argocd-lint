# argocd-lint

[![CI](https://github.com/argocd-lint/argocd-lint/actions/workflows/ci.yaml/badge.svg)](https://github.com/argocd-lint/argocd-lint/actions/workflows/ci.yaml)
[![Release](https://github.com/argocd-lint/argocd-lint/actions/workflows/release.yaml/badge.svg)](https://github.com/argocd-lint/argocd-lint/actions/workflows/release.yaml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE)

`argocd-lint` is a fast, offline-first linter for Argo CD `Application` and `ApplicationSet` manifests. It embeds the official CRD schemas, encodes common platform guardrails as rules, and produces CI-friendly output formats so you can block risky changes before they reach the cluster.

## Features

- **Schema aware** – validates manifests against the Argo CD CRDs without needing a cluster.
- **Rule engine** – ships with opinionated best-practice checks and per-rule severity overrides.
- **Flexible output** – table (interactive), JSON (automation), and SARIF (GitHub Code Scanning).
- **Optional rendering** – Helm/Kustomize rendering and findings via `--render`.
- **Integrations** – ready-to-use pre-commit hook, CI workflows, and SARIF upload recipe.
- **Single binary** – no runtime dependencies; ideal for Git hooks and build agents.

## Installation

Choose the method that best fits your workflow:

- **Download a release** (Linux/macOS/Windows, amd64/arm64) from [GitHub Releases](https://github.com/argocd-lint/argocd-lint/releases) and place the binary on your `$PATH`.
- **Build from source**:
  ```bash
  git clone https://github.com/argocd-lint/argocd-lint.git
  cd argocd-lint
  go build -o bin/argocd-lint ./cmd/argocd-lint
  ```
- **Use Go install** (requires Go 1.22+):
  ```bash
  go install github.com/argocd-lint/argocd-lint/cmd/argocd-lint@latest
  ```

Verify the installation:

```bash
argocd-lint --version
```

## Quick start

Lint all Argo CD apps in a folder and fail on warnings:

```bash
argocd-lint ./apps --severity-threshold=warn
```

Only lint `ApplicationSet` manifests and render Helm charts beforehand:

```bash
argocd-lint ./clusters \
  --appsets \
  --render \
  --helm-binary=$(which helm)
```

## Configuration

Fine-tune rules with a YAML file:

```yaml
rules:
  AR001:
    severity: error      # escalate floating targetRevision to error
  AR006:
    enabled: false       # disable finalizer guidance globally

severityThreshold: warn  # default threshold for exit code (overridden by CLI flag)

overrides:
  - pattern: "environments/prod/**"
    rules:
      AR007:
        severity: error  # tighten ignoreDifferences in production
```

Apply the configuration via `--rules`:

```bash
argocd-lint ./manifests --rules rules.yaml --format json
```

### Optional rendering

Use local Helm/Kustomize sources when linting:

```bash
argocd-lint ./apps \
  --render \
  --helm-binary=/opt/homebrew/bin/helm \
  --kustomize-binary=/opt/homebrew/bin/kustomize \
  --repo-root=$(pwd)
```

Rendering failures surface as `RENDER_HELM` or `RENDER_KUSTOMIZE` findings and respect your rule overrides.

### Output formats

- `table` (default) – human-readable summary.
- `json` – machine-friendly format for scripting.
- `sarif` – upload to GitHub Code Scanning.

## Shipped rules

| ID | Kind(s) | Default | Category | Summary |
| --- | --- | --- | --- | --- |
| AR001 | Application, ApplicationSet | warn | Delivery | `targetRevision` must be pinned (no floating refs). |
| AR002 | Application, ApplicationSet | error | Governance | `spec.project` must not be empty or `default`. |
| AR003 | Application | error | Safety | Namespace destinations must declare `destination.namespace`. |
| AR004 | Application | warn | Operations | `syncPolicy` should explicitly choose automated/manual. |
| AR005 | Application | warn | Operations | Automated sync should enable `prune` and `selfHeal`. |
| AR006 | Application | info | Safety | Finalizer usage should be intentional. |
| AR007 | Application | warn | Drift | `ignoreDifferences` must remain tightly scoped. |
| AR008 | ApplicationSet | warn | Delivery | Enable `missingkey=error` for Go templates. |
| AR009 | Application | error | Delivery | Source definitions must be consistent (`path` vs `chart`). |
| AR010 | Application, ApplicationSet | info | Observability | Recommend `app.kubernetes.io/name` label. |
| AR011 | Application | error | Consistency | Application names must be unique within a lint run. |
| SCHEMA_APPLICATION | Application | error | Compliance | Built-in CRD schema validation. |
| SCHEMA_APPLICATIONSET | ApplicationSet | error | Compliance | Built-in CRD schema validation. |
| RENDER_HELM | Application, ApplicationSet | error | Render | `helm template` must succeed (`--render`). |
| RENDER_KUSTOMIZE | Application, ApplicationSet | error | Render | `kustomize build` must succeed (`--render`). |

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

End-to-end example:

```yaml
name: Lint Argo CD manifests

on: [pull_request]

jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go install github.com/argocd-lint/argocd-lint/cmd/argocd-lint@latest
      - run: argocd-lint apps --format sarif > argocd-lint.sarif
      - uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: argocd-lint.sarif
```

The repository also ships reusable workflows in `.github/workflows/` for CI and release automation.

## Development

```bash
make build     # compile binary into ./bin
make test      # go test ./...
make check     # gofmt check + go test
make release   # cross-compile into ./dist
```

Helpful references:

- [CONTRIBUTING.md](CONTRIBUTING.md) – development workflow and expectations.
- [AGENT.md](AGENT.md) – guardrails for automation.
- [CHANGELOG.md](CHANGELOG.md) – release notes history.
- [docs/RELEASING.md](docs/RELEASING.md) – maintainer release guide.
- [docs/PLUGINS.md](docs/PLUGINS.md) – roadmap for custom policy plug-ins.
- Releases are automated: every merge to `main` triggers release-please to prepare a draft; tags (`v*`) build cross-platform binaries and publish to GitHub Releases.
- Container image is published to GHCR (`ghcr.io/<org>/argocd-lint`) alongside each tagged release.

## Roadmap

- Server-side dry-run (kubeconform / `kubectl --dry-run=server`).
- Policy plug-ins (Rego/OPA) and repo-server plug-in starter kit.
- Additional best-practice rules (app-of-apps layout, repo structure).

## Community & support

- Questions or ideas? Open a [feature request](.github/ISSUE_TEMPLATE/feature_request.md).
- Found a bug? File a [bug report](.github/ISSUE_TEMPLATE/bug_report.md) with reproduction steps.
- Contributions are welcome—see [CONTRIBUTING.md](CONTRIBUTING.md) and abide by the [Code of Conduct](CODE_OF_CONDUCT.md).

## License

Apache License 2.0 – see [LICENSE](LICENSE).
