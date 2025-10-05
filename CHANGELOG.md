# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Added
- Initial project scaffolding, schema validation, and best-practice rules.
- Optional Helm/Kustomize rendering with `RENDER_*` findings.
- Release tooling, contribution docs, and automation guidance.
- Additional unit tests for output formatting and the lint runner.
- Schema smoke tests plus renderer coverage.
- GHCR container image built and published for each release.
- Optional dry-run validation (`--dry-run=kubeconform|server`) with findings.
- Rego plugin adapter with `--plugin` / `--plugin-dir` CLI flags and sample policy.
- AppProject guardrails plus stricter source/label checks for Applications and ApplicationSets.
- `--argocd-version` flag to pin schema validation to bundled Argo CD CRD versions with compatibility tests.
- `argocd-lint plugins list` subcommand for exploring curated bundle metadata.
- Community bundle submission guidelines and review checklist.
- `argocd-lint applicationset plan` preview command to simulate generated Applications and drift.

### Changed
- Strengthened schema validation (Kubernetes-compliant names, richer source checks).
- CI `gofmt` check now fails without mutating sources and validates failing manifest behaviour.
- CLI honours config-driven severity threshold and documents `--version` usage.
- README documents dry-run usage and expanded rule catalog categories.

## [0.1.0] - 2024-04-01
### Added
- MVP CLI supporting Application/ApplicationSet linting with JSON/Table/SARIF outputs.
- Rule configuration overrides and severity thresholds.
- Sample manifests, pre-commit hook, and GitHub Actions workflow.

[Unreleased]: https://github.com/argocd-lint/argocd-lint/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/argocd-lint/argocd-lint/releases/tag/v0.1.0
