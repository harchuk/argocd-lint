FROM quay.io/argoproj/argocd:v3.5.2

USER root

ARG ARGOCD_LINT_VERSION=latest
ARG TARGETARCH=amd64

RUN set -euxo pipefail \
	&& if [ "$ARGOCD_LINT_VERSION" = "latest" ]; then \
	     url="https://github.com/harchuk/argocd-lint/releases/latest/download/argocd-lint-linux-${TARGETARCH}"; \
	   else \
	     url="https://github.com/harchuk/argocd-lint/releases/download/${ARGOCD_LINT_VERSION}/argocd-lint-linux-${TARGETARCH}"; \
	   fi \
	&& curl -fsSL "$url" -o /usr/local/bin/argocd-lint \
	&& chmod +x /usr/local/bin/argocd-lint

COPY bundles /opt/argocd-lint/bundles

USER 999
