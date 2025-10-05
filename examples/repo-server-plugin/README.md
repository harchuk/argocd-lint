# Repo-server plugin starter kit

This directory contains a minimal setup for running `argocd-lint` as an Argo CD
Config Management Plugin (CMP). Use it as a template and adjust the versions and
flags for your environment.

## Contents

- `Dockerfile` – extends the official repo-server image with the `argocd-lint`
  binary and curated Rego bundles shipped with this repository.
- `plugin.yaml` – CMP definition that executes `argocd-lint` against the
  application sources before streaming the manifests back to Argo CD.

## Usage

1. Build and push a repo-server image:

   ```bash
   export ARGOCD_LINT_VERSION=v0.5.0   # choose the tag you want to deploy
   export TARGETARCH=amd64             # match the cluster architecture

   docker build \
     --build-arg ARGOCD_LINT_VERSION=$ARGOCD_LINT_VERSION \
     --build-arg TARGETARCH=$TARGETARCH \
     -f examples/repo-server-plugin/Dockerfile \
     -t ghcr.io/your-org/argocd-repo-server:lint \
     .

   docker push ghcr.io/your-org/argocd-repo-server:lint
   ```

2. Patch the Argo CD repo-server stateful set to use the custom image:

   ```bash
   kubectl patch sts argocd-repo-server \
     -n argocd \
     --type merge \
     -p '{"spec":{"template":{"spec":{"containers":[{"name":"repo-server","image":"ghcr.io/your-org/argocd-repo-server:lint"}]}}}}'
   ```

3. Publish the plugin definition by adding `plugin.yaml` to the `argocd-cmp-cm`
   ConfigMap:

   ```bash
   kubectl patch configmap argocd-cmp-cm \
     -n argocd \
     --type merge \
     --patch "$(cat examples/repo-server-plugin/plugin.yaml)"
   ```

4. Reference the plugin from an application:

   ```yaml
   apiVersion: argoproj.io/v1alpha1
   kind: Application
   metadata:
     name: demo
     namespace: argocd
   spec:
     destination:
       namespace: default
       server: https://kubernetes.default.svc
     project: default
     source:
       repoURL: https://github.com/your-org/app-config
       targetRevision: main
       path: apps
       plugin:
         name: argocd-lint
         parameters:
           - name: severity
             value: warn
   ```

## Runtime behaviour

- The plugin aborts the sync early when `argocd-lint` reports findings at or
  above the configured severity threshold.
- If linting succeeds the plugin streams all `.yaml`/`.yml` files from the
  application directory back to Argo CD, preserving the standard GitOps flow.
- Curated bundles are mounted at `/opt/argocd-lint/bundles`, so you can extend
  them with additional policies or point the CLI to bespoke directories.

See `docs/REPO_SERVER.md` for additional background and tuning advice.
