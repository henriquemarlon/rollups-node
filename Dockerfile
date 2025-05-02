# (c) Cartesi and individual authors (see AUTHORS)
# SPDX-License-Identifier: Apache-2.0 (see LICENSE)

# syntax=docker.io/docker/dockerfile:1

ARG EMULATOR_VERSION=0.19.0

# Build directories.
ARG GO_BUILD_PATH=/build/cartesi/go

FROM debian:bookworm-20250113 AS common-env

USER root

# Re-declare ARGs so they can be used in the RUN block
ARG EMULATOR_VERSION
ARG GO_BUILD_PATH

# Install ca-certificates and curl (setup).
RUN <<EOF
    set -e
    apt-get update
    apt-get install -y --no-install-recommends \
        ca-certificates curl wget build-essential pkg-config libssl-dev
    addgroup --system --gid 102 cartesi
    adduser --system --uid 102 --ingroup cartesi --disabled-login --no-create-home --home /nonexistent --gecos "cartesi user" --shell /bin/false cartesi
    ARCH=$(dpkg --print-architecture)
    wget -O /tmp/cartesi-machine-emulator.deb "https://github.com/cartesi/machine-emulator/releases/download/v${EMULATOR_VERSION}-alpha4/machine-emulator_${ARCH}.deb"
    case "$ARCH" in
        amd64) echo "c18c078bc42e5fdbb1366912d0dd99173de96522580589478d96dbd0f8aa48bf  /tmp/cartesi-machine-emulator.deb" | sha256sum --check ;;
        arm64) echo "bc19d297d48c4b86843486f8b0a998d46f05373a11b8fa96ad8f009fb76edbd2  /tmp/cartesi-machine-emulator.deb" | sha256sum --check ;;
        *) echo "unsupported architecture: $ARCH"; exit 1 ;;
    esac
    apt-get install -y --no-install-recommends /tmp/cartesi-machine-emulator.deb
    mkdir -p /opt/go ${GO_BUILD_PATH}/rollups-node
    chown -R cartesi:cartesi /opt/go ${GO_BUILD_PATH}
EOF

USER cartesi

# =============================================================================
# STAGE: go-installer
#
# This stage installs Go in the /opt directory.
# =============================================================================

FROM common-env AS go-installer
# Download and verify Go based on the target architecture
RUN <<EOF
    set -e
    ARCH=$(dpkg --print-architecture)
    wget -O /tmp/go.tar.gz "https://go.dev/dl/go1.24.1.linux-${ARCH}.tar.gz"
    case "$ARCH" in
        amd64) echo "cb2396bae64183cdccf81a9a6df0aea3bce9511fc21469fb89a0c00470088073  /tmp/go.tar.gz" | sha256sum --check ;;
        arm64) echo "8df5750ffc0281017fb6070fba450f5d22b600a02081dceef47966ffaf36a3af  /tmp/go.tar.gz" | sha256sum --check ;;
        *) echo "unsupported architecture: $ARCH"; exit 1 ;;
    esac
    tar -C /opt -xzf /tmp/go.tar.gz
    rm /tmp/go.tar.gz
EOF

# Set up Go environment variables
ENV PATH="/opt/go/bin:$PATH"

# =============================================================================
# STAGE: go-prepare
#
# This stage prepares the Go build environment. It downloads the external
# =============================================================================

FROM go-installer AS go-prepare

ARG GO_BUILD_PATH
WORKDIR ${GO_BUILD_PATH}

ENV GOCACHE=${GO_BUILD_PATH}/.cache
ENV GOENV=${GO_BUILD_PATH}/.config/go/env
ENV GOPATH=${GO_BUILD_PATH}/.go

# Download external dependencies.
COPY --chown=cartesi:cartesi go.mod ${GO_BUILD_PATH}/rollups-node/
COPY --chown=cartesi:cartesi go.sum ${GO_BUILD_PATH}/rollups-node/
RUN cd ${GO_BUILD_PATH}/rollups-node && go mod download

# =============================================================================
# STAGE: go-builder
#
# This stage builds the node Go binaries. First it downloads the external
# dependencies and then it builds the binaries.
# =============================================================================

FROM go-prepare AS go-builder

ARG GO_BUILD_PATH

# Build application.
COPY --chown=cartesi:cartesi . ${GO_BUILD_PATH}/rollups-node/

WORKDIR ${GO_BUILD_PATH}/rollups-node

RUN make build-go

# =============================================================================
# STAGE: debian-packager
#
# This stage packages the node binaries into a Debian package.
# =============================================================================

FROM go-builder AS debian-packager

RUN make build-debian-package DESTDIR=$PWD/_install

# =============================================================================
# STAGE: rollups-node
#
# This stage prepares the final Docker image that will be used in the production
# environment. It installs in /usr/bin all the binaries necessary to run the
# node.
#
# (This stage copies the binaries from previous stages.)
# =============================================================================

FROM debian:bookworm-20250113 AS rollups-node

ARG NODE_RUNTIME_DIR=/var/lib/cartesi-rollups-node
ARG GO_BUILD_PATH

USER root

COPY --from=common-env \
    /tmp/cartesi-machine-emulator.deb \
    cartesi-machine-emulator.deb
COPY --from=debian-packager \
    ${GO_BUILD_PATH}/rollups-node/cartesi-rollups-node-v*.deb \
    cartesi-rollups-node.deb

# Download system dependencies required at runtime.
ARG DEBIAN_FRONTEND=noninteractive
RUN <<EOF
    set -e
    addgroup --system --gid 102 cartesi
    adduser --system --uid 102 --ingroup cartesi --disabled-login --no-create-home --home /nonexistent --gecos "cartesi user" --shell /bin/false cartesi
    apt-get update
    apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        procps \
        ./cartesi-machine-emulator.deb \
        ./cartesi-rollups-node.deb
    rm -rf /var/lib/apt/lists/* cartesi-*.deb
    mkdir -p ${NODE_RUNTIME_DIR}/snapshots ${NODE_RUNTIME_DIR}/data
    chown -R cartesi:cartesi ${NODE_RUNTIME_DIR}
EOF

# Set user to low-privilege.
USER cartesi

WORKDIR ${NODE_RUNTIME_DIR}

HEALTHCHECK --interval=1s --timeout=1s --retries=5 \
    CMD curl -G -f -H 'Content-Type: application/json' http://127.0.0.1:10000/healthz

# Set the Go supervisor as the command.
CMD [ "cartesi-rollups-node" ]
