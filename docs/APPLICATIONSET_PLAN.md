# ApplicationSet Plan Preview

The `argocd-lint applicationset plan` subcommand expands an ApplicationSet locally and reports which
Applications would be **created**, **deleted**, or **left unchanged** before you ask Argo CD to sync.
It uses the same parsing and templating logic as Argo CD and requires no cluster access.

## Usage

```bash
argocd-lint applicationset plan \
  --file environments/canary-appset.yaml \
  --current manifests/current-apps \
  --format table
```

### Flags

| Flag | Description |
| --- | --- |
| `--file` | Path to the ApplicationSet manifest to preview (required). |
| `--current` | Directory or file containing existing `Application` manifests to compare against. Defaults to the current working tree. |
| `--format` | Output format (`table` or `json`, default `table`). |

## Example output

```
+---------+-----------+----------------------------+---------------------------------------------+
| Action  | Name      | Destination                | Source                                      |
+---------+-----------+----------------------------+---------------------------------------------+
| CREATE  | canary-01 | apps | https://example.com | https://git.example.com/cd/apps | env/canary |
| DELETE  | canary-02 | apps | https://example.com | https://git.example.com/cd/apps | env/canary |
| UNCHANGED | canary-03 | apps | https://example.com | https://git.example.com/cd/apps | env/canary |
+---------+-----------+----------------------------+---------------------------------------------+

Total: 3  create=1  delete=1  unchanged=1
```

## How it works

1. Parses the ApplicationSet manifest with the same parser used for linting.
2. Renders supported generators (currently list-based generators) using Go `text/template`
   with sprig helper functions. All `item` keys are also injected as top-level variables for
   convenience.
3. Builds a synthetic `Application` object from the rendered template block.
4. Discovers existing Applications from `--current` (or the working directory) so that drift
   can be computed without touching an Argo CD API server.

## Tips

- Commit the plan output in pull requests so reviewers understand drift without checking Argo CD.
- Combine with the standard lint run inside GitHub Actions or any CI runner:

  ```yaml
  - name: ApplicationSet plan preview
    run: argocd-lint applicationset plan --file appsets/canary.yaml --current manifests --format table
  ```
- Use `--format json` when you want to post-process the plan (for example, to fail a build if the
  summary contains deletions).

## Related documentation

- [README.md](../README.md#applicationset-drift-preview) for a quick overview.
- [docs/REPO_SERVER.md](REPO_SERVER.md) to enforce guardrails during Argo CD syncs.
