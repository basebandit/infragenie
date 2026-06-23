# InfraGenie GitHub Action image: the infragenie binary plus the Layer-1
# scanners it shells out to, pinned for reproducible CI runs.
#
# infragenie is installed with `go install` so this image builds the same way
# whether GitHub builds it from the action directory (`image: Dockerfile`) or
# release CI builds and pushes it to GHCR. Override the version at build time:
#   docker build --build-arg INFRAGENIE_VERSION=v0.4.0 ...
ARG INFRAGENIE_VERSION=latest

FROM golang:1.25-bookworm AS build
ARG INFRAGENIE_VERSION
RUN GOBIN=/out go install github.com/basebandit/infragenie/cmd/infragenie@${INFRAGENIE_VERSION} \
 && GOBIN=/out go install github.com/securego/gosec/v2/cmd/gosec@latest

FROM python:3.12-slim
# Pinned scanners. checkov + semgrep via pip; hadolint as a static binary.
ARG HADOLINT_VERSION=v2.12.0
RUN pip install --no-cache-dir checkov==3.2.334 semgrep==1.86.0 \
 && apt-get update && apt-get install -y --no-install-recommends ca-certificates curl \
 && curl -fsSL -o /usr/local/bin/hadolint \
      https://github.com/hadolint/hadolint/releases/download/${HADOLINT_VERSION}/hadolint-Linux-x86_64 \
 && chmod +x /usr/local/bin/hadolint \
 && apt-get purge -y curl && apt-get autoremove -y && rm -rf /var/lib/apt/lists/*

COPY --from=build /out/infragenie /usr/local/bin/infragenie
COPY --from=build /out/gosec /usr/local/bin/gosec
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
