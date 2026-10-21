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
	# Strip any userinfo so credentials never end up in the repository.
	echo "repo: $(git -C "$src" remote get-url origin | sed -E 's#(://)[^/@]+@#\1#')"
	echo "commit: $(git -C "$src" rev-parse HEAD)"
	for f in "${files[@]}"; do
		echo "sha256 $f $(sha256sum "$dst/$f" | cut -d' ' -f1)"
	done
} >"$dst/SOURCE"

cd "$root"
go test ./webui/ -run 'TestAssets|TestAPISurface' -count=1
