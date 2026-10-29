# syntax=docker/dockerfile:1

# Builds the rclone binary for Android with cgo, using the Android NDK as
# the C toolchain, as the android job of .github/workflows/build.yml does.
# The context is the repository root, see webui/scripts/build-android.sh:
#
#   docker build -f webui/android/Dockerfile --target binary \
#       --output type=local,dest=build .
#
# ARCH selects the target: arm64 (default), arm (armv7a), amd64 or 386.
# The NDK toolchain only runs on linux/amd64, so the build stage is pinned
# to that platform.

ARG GO_VERSION=1.26
ARG NDK_HOST_PLATFORM=linux/amd64

FROM --platform=${NDK_HOST_PLATFORM} golang:${GO_VERSION}-bookworm AS build
ARG NDK_VERSION=r27c
ARG NDK_API=21
RUN apt-get update && \
    apt-get install -y --no-install-recommends unzip && \
    rm -rf /var/lib/apt/lists/*
RUN curl -fsSL -o /tmp/ndk.zip \
        https://dl.google.com/android/repository/android-ndk-${NDK_VERSION}-linux.zip && \
    unzip -q /tmp/ndk.zip -d /opt && \
    rm /tmp/ndk.zip
ENV ANDROID_NDK=/opt/android-ndk-${NDK_VERSION}

WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
ARG ARCH=arm64 VERSION=dev
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    case "$ARCH" in \
        arm64) triple=aarch64-linux-android ;; \
        arm) triple=armv7a-linux-androideabi; export GOARM=7 ;; \
        amd64) triple=x86_64-linux-android ;; \
        386) triple=i686-linux-android ;; \
        *) echo "unsupported ARCH $ARCH" >&2; exit 1 ;; \
    esac && \
    CC=$ANDROID_NDK/toolchains/llvm/prebuilt/linux-x86_64/bin/${triple}${NDK_API}-clang \
    CGO_ENABLED=1 CGO_LDFLAGS="-fuse-ld=lld -s -w" GOOS=android GOARCH=$ARCH \
    go build -trimpath -tags android -ldflags "-s -X github.com/rclone/rclone/fs.Version=$VERSION" -o /out/android .

FROM scratch AS binary
COPY --from=build /out/android /android
