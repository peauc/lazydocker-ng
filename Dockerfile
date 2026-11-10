ARG BASE_IMAGE_BUILDER=golang
ARG ALPINE_VERSION=3.24
ARG GO_VERSION=1.27
ARG DOCKER_VERSION=29.9.3

FROM ${BASE_IMAGE_BUILDER}:${GO_VERSION}-alpine${ALPINE_VERSION} AS builder
ARG TARGETOS
ARG TARGETARCH
ARG VERSION
ARG VCS_REF
WORKDIR /tmp/gobuild
COPY ./ .
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64} \
    go build -a -mod=vendor -o /tmp/gobuild/lazydocker-ng \
    -ldflags="-s -w \
    -X main.commit=${VCS_REF} \
    -X main.version=${VERSION} \
    -X main.buildSource=Docker"

FROM docker:${DOCKER_VERSION}-cli AS docker-cli

FROM scratch
ARG BUILD_DATE
ARG VCS_REF
ARG VERSION
LABEL \
    org.opencontainers.image.authors="Clément PÉAU (https://github.com/peauc)" \
    org.opencontainers.image.created=$BUILD_DATE \
    org.opencontainers.image.version=$VERSION \
    org.opencontainers.image.revision=$VCS_REF \
    org.opencontainers.image.url="https://github.com/peauc/lazydocker-ng" \
    org.opencontainers.image.documentation="https://github.com/peauc/lazydocker-ng" \
    org.opencontainers.image.source="https://github.com/peauc/lazydocker-ng" \
    org.opencontainers.image.title="lazydocker-ng" \
    org.opencontainers.image.description="The lazier way to manage everything docker" \
    org.opencontainers.image.licenses="MIT"
ENTRYPOINT [ "/bin/lazydocker-ng" ]
COPY --from=docker-cli /usr/local/bin/docker /bin/docker
COPY --from=builder /tmp/gobuild/lazydocker-ng /bin/lazydocker-ng
