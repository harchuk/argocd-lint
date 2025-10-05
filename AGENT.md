# Automation Agent Guide

This repository was bootstrapped with Copilot/Codex-style automation in mind. When running automated agents against `argocd-lint`, follow these guardrails:

1. Always run `go mod tidy` before building to ensure the module graph and `go.sum` are up to date.
2. Run the full validation suite:
   ```bash
   gofmt -w $(find . -name '*.go' -not -path './vendor/*')
   go test ./...
   make build
   ./bin/argocd-lint examples/apps --format table
   ```
3. Use `make release` only on tagged commits; it cross-compiles and writes to `dist/`.
4. Respect `CHANGELOG.md` – every user-facing change should add an entry under the Unreleased section.
5. Keep `pkg/version/version.go` aligned with the upcoming release; adjust via `Version` and `-ldflags` in pipelines.
6. Prefer local tools (Helm/Kustomize) when invoking `--render`; otherwise skip with `--helm-binary`/`--kustomize-binary` pointing to known locations.
7. Do not remove or modify files under `.github/` without maintainer approval – они поддерживают issue/PR шаблоны.
8. Перед коммитом очищайте локальные кеши Go (`rm -rf .gocache .gomodcache`) и не добавляйте их в git: `.gitignore` уже содержит эти пути.

Thanks for helping keep the repository healthy!
