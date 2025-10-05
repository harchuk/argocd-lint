FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY . .
RUN go build -ldflags "-s -w" -o /out/argocd-lint ./cmd/argocd-lint

FROM alpine:3.19
RUN addgroup -S argocd && adduser -S argocd -G argocd
COPY --from=builder /out/argocd-lint /usr/local/bin/argocd-lint
USER argocd
ENTRYPOINT ["argocd-lint"]
CMD ["--help"]
