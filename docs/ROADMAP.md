# Roadmap

## Completed

- Added kubeconform / `kubectl --dry-run=server` validation with CLI flags.
- Rego plugin adapter with CLI wiring to load external policies.
- Documented repo-server integration and published a starter kit.
- Established curated plugin bundles and packaging workflow.
- Added bundle conformance tests to CI to guard curated policies.
- Expanded the policy catalogue with a security-focused bundle.
- Surfaced remediation suggestions in JSON/SARIF outputs for actionable lint fixes.
- Refined CLI/table presentation for friendlier human-readable reports.
- Delivered AppProject guardrails, multi-source conflict detection, and ownership label guidance.
- Added `--argocd-version` schema pinning with compatibility tests across bundled CRDs.
- Documented community bundle submission guidelines.
- Added `argocd-lint plugins list` metadata explorer for curated policy bundles.

## Upcoming Enhancements

- **ApplicationSet plan preview**: introduce an `applicationset plan` preview mode with drift impact estimation.
