#!/usr/bin/env bash
# Builds rclone with the web UI, marking the version with a -webui suffix.
# Usage: build.sh   (writes ./rclone; set OUT to choose another path)
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

version="$(git describe --tags --always --dirty)-webui"
go build -ldflags "-s -X github.com/rclone/rclone/fs.Version=$version" -o "${OUT:-rclone}" .
echo "built ${OUT:-rclone} $version"
