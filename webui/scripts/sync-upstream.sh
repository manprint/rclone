#!/usr/bin/env bash
# Merges upstream rclone into the current branch and checks the fork still
# holds. Never pushes. Environment: UPSTREAM_REMOTE (default: "upstream" if
# that remote exists, else "origin"), UPSTREAM_BRANCH (default: master),
# UPSTREAM_REF (a ref to merge instead, eg the release tag v1.76.0).
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

REMOTE=${UPSTREAM_REMOTE:-}
if [ -z "$REMOTE" ]; then
	if git remote | grep -qx upstream; then
		REMOTE=upstream
	else
		REMOTE=origin
	fi
fi
BRANCH=${UPSTREAM_BRANCH:-master}
REF=${UPSTREAM_REF:-$REMOTE/$BRANCH}

# Untracked files are fine, changes to tracked ones are not.
if ! git diff --quiet || ! git diff --cached --quiet; then
	echo "sync-upstream: working tree has uncommitted changes to tracked files; commit or stash them first" >&2
	exit 1
fi

echo "sync-upstream: fetching $REMOTE"
git fetch "$REMOTE"

echo "sync-upstream: merging $REF into $(git rev-parse --abbrev-ref HEAD)"
if ! git -c rerere.enabled=true merge --no-edit "$REF"; then
	echo >&2
	echo "sync-upstream: merge conflict in:" >&2
	git diff --name-only --diff-filter=U | sed 's/^/  /' >&2
	shopt -s nullglob
	for patch in webui/backports/*.patch; do
		files=$(git apply --numstat "$patch" | cut -f3 | grep -Fx -f <(git diff --name-only --diff-filter=U) || true)
		if [ -n "$files" ]; then
			subject=$(awk '/^Subject: /{s=$0; while ((getline l) > 0 && l ~ /^ /) s=s l; sub(/^Subject: (\[PATCH[^]]*\] )?/, "", s); print s; exit}' "$patch")
			echo "  changed by the backport $patch ($subject)" >&2
		fi
	done
	shopt -u nullglob
	cat >&2 <<'MSG'

To resolve:
  1. Edit the files above keeping upstream's code. In
     cmd/serve/webdav/webdav.go re-add the three lines marked // fork:webui:
       - the struct field    webui *webuiGlue // fork:webui
       - the option          w.webuiPreAuth(), // fork:webui   (in libhttp.NewServer)
       - the call            w.webuiUse(router) // fork:webui  (before router.Handle)
  2. git add <files> && git commit
  3. Re-run this script (the merge is then a no-op and only the checks run).
For a file changed by a backport: if upstream merged that change (maybe
reworked), keep upstream's version of the file and git rm the patch;
otherwise keep upstream's code and redo the backport on top of it.
Or give up with: git merge --abort
MSG
	exit 1
fi

# Backports which upstream now contains are dropped: their patch reverses
# cleanly on the merged upstream.
shopt -s nullglob
patches=(webui/backports/*.patch)
shopt -u nullglob
if [ ${#patches[@]} -gt 0 ]; then
	tmp=$(mktemp -d)
	trap 'rm -rf "$tmp"' EXIT
	dropped=()
	for patch in "${patches[@]}"; do
		if GIT_INDEX_FILE=$tmp/index sh -c 'git read-tree "$1" && git apply --cached --reverse --check "$2"' _ "$REF" "$patch" 2>/dev/null; then
			git rm -q -- "$patch"
			dropped+=("$(basename "$patch")")
		fi
	done
	if [ ${#dropped[@]} -gt 0 ]; then
		git commit -q -m "webui: drop the backports merged upstream" -m "${dropped[*]}"
		echo "sync-upstream: dropped the backports merged upstream: ${dropped[*]}"
	fi
fi

# The fork CI is generated from the upstream workflows.
echo "sync-upstream: fork CI"
if ! python3 webui/scripts/gen-ci.py; then
	echo "sync-upstream: an upstream workflow changed where the fork CI is generated from: adapt webui/scripts/gen-ci.py, run it, commit and re-run this script" >&2
	exit 1
fi
if [ -n "$(git status --porcelain -- .github/workflows/fork-*.yml)" ]; then
	git add -- .github/workflows/fork-*.yml
	git commit -q -m "webui: regenerate the fork CI from the upstream workflows"
	echo "sync-upstream: committed the regenerated fork CI"
fi

# Compare the footprint with the ref just merged.
export UPSTREAM_REF="$REF"
echo "sync-upstream: footprint"
webui/scripts/check-footprint.sh
echo "sync-upstream: build"
go build ./...
echo "sync-upstream: tests"
go test ./webui/... ./cmd/serve/webdav/ ./lib/http/...
echo "sync-upstream: tests with the web UI off"
RCLONE_WEBUI=false go test ./cmd/serve/webdav/

echo "sync-upstream: OK, $REF merged and checks passed (nothing pushed)"
