#!/usr/bin/env bash
# Fails if the fork changes anything outside the files it owns, or more of
# cmd/serve/webdav/webdav.go than the marked lines it adds.
#
# The upstream changes carried as backports (webui/backports/*.patch, one
# commit each) count as part of upstream: the fork is compared with the
# upstream base with the patches applied, so a backported file must match
# its patch exactly.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

BASE=${UPSTREAM_REF:-}
if [ -z "$BASE" ]; then
	if git remote | grep -qx upstream; then
		BASE=upstream/master
	else
		BASE=origin/master
	fi
fi
MB=$(git merge-base HEAD "$BASE")

fail=0
REF_TREE=$MB
shopt -s nullglob
patches=(webui/backports/*.patch)
shopt -u nullglob
if [ ${#patches[@]} -gt 0 ]; then
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	export GIT_INDEX_FILE=$tmp/index
	git read-tree "$MB"
	for patch in "${patches[@]}"; do
		if ! git apply --cached "$patch"; then
			echo "footprint: backport $patch doesn't apply to $BASE: upstream changed the same lines; if upstream merged it, delete the patch and keep upstream's code" >&2
			exit 1
		fi
	done
	REF_TREE=$(git write-tree)
	unset GIT_INDEX_FILE
	dups=$(for patch in "${patches[@]}"; do git apply --numstat "$patch" | cut -f3; done | sort | uniq -d)
	if [ -n "$dups" ]; then
		echo "footprint: files changed by more than one backport: $dups" >&2
		fail=1
	fi
fi
# Only changes to tracked files count (staged and working tree included):
# untracked files are not part of the fork until committed.
allowed='^(webui/|fork-doc/|docker-serve-webdav/|\.github/workflows/fork-[^/]*\.yml$|cmd/serve/webdav/[^/]*_webui[^/]*\.go$|cmd/serve/webdav/webdav\.go$)'
while IFS= read -r file; do
	if ! [[ $file =~ $allowed ]]; then
		echo "footprint: file outside the fork footprint: $file" >&2
		fail=1
	fi
done < <(git diff --name-only "$REF_TREE")

diff=$(git diff -U0 "$REF_TREE" -- cmd/serve/webdav/webdav.go)
if grep -Eq '^-[^-]' <<<"$diff"; then
	echo "footprint: webdav.go has removed or modified lines" >&2
	fail=1
fi
added=0
while IFS= read -r line; do
	added=$((added + 1))
	if [[ $line != *fork:webui* ]]; then
		echo "footprint: webdav.go added line without fork:webui marker: ${line#+}" >&2
		fail=1
	fi
done < <(grep -E '^\+[^+]' <<<"$diff" || true)
if [ "$added" -gt 3 ]; then
	echo "footprint: webdav.go has $added added lines, at most 3 allowed" >&2
	fail=1
fi

if [ "$fail" -ne 0 ]; then
	exit 1
fi
echo "footprint OK"
