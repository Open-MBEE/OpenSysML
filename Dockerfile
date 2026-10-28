# syntax=docker/dockerfile:1
#
# The openmbee/opensysml image: sysml-grpc as the entrypoint, with sysml and
# sysml-lsp beside it in /usr/local/bin. `make docker` builds it locally and
# the release's publish-docker job pushes it for linux/amd64 and linux/arm64.
#
# The build stage runs the Go toolchain go.mod pins (`toolchain go1.27.3`),
# so the image's binaries are built by the same compiler as the release
# archives; tests/hygiene/container_image_test.go holds the two in lockstep. The
# stage always runs on the build host's own platform and cross-compiles for
# the target, which Go does without an emulator.
FROM --platform=$BUILDPLATFORM golang:1.27.3-alpine AS build

# Release metadata the Makefile stamps into the binaries with -ldflags; the
# defaults name a local development build. GO_VERSION is left to the
# Makefile, which reads it from the toolchain this stage actually runs.
ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
ARG TARGETOS
ARG TARGETARCH

RUN apk add --no-cache make

WORKDIR /src
# A newer go.mod must fail here, not download a toolchain the pin above
# does not name.
ENV GOTOOLCHAIN=local GOFLAGS=-mod=readonly
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOOS="$TARGETOS" GOARCH="$TARGETARCH" \
    make build-sysml build-lsp build-grpc \
        VERSION="$VERSION" COMMIT="$COMMIT" BUILD_TIME="$BUILD_TIME"

# The binaries are static (CGO_ENABLED=0), so the runtime stage needs no libc,
# shell or package manager: distroless static carries CA certificates, tzdata
# and a non-root user and nothing else, which is the smallest surface that
# still lets sysml-grpc fetch an https library archive and log in local time.
FROM gcr.io/distroless/static-debian12:nonroot

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown
LABEL org.opencontainers.image.title="OpenSysML" \
      org.opencontainers.image.description="SysML v2 / KerML runtime: sysml-grpc service with the sysml and sysml-lsp binaries" \
      org.opencontainers.image.source="https://github.com/Open-MBEE/OpenSysML" \
      org.opencontainers.image.documentation="https://open-mbee.github.io/OpenSysML/guide/01-install/" \
      org.opencontainers.image.licenses="Apache-2.0" \
      org.opencontainers.image.version="$VERSION" \
      org.opencontainers.image.revision="$COMMIT" \
      org.opencontainers.image.created="$BUILD_TIME"

COPY --from=build /src/bin/sysml /src/bin/sysml-lsp /src/bin/sysml-grpc /usr/local/bin/

# The standard library's symbol cache; a read-only root filesystem mounts a
# writable volume here, or the service runs uncached and says so once.
ENV XDG_CACHE_HOME=/home/nonroot/.cache

# 50051 serves gRPC, Connect and gRPC-Web; 8081 answers GET /health.
EXPOSE 50051 8081

# `docker run … openmbee/opensysml -port 9000` replaces the CMD, so the
# flags below are defaults a deployer overrides by naming new ones.
ENTRYPOINT ["/usr/local/bin/sysml-grpc"]
CMD ["-port", "50051", "-health-port", "8081"]
