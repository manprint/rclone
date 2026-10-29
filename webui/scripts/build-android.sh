#!/usr/bin/env bash
# Builds the Android binary in docker into build/android. ANDROID_ARCH selects
# the target: arm64 (default), arm, amd64 or 386. The version is the one
# `make version` reports.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

ARCH=${ANDROID_ARCH:-arm64}
VERSION=${VERSION:-$(make -s version)}

docker build -f webui/android/Dockerfile --target binary \
	--build-arg ARCH="$ARCH" --build-arg VERSION="$VERSION" \
	--output type=local,dest=build .
