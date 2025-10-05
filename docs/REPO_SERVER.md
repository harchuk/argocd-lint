# Repo-server Integration Guide

This guide shows how to execute `argocd-lint` inside the Argo CD repo-server so
the same rule set that protects pull requests also validates applications at
deployment time. The approach relies on the
[Config Management Plugin (CMP)](https://argo-cd.readthedocs.io/en/stable/user-guide/config-management-plugins/)
workflow.

## Starter kit

The repository ships an example in `examples/repo-server-plugin/` that
contains:

- a `Dockerfile` extending the official repo-server image with the
  `argocd-lint` binary and the curated Rego bundle shipped with releases;
- `plugin.yaml`, a CMP definition that runs `argocd-lint` against the current
  application sources and then hands control back to the default generators;
- a `README.md` walking through deployment and maintenance.

You can clone the directory and adapt the values (binary version, severity
thresholds, bundle selection) for your environment.

## High level steps

1. **Build a repo-server image** that layers the CLI and plugin bundle on top
   of the stock `quay.io/argoproj/argocd` image.
2. **Register a CMP** by mounting `plugin.yaml` through the standard
   `argocd-cmp-cm` ConfigMap and reloading the repo-server stateful set.
3. **Wire the plugin to an application** with the `spec.source.plugin.name`
   field so Argo CD invokes `argocd-lint` before rendering manifests.

## Plugin behaviour

The provided CMP executes the following steps:

1. Invoke `argocd-lint $ARGOCD_APP_BASE_DIR --plugin-dir /opt/argocd-lint/bundles/core --severity-threshold=error`.
2. If linting fails, the plugin returns a non-zero exit code and the sync is
   halted, surfacing the findings inside the Argo CD UI.
3. On success it streams the original manifests to `stdout` so Argo CD (and any
   other configured generators) continue unaffected.

Because the linter exits early on violations, the repo-server prevents bad
changes from reaching the cluster even if they bypass external CI checks.

## Customising the setup

- Choose a different curated bundle by updating the `--plugin-dir` flag or add
  `--plugin` flags for bespoke policies.
- Override the severity threshold per application with
  `spec.source.plugin.parameters` or global CMP arguments.
- Pin the repo-server image tag to the same version of Argo CD you run in
  production to avoid compatibility drift.

Refer to `examples/repo-server-plugin/README.md` for end-to-end YAML snippets
covering ConfigMap patches, application manifests, and build commands.
