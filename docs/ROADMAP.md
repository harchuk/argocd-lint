# Roadmap

## Upcoming Enhancements

- **Server-side validation**: integrate kubeconform or `kubectl --dry-run=server` via a new validator module. Reuse kubeconfig handling and allow per-environment toggles.
- **Policy plugins**: extend `pkg/plugin` with Rego/OPA adapters and expose CLI flags to load external policies.
- **Repo-server integration**: document how to run the same rules inside Argo CD repo-server (plugin starter kit).
