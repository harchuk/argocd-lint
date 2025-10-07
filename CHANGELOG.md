# Changelog

All notable changes to this project are documented here. Dates reflect the commit timestamps on `main`.

## [0.2.0] - 2025-10-05

### Added
- `argocd-lint applicationset plan` command for previewing generated Applications and reporting create/delete/unchanged drift before syncing (`e087d78`).
- `argocd-lint plugins list` command plus metadata discovery helpers for curated/community bundles (`d796667`, `pkg/plugin/rego`).
- Schema-version pinning via `--argocd-version`, AppProject guardrails, and richer multi-source Application validations (`de36265`).
- Rule profiles (`dev`, `prod`, `security`, `hardening`) with CLI flag and SARIF severity alignment (`profiles` update).

### Changed
- Human-facing CLI/table output now renders bordered tables with summaries and consistent error banners (`4b898ec`).

### Documentation
- README quick-start updated with ApplicationSet planning and plugin listing usage.
- Roadmap and contribution guides refreshed to capture community bundle workflow.

## [0.1.0] - 2025-10-05

### Added
- Rego plugin engine with CLI wiring for custom policies (`45f344d`).
- Curated bundle conformance tests executed during CI to keep published policies healthy (`0e04d81`).

### Fixed
- Rego loader upgraded to the latest OPA APIs for forward compatibility (`29b1f83`).

### Documentation
- Repo-server integration guide and plugin bundle docs (`8eeafbd`).
- README quick-start, visuals, and overview polish (`f5b9cd7`, `bcb74dc`, `2d231c4`).

[0.2.0]: https://github.com/harchuk/argocd-lint/compare/45f344d...e087d78
[0.1.0]: https://github.com/harchuk/argocd-lint/tree/45f344d
