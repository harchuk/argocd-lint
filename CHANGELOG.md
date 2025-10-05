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

### Changed
- Strengthened schema validation (Kubernetes-compliant names, richer source checks).
- CI `gofmt` check now fails without mutating sources and validates failing manifest behaviour.
- CLI honours config-driven severity threshold and documents `--version` usage.

## [0.1.0] - 2024-04-01
### Added
- MVP CLI supporting Application/ApplicationSet linting with JSON/Table/SARIF outputs.
- Rule configuration overrides and severity thresholds.
- Sample manifests, pre-commit hook, and GitHub Actions workflow.

[Unreleased]: https://github.com/argocd-lint/argocd-lint/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/argocd-lint/argocd-lint/releases/tag/v0.1.0
