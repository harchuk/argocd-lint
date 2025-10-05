# Releasing argocd-lint

Releases are generated automatically. Every push to the `main` branch runs the `Release` workflow, which:

1. Builds cross-platform binaries with the current commit SHA embedded in the version string.
2. Publishes a GitHub Release whose tag is derived from the timestamp and short commit hash.
3. Uploads the build artifacts and a checksum file to the release.
4. Pushes a multi-architecture container image to GHCR (`ghcr.io/<owner>/<repo>/argocd-lint`).

No manual tagging is required. As long as changes land on `main`, a fresh release and container image appear a few minutes later.

## Development checklist before merging to main

To keep published artifacts healthy, run through the usual checks before merging:

```bash
make check      # gofmt + go test
make build      # local build sanity
./bin/argocd-lint examples/apps --render
```

Large feature releases can still include curated notes by editing `CHANGELOG.md` before merging; the automatic release body will reference the commit SHA, so the changelog remains the canonical place for human-readable notes.

## Verifying published assets

After the workflow completes:

- Inspect the new entry under GitHub Releases to confirm artifacts and `checksums.txt` are attached.
- Pull the container image:
  ```bash
  docker pull ghcr.io/<owner>/<repo>/argocd-lint:<derived-version>
  ```
- Validate binary integrity when desired:
  ```bash
  shasum -a 256 argocd-lint-linux-amd64
  ```

## Manual reruns

If a job in the release workflow fails, re-run it from the Actions tab. The workflow reuses the same version identifier for that commit, so the release and container tags remain consistent.
