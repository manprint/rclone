#!/usr/bin/env bash
# End-to-end check of the web UI on a compiled rclone serving a temporary
# directory: the pages for browsers, the listing for other clients, writes
# with an Origin, the zip and json views, the refusal to overwrite, the
# switch that turns the UI off, and the download tokens and public links.
# Prints "smoke OK" and exits 0 if everything is as expected.
set -euo pipefail

cd "$(dirname "${BASH_SOURCE[0]}")/../.."

TMP=$(mktemp -d)
PID=""
CODE=""

stop_server() {
	if [ -n "$PID" ]; then
		kill "$PID" 2>/dev/null || true
		wait "$PID" 2>/dev/null || true
		PID=""
	fi
}

cleanup() {
	stop_server
	rm -rf "$TMP"
}
trap cleanup EXIT

fail() {
	echo "FAIL: $*" >&2
	if [ -f "$TMP/server.log" ]; then
		echo "--- server log" >&2
		tail -n 20 "$TMP/server.log" >&2
	fi
	exit 1
}

PORT=$(python3 -c 'import socket; s = socket.socket(); s.bind(("127.0.0.1", 0)); print(s.getsockname()[1])')
BASE="http://127.0.0.1:$PORT"
ORIGIN=$BASE
BROWSER=(-H 'User-Agent: Mozilla/5.0' -H 'Accept: text/html')

echo "building rclone"
go build -o "$TMP/rclone" .
mkdir "$TMP/root"

# start_server [flags...] runs the server on $PORT, stopping a running one
start_server() {
	stop_server
	RCLONE_CONFIG=/notfound "$TMP/rclone" serve webdav --addr "127.0.0.1:$PORT" "$@" "$TMP/root" >"$TMP/server.log" 2>&1 &
	PID=$!
	for _ in $(seq 100); do
		kill -0 "$PID" 2>/dev/null || fail "server exited at start"
		if (exec 3<>"/dev/tcp/127.0.0.1/$PORT") 2>/dev/null; then
			return
		fi
		sleep 0.1
	done
	fail "server not listening on port $PORT after 10s"
}

# http METHOD PATH [curl args...] sets CODE and saves the body and headers
# of the response in $TMP/body and $TMP/headers
http() {
	local method=$1 path=$2
	shift 2
	CODE=$(curl -s -o "$TMP/body" -D "$TMP/headers" -w '%{http_code}' -X "$method" "$@" "$BASE$path") || true
}

expect_code() {
	[ "$CODE" = "$2" ] || fail "$1: expected HTTP $2, got $CODE"
}

expect_body() {
	grep -qF -- "$2" "$TMP/body" || fail "$1: expected the body to contain '$2', got: $(head -c 200 "$TMP/body")"
}

expect_no_body() {
	if grep -qF -- "$2" "$TMP/body"; then
		fail "$1: expected the body not to contain '$2'"
	fi
}

echo "1. browser GET / is the web UI"
start_server
http GET / "${BROWSER[@]}"
expect_code "browser GET /" 200
expect_body "browser GET /" index-data
expect_body "browser GET /" __webui_

echo "2. other clients get the upstream listing"
http GET /
expect_code "curl GET /" 200
expect_no_body "curl GET /" index-data

echo "3. PUT with an Origin creates the parents"
http PUT /newdir/sub/f.txt -H "Origin: $ORIGIN" --data-binary 'smoke content'
expect_code "PUT /newdir/sub/f.txt" 201
http GET /newdir/sub/f.txt
expect_code "GET /newdir/sub/f.txt" 200
expect_body "GET /newdir/sub/f.txt" 'smoke content'

echo "4. ?json lists the directory"
http GET '/?json' "${BROWSER[@]}"
expect_code "GET /?json" 200
expect_body "GET /?json" '"newdir"'

echo "5. ?zip downloads a directory"
http GET '/newdir/?zip'
expect_code "GET /newdir/?zip" 200
grep -qi '^content-type:.*zip' "$TMP/headers" || fail "GET /newdir/?zip: expected a zip Content-Type, got: $(grep -i '^content-type' "$TMP/headers" || true)"
python3 -I -c 'import sys, zipfile; names = zipfile.ZipFile(sys.argv[1]).namelist(); sys.exit(0 if any(n.endswith("sub/f.txt") for n in names) else "sub/f.txt not in zip: %s" % names)' "$TMP/body" ||
	fail "GET /newdir/?zip: sub/f.txt missing from the zip"

echo "6. MOVE does not overwrite by default"
http PUT /dst/keep.txt -H "Origin: $ORIGIN" --data-binary 'keep'
expect_code "PUT /dst/keep.txt" 201
http MOVE /newdir/sub/f.txt -H "Origin: $ORIGIN" -H "Destination: $BASE/dst"
expect_code "MOVE onto an existing directory" 412
http GET /dst/keep.txt
expect_code "GET /dst/keep.txt after the refused MOVE" 200
http GET /newdir/sub/f.txt
expect_code "GET /newdir/sub/f.txt after the refused MOVE" 200

echo "7. --webui=false serves no web UI"
start_server --webui=false
http GET / "${BROWSER[@]}"
expect_code "browser GET / with --webui=false" 200
expect_no_body "browser GET / with --webui=false" index-data

echo "8. tokens and public links with authentication"
export RCLONE_WEBUI_SECRET=smoke
start_server --user u --pass p --webui-publink-paths newdir
http GET / "${BROWSER[@]}"
expect_code "unauthenticated GET /" 401
http GET '/newdir/sub/f.txt?tokengen' -u u:p
expect_code "GET ?tokengen" 200
TOKEN=$(cat "$TMP/body")
[ -n "$TOKEN" ] || fail "GET ?tokengen: empty token"
http GET "/newdir/sub/f.txt?token=$TOKEN"
expect_code "unauthenticated GET ?token" 200
expect_body "unauthenticated GET ?token" 'smoke content'
http GET '/?json' "${BROWSER[@]}" -u u:p
expect_code "authenticated GET /?json" 200
PUBTOKEN=$(python3 -I -c 'import json, sys; print(next((p.get("publink_token", "") for p in json.load(sys.stdin)["paths"] if p["name"] == "newdir"), ""))' <"$TMP/body")
[ -n "$PUBTOKEN" ] || fail "GET /?json: no publink_token for newdir in: $(head -c 300 "$TMP/body")"
http GET "/@public/$PUBTOKEN/newdir/sub/f.txt"
expect_code "unauthenticated GET /@public/" 200
expect_body "unauthenticated GET /@public/" 'smoke content'

echo "smoke OK"
