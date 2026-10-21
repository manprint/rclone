#!/usr/bin/env bash
# Vendors the superdufs web UI assets into webui/assets and records their
# provenance in webui/assets/SOURCE. Usage: sync-assets.sh <superdufs-dir>
set -euo pipefail

if [ $# -ne 1 ]; then
	echo "usage: $0 <superdufs-dir>" >&2
	exit 2
fi

src=$(cd "$1" && pwd)
root=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
dst="$root/webui/assets"
files=(index.html index.js index.css favicon.ico)

mkdir -p "$dst"
for f in "${files[@]}"; do
	cp "$src/assets/$f" "$dst/$f"
done

{
	# The checkout may come from a private server, so its remote URL isn't
	# recorded unless SUPERDUFS_REPO names a public one.
	echo "repo: ${SUPERDUFS_REPO:-superdufs, fork of https://github.com/sigoden/dufs}"
	echo "commit: $(git -C "$src" rev-parse HEAD)"
	for f in "${files[@]}"; do
		echo "sha256 $f $(sha256sum "$dst/$f" | cut -d' ' -f1)"
	done
} >"$dst/SOURCE"

cd "$root"
go test ./webui/ -run 'TestAssets|TestAPISurface' -count=1
