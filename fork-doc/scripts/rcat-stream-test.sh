#!/usr/bin/env bash
# Streams data with `rclone rcat` to `rclone serve webdav|sftp|s3`, with client
# and server each in a container limited to 1 GiB of memory and 1 CPU, a
# read-only root and a 64 MiB tmpfs /tmp, so any spooling of the stream fails.
#
# Usage: rcat-stream-test.sh PROTO LABEL MIB [serve flags...]
#   PROTO   webdav, sftp or s3
#   LABEL   name of the run, used for the CSV of the samples
#   MIB     size of the stream in MiB
#
# Environment:
#   T       work dir holding bin/$B (a static rclone), data/ and logs/ (default ./srvtest)
#   B       rclone binary name in $T/bin (default rclone)
#   CLIX    extra flags for the client rcat
#   VENDOR  webdav vendor of the client remote (default rclone)
#   SRC     command writing the stream (default MIB of /dev/urandom)
#   TMPFS   size of /tmp in both containers (default 64m)
#   DATA    directory served (default /t/data); with SRVX="--tmpfs /d:size=96m,mode=1777"
#           and DATA=/d it simulates a full disk
#   SRVX    extra docker run flags for the server
#   PRE     create dumps/x.bin of PRE MiB first, to test overwriting
#   KILL    kill the client after KILL seconds
#   LSAFTER list dumps/ through the server at the end
#
# Prints the client output, the file stored and its md5 (compare with src_md5),
# the elapsed time, the server CPU and the maximum memory, /tmp and disk use
# sampled every second.
set -u
T=${T:-$PWD/srvtest}
B=${B:-rclone}
proto=$1 label=$2 mib=$3
shift 3
SRC=${SRC:-"head -c $((mib * 1048576)) /dev/urandom"}
DATA=${DATA:-/t/data}
img=alpine:3.24
docker rm -f rcs-srv rcs-cli >/dev/null 2>&1
mkdir -p "$T/logs"
docker run --rm -v "$T:/t" $img sh -c 'mkdir -p /t/data/dumps && rm -f /t/data/dumps/x.bin'
if [ -n "${PRE:-}" ]; then
	docker run --rm -v "$T:/t" $img sh -c "head -c $((PRE * 1048576)) /dev/urandom > /t/data/dumps/x.bin"
	echo "pre: md5=$(docker run --rm -v "$T:/t" $img md5sum /t/data/dumps/x.bin | cut -c1-12)"
fi
lim="--memory=1g --memory-swap=1g --cpus=1 --read-only --tmpfs /tmp:size=${TMPFS:-64m},mode=1777 -e TMPDIR=/tmp"
case $proto in
webdav)
	srv="serve webdav $DATA --addr :8080"
	cenv="-e RCLONE_CONFIG_R_TYPE=webdav -e RCLONE_CONFIG_R_URL=http://127.0.0.1:8080 -e RCLONE_CONFIG_R_VENDOR=${VENDOR:-rclone}"
	;;
sftp)
	srv="serve sftp $DATA --addr :2022 --user u --pass p"
	pass=$(docker run --rm -v "$T/bin:/b:ro" $img /b/$B obscure p)
	cenv="-e RCLONE_CONFIG_R_TYPE=sftp -e RCLONE_CONFIG_R_HOST=127.0.0.1 -e RCLONE_CONFIG_R_PORT=2022 -e RCLONE_CONFIG_R_USER=u -e RCLONE_CONFIG_R_PASS=$pass"
	;;
s3)
	srv="serve s3 $DATA --addr :8080 --auth-key accesskey1,secretkey1"
	cenv="-e RCLONE_CONFIG_R_TYPE=s3 -e RCLONE_CONFIG_R_PROVIDER=Rclone -e RCLONE_CONFIG_R_ENDPOINT=http://127.0.0.1:8080 -e RCLONE_CONFIG_R_ACCESS_KEY_ID=accesskey1 -e RCLONE_CONFIG_R_SECRET_ACCESS_KEY=secretkey1"
	;;
*)
	echo "unknown protocol $proto" >&2
	exit 1
	;;
esac
# shellcheck disable=SC2086
docker run -d --name rcs-srv $lim ${SRVX:-} --log-driver none -v "$T:/t" $img \
	/t/bin/$B $srv --cache-dir /tmp/cache --log-file /t/logs/rcs-srv.log --log-level INFO "$@" >/dev/null
