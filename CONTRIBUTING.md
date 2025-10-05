# Contributing to argocd-lint

Thank you for your interest in improving `argocd-lint`! We welcome bug reports, feature suggestions, and pull requests. This document walks through the workflow and expectations for contributors.

## Code of conduct

Please review and follow our [Code of Conduct](CODE_OF_CONDUCT.md). By participating, you agree to uphold these guidelines.

## Getting started

1. Fork the repository and create a feature branch from `main`.
2. Install Go 1.22 or newer.
3. Run `go mod tidy` to populate dependencies.
4. Build and test locally:
   ```bash
   make build
   make test
   ```
5. Run the linter over sample manifests: `make examples`.

## Development workflow

- **Formatting:** run `gofmt` on all modified Go files before submitting.
- **Testing:** add or update unit tests alongside your changes.
- **Commits:** keep commits focused; use descriptive messages.
- **Pull requests:** reference related issues, describe the change, and include validation notes (builds, tests, manual checks).

## Adding rules

When adding or enhancing lint rules:

- Update rule metadata in `internal/rule/rules.go`.
- Extend the rule table in `README.md` and note the change in `CHANGELOG.md`.
- Add targeted tests in `internal/rule` or appropriate packages.

## Release process

Maintainers follow the steps in `docs/RELEASING.md`. If your change impacts release artifacts (new CLI flags, config keys, etc.), mention it in your PR description.

## Reporting issues

Use the GitHub issue templates to file bugs or feature requests. Include reproduction steps, expected vs. actual behaviour, and environment details (OS, Go version, relevant manifest snippets).

We appreciate your contributions—thank you for helping make `argocd-lint` better!
