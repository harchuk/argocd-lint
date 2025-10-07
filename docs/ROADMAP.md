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
- Shipped `argocd-lint applicationset plan` preview for ApplicationSet drift estimation.
- Added render caching, parallel workers, rule profiles, and metrics summaries for faster telemetry-driven linting.

## Upcoming Enhancements

- **Security & governance rules**: more out-of-the-box checks covering repoURL protocol/domain whitelists and Argo CD Project destination enforcement.
- **Environment profiles**: ship rule presets for dev/prod/ephemeral environments, building on the existing overrides demo.
- **CMP hardening guide**: publish a hardened Config Management Plugin image example with seccomp, read-only filesystem, and capability drops.
- **Compatibility matrix**: document and test supported Argo CD/CRD/Helm/Kustomize combinations with versioned fixtures.
- **Performance targets**: render caching, parallel execution, tunable limits—goal: average repo lint ≤ 60s.
- **Exception management**: waivers with TTL/reason fields, baseline mode, and aging reports for long-lived deviations.
- **Rule profiles & SARIF mapping**: ship dev/prod/security/hardening bundles and align severities for meaningful PR feedback.
- **Release security**: signed artifacts, SBOMs, hardened Docker/CMP deployment examples.
- **Rego plugin cookbook**: templates, testing guidance, local dev workflow, and “top 10” example policies for Argo CD.
