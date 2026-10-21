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
	cat >&2 <<'MSG'

To resolve:
  1. Edit the files above keeping upstream's code. In
     cmd/serve/webdav/webdav.go re-add the three lines marked // fork:webui:
       - the struct field    webui *webuiGlue // fork:webui
       - the option          w.webuiPreAuth(), // fork:webui   (in libhttp.NewServer)
       - the call            w.webuiUse(router) // fork:webui  (before router.Handle)
  2. git add <files> && git commit
  3. Re-run this script (the merge is then a no-op and only the checks run).
Or give up with: git merge --abort
MSG
	exit 1
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
