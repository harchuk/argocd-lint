# Releasing argocd-lint

This guide documents the steps maintainers follow to publish a new release.

1. Update version metadata:
   - Bump `Version` in `pkg/version/version.go`.
   - Add release notes to `CHANGELOG.md` under a new section.
2. Ensure dependencies are tidy:
   ```bash
   go mod tidy
   ```
3. Run the full validation suite locally:
   ```bash
   gofmt -w $(find . -name '*.go' -not -path './vendor/*')
   go test ./...
   make build
   ./bin/argocd-lint examples/apps --render
   ```
4. Commit the changes and open a pull request. Wait for all GitHub Actions to succeed.
5. Create a signed tag matching the new version (e.g., `v0.2.0`):
   ```bash
   git tag -s v0.2.0 -m "Release v0.2.0"
   git push origin v0.2.0
   ```
6. The `Release` GitHub Action builds cross-platform binaries, calculates checksums, and drafts the GitHub release with artifacts.
7. Verify the release notes and attach additional assets if required.

## Verifying binaries

Downloaded binaries can be verified with the published SHA-256 checksums. Example:

```bash
shasum -a 256 argocd-lint-linux-amd64
```

## Post-release housekeeping

- Create an `[Unreleased]` header in `CHANGELOG.md` if missing.
- Reset the default development version in `pkg/version/version.go` to the next snapshot (e.g., `0.2.1-dev`).
