# Curated plugin bundles

The `bundles/` directory groups maintained Rego policies that ship alongside the
`argocd-lint` binary. Release automation can package each subdirectory as a
tarball and upload it as an additional asset.

## Structure

- `core/` – default bundle recommended for most teams.
- `security/` – transport security and Git source hardening policies.

Additional bundle folders can be added to target specific platforms or
environments.

## Packaging

Use `scripts/package-plugin-bundles.sh` to build archives for every bundle:

```bash
./scripts/package-plugin-bundles.sh dist
```

The script produces `dist/<bundle>.tar.gz` archives that preserve the directory
layout. The release workflow can publish the tarballs alongside the CLI binary
so users can mount them directly into the repo-server or CI pipelines.
