# syntax=docker/dockerfile:1

# Builds the rclone fork with the web UI: the image (default target) for
# linux/amd64 or linux/arm64, or only the binary (target "binary") for any
# platform Go cross-compiles to. The context is the repository root, see
# webui/justfile:
#
#   docker buildx build -f webui/docker/Dockerfile --platform linux/arm64 .
#
# Every RUN executes on the build platform, so building for another
# architecture doesn't need QEMU.

ARG GO_VERSION=1.26
ARG ALPINE_VERSION=3.24

FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG TARGETOS TARGETARCH VERSION=dev-webui
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH \
    go build -trimpath -tags cmount -ldflags "-s -X github.com/rclone/rclone/fs.Version=$VERSION" -o /out/ .

FROM scratch AS binary
COPY --from=build /out/ /

# Collects into /out the files the upstream image adds to alpine: the
# packages are installed for the target architecture without running
# their scripts, and only the ones missing from the base image are kept.
FROM --platform=$BUILDPLATFORM alpine:${ALPINE_VERSION} AS rootfs
ARG TARGETARCH RCLONE_UID=1009 RCLONE_GID=1009
RUN case "$TARGETARCH" in \
        amd64) arch=x86_64 ;; \
        arm64) arch=aarch64 ;; \
        *) echo "image not supported for $TARGETARCH" >&2; exit 1 ;; \
    esac && \
    apk add --quiet --no-cache --root /rootfs --arch "$arch" --initdb --no-scripts \
        --keys-dir /usr/share/apk/keys --repositories-file /etc/apk/repositories \
        ca-certificates fuse3 mailcap tzdata && \
    apk info -q | sort > /tmp/base && \
    apk --root /rootfs info -q | sort > /tmp/all && \
    for pkg in $(comm -13 /tmp/base /tmp/all); do apk --root /rootfs info -qL "$pkg"; done | \
        grep -v '^$' > /tmp/files && \
    mkdir /out && \
    tar -C /rootfs -cf - -T /tmp/files | tar -C /out -xpf - && \
    echo "user_allow_other" >> /out/etc/fuse.conf && \
    addgroup -g "$RCLONE_GID" rclone && \
    adduser -u "$RCLONE_UID" -Ds /bin/sh -G rclone rclone && \
    tar -C / -cf - etc/passwd etc/group etc/shadow home/rclone | tar -C /out -xpf -

FROM alpine:${ALPINE_VERSION} AS image
COPY --from=rootfs /out/ /
COPY --from=build /out/rclone /usr/local/bin/
ENTRYPOINT [ "rclone" ]
WORKDIR /data
ENV XDG_CONFIG_HOME=/config
