FROM quay.io/argoproj/argocd:v2.9.3

USER root

ARG ARGOCD_LINT_VERSION=v0.5.0
ARG TARGETARCH=amd64

RUN set -euxo pipefail \
	&& curl -fsSL "https://github.com/argocd-lint/argocd-lint/releases/download/${ARGOCD_LINT_VERSION}/argocd-lint_${ARGOCD_LINT_VERSION}_linux_${TARGETARCH}.tar.gz" \
	  | tar -xz -C /usr/local/bin argocd-lint \
	&& chmod +x /usr/local/bin/argocd-lint

COPY bundles /opt/argocd-lint/bundles

USER 999