sleep 3
docker exec rcs-srv mkdir -p "$DATA/dumps"
# The stream goes through a fifo to md5sum so its md5 can be compared with the stored file
# shellcheck disable=SC2086
docker run -d --name rcs-cli --network container:rcs-srv $lim -v "$T/bin:/b:ro" \
	-e RCLONE_CONFIG=/tmp/none.conf -e RCLONE_CACHE_DIR=/tmp/cache $cenv $img sh -c "
	mkfifo /tmp/f; md5sum /tmp/f > /tmp/md5 &
	$SRC | tee /tmp/f | time /b/$B rcat R:dumps/x.bin ${CLIX:-} --log-level INFO 2>&1
	echo rcat_rc=\$?; wait; echo src_md5=\$(cut -c1-12 /tmp/md5)" >/dev/null
[ -n "${KILL:-}" ] && (sleep "$KILL"; docker kill rcs-cli >/dev/null) &
csv=$T/logs/rcs-$label.csv
echo "t,srv_anon,srv_tmp_k,data_k,files,cli_anon,cli_tmp_k" >"$csv"
t0=$(date +%s)
while [ "$(docker inspect -f '{{.State.Running}}' rcs-cli 2>/dev/null)" = true ]; do
	s=$(docker exec -e D="$DATA" rcs-srv sh -c 'echo $(($(grep "^anon " /sys/fs/cgroup/memory.stat|cut -d" " -f2)>>20)),$(du -sk /tmp|cut -f1),$(du -sk $D/dumps|cut -f1),$(ls -a $D/dumps|wc -l)' 2>/dev/null)
	c=$(docker exec rcs-cli sh -c 'echo $(($(grep "^anon " /sys/fs/cgroup/memory.stat|cut -d" " -f2)>>20)),$(du -sk /tmp|cut -f1)' 2>/dev/null)
	[ -n "$c" ] && echo "$(($(date +%s) - t0)),$s,$c" >>"$csv"
	sleep 1
done
el=$(($(date +%s) - t0))
# After a kill the server only notices when the connection closes: wait for it to clean up
if [ -n "${KILL:-}" ]; then
	for _ in $(seq 20); do
		grep -qE "Rcat failed|Removing partially|Write error" "$T/logs/rcs-srv.log" && break
		sleep 0.5
	done
fi
srvcpu=$(docker exec rcs-srv sh -c 'grep usage_usec /sys/fs/cgroup/cpu.stat' | cut -d' ' -f2)
dst=$(docker exec -e D="$DATA" rcs-srv sh -c 'f=$D/dumps/x.bin; [ -f $f ] && echo size=$(stat -c%s $f) md5=$(md5sum $f|cut -c1-12) || echo missing; ls -a $D/dumps | tr "\n" " "')
echo "== $label ($proto, $mib MiB, $B, srv: $*, cli: ${CLIX:-})"
docker logs rcs-cli 2>&1 | grep -vE '^\s*$' | tail -8 | sed 's/^/  cli| /'
echo "  dst: $dst"
echo "  elapsed ${el}s, srv cpu $((srvcpu / 1000)) ms"
awk -F, 'NR>1{for(i=2;i<=NF;i++) if($i>m[i]) m[i]=$i} END{printf "  max: srv_anon=%dMiB srv_tmp=%dKiB data=%dMiB files=%d cli_anon=%dMiB cli_tmp=%dKiB samples=%d\n", m[2],m[3],m[4]/1024,m[5]-2,m[6],m[7],NR-1}' "$csv"
if [ -n "${LSAFTER:-}" ]; then
	# shellcheck disable=SC2086
	docker run --rm --network container:rcs-srv -v "$T/bin:/b:ro" -e RCLONE_CONFIG=/tmp/none.conf $cenv $img \
		/b/$B lsl R:dumps 2>&1 | sed 's/^/  ls| /'
fi
grep -iE "error|err=" "$T/logs/rcs-srv.log" | grep -v 'login failed' | tail -3 | sed 's/^/  srv| /'
docker rm -f rcs-srv rcs-cli >/dev/null 2>&1
docker run --rm -v "$T:/t" $img rm -f /t/data/dumps/x.bin
rm -f "$T/logs/rcs-srv.log"
