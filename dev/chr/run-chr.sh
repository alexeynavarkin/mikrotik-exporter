#!/usr/bin/env bash
# Boot a MikroTik RouterOS CHR (Cloud Hosted Router) in QEMU, headless, in the background,
# with the RouterOS API forwarded to 127.0.0.1. Idempotent: if it is already running it only
# re-checks the API and the demo config.
#
# Env (all optional):
#   CHR_VERSION    RouterOS version (default 7.24.5 = "stable" on 2026-10-10; 7.23.8 = "long-term")
#   CHR_HOME       images, overlays and state (default: ~/.cache/mikrotik-exporter-chr, ~450MB)
#   CHR_API_PORT   host port for API      (default 8728)
#   CHR_APISSL_PORT host port for API-SSL (default 8729)
#   CHR_SSH_PORT   host port for SSH      (default 2222)
#   CHR_HTTP_PORT  host port for WebFig   (default 8080)
#   CHR_MON_PORT   QEMU monitor on 127.0.0.1 (default 45454; unix socket paths here are too long)
#   CHR_MEM        guest RAM MiB (default 512), CHR_CPUS (default 2)
#   CHR_FRESH=1    discard the writable overlay (factory-fresh router)
#   CHR_DEMO=0     skip creating demo objects (wireguard peers, dhcp lease, extra disk)
#   CHR_PASSWORD   if set, set admin's password to this (default: keep empty password)
#   CHR_BOOT_TIMEOUT seconds to wait for the API (default 600)
#
# Image source: download.mikrotik.com if reachable, otherwise MikroTik's official Docker Hub
# image mikrotik/chr:<version>, whose layer contains diskimage/chr-<version>.img (raw).
set -euo pipefail

SCRIPTS=$(cd "$(dirname "$0")" && pwd)
REPO=$(cd "$SCRIPTS/../.." && pwd)
CHR_HOME=${CHR_HOME:-${XDG_CACHE_HOME:-$HOME/.cache}/mikrotik-exporter-chr}
VER=${CHR_VERSION:-7.24.5}
API_PORT=${CHR_API_PORT:-8728}
APISSL_PORT=${CHR_APISSL_PORT:-8729}
SSH_PORT=${CHR_SSH_PORT:-2222}
HTTP_PORT=${CHR_HTTP_PORT:-8080}
MON_PORT=${CHR_MON_PORT:-45454}
MEM=${CHR_MEM:-512}
CPUS=${CHR_CPUS:-2}
DL=$CHR_HOME/downloads
RUN=$CHR_HOME/run
BIN=$CHR_HOME/bin
ROSQ=$BIN/rosq
PIDFILE=$RUN/qemu.pid
BASE=$DL/chr-$VER.img
mkdir -p "$DL" "$RUN" "$BIN"

log() { printf '[run-chr %s] %s\n' "$(date +%T)" "$*" >&2; }

need() { command -v "$1" >/dev/null || { log "missing $1 (apt-get install -y qemu-system-x86 qemu-utils)"; exit 1; }; }
need qemu-system-x86_64; need qemu-img; need curl; need python3

# --- rosq: tiny RouterOS API client used for readiness + setup ----------------------------
if [ ! -x "$ROSQ" ]; then
  need go
  log "building rosq"
  (cd "$REPO" && go build -o "$ROSQ" ./dev/rosq)
fi
PASS_NOW=""
rq() { "$ROSQ" -a "127.0.0.1:$API_PORT" -p "$PASS_NOW" "$@"; }

# --- image -------------------------------------------------------------------------------
fetch_image() {
  local tmp="$DL/official-$VER"
  if curl -fsS --max-time 20 -o "$DL/chr-$VER.img.zip" \
       "https://download.mikrotik.com/routeros/$VER/chr-$VER.img.zip" 2>/dev/null; then
    mkdir -p "$tmp" && unzip -o -q "$DL/chr-$VER.img.zip" -d "$tmp"
    cp "$(find "$tmp" -name '*.img' | head -1)" "$BASE"
    log "image from download.mikrotik.com"
    return
  fi
  rm -f "$DL/chr-$VER.img.zip"
  log "download.mikrotik.com not reachable; using Docker Hub mikrotik/chr:$VER (official MikroTik image)"
  local layers="$DL/mikrotik-chr-$VER"
  "$SCRIPTS/pull-layers.sh" mikrotik/chr "$VER" "$layers" >/dev/null
  local f
  for f in "$layers"/*.tar; do
    if tar -tf "$f" | grep -q "^diskimage/chr-.*\.img$"; then
      mkdir -p "$layers/x"
      tar -xf "$f" -C "$layers/x" --wildcards 'diskimage/chr-*.img'
    fi
  done
  cp "$(ls "$layers"/x/diskimage/chr-*.img | head -1)" "$BASE"
}
[ -s "$BASE" ] || fetch_image
log "base image: $BASE ($(qemu-img info --output=json "$BASE" | python3 -I -c 'import json,sys;print(json.load(sys.stdin)["format"])'))"

running() { [ -s "$PIDFILE" ] && kill -0 "$(cat "$PIDFILE")" 2>/dev/null && grep -q qemu-system "/proc/$(cat "$PIDFILE")/cmdline" 2>/dev/null; }

if running; then
  log "already running, pid $(cat "$PIDFILE")"
else
  OVL=$RUN/chr-$VER.qcow2
  [ "${CHR_FRESH:-0}" = 1 ] && rm -f "$OVL" "$RUN/data.qcow2"
  [ -f "$OVL" ] || qemu-img create -q -f qcow2 -F raw -b "$BASE" "$OVL"
  [ -f "$RUN/data.qcow2" ] || qemu-img create -q -f qcow2 "$RUN/data.qcow2" 64M
  ACCEL="tcg,thread=multi"
  if [ -w /dev/kvm ]; then ACCEL=kvm; fi
  log "starting qemu (accel=$ACCEL, ${MEM}MiB, $CPUS vCPU), API -> 127.0.0.1:$API_PORT"
  rm -f "$RUN/serial.log"
  echo "$MON_PORT" > "$RUN/monitor.port"
  date +%s > "$RUN/start.ts"
  qemu-system-x86_64 -name chr -accel "$ACCEL" -smp "$CPUS" -m "$MEM" \
    -drive file="$OVL",if=virtio,format=qcow2 \
    -drive file="$RUN/data.qcow2",if=virtio,format=qcow2 \
    -netdev user,id=n0,hostfwd=tcp:127.0.0.1:$API_PORT-:8728,hostfwd=tcp:127.0.0.1:$APISSL_PORT-:8729,hostfwd=tcp:127.0.0.1:$SSH_PORT-:22,hostfwd=tcp:127.0.0.1:$HTTP_PORT-:80 \
    -device virtio-net-pci,netdev=n0 \
    -serial file:"$RUN/serial.log" -monitor tcp:127.0.0.1:$MON_PORT,server,nowait \
    -display none -pidfile "$PIDFILE" -daemonize
fi

# --- wait for API login (QEMU's hostfwd accepts TCP immediately, so test a real login) ----
T0=$(cat "$RUN/start.ts" 2>/dev/null || date +%s)
DEADLINE=$(( $(date +%s) + ${CHR_BOOT_TIMEOUT:-600} ))
until rq -q -t 5s 2>/dev/null || { [ -n "${CHR_PASSWORD:-}" ] && PASS_NOW=$CHR_PASSWORD rq -q -t 5s 2>/dev/null && PASS_NOW=$CHR_PASSWORD; }; do
  running || { log "qemu died; see $RUN/serial.log"; exit 1; }
  [ "$(date +%s)" -lt "$DEADLINE" ] || { log "timeout waiting for API"; exit 1; }
  sleep 3
done
log "API login OK ($(( $(date +%s) - T0 ))s since qemu start)"

# --- setup -------------------------------------------------------------------------------
# api service is enabled by default on CHR; make sure anyway.
rq /ip/service/enable =numbers=api >/dev/null
if [ -n "${CHR_PASSWORD:-}" ] && [ "$PASS_NOW" != "$CHR_PASSWORD" ]; then
  rq /user/set =numbers=admin "=password=$CHR_PASSWORD" >/dev/null && PASS_NOW=$CHR_PASSWORD
  log "admin password set"
fi

if [ "${CHR_DEMO:-1}" = 1 ] && ! rq /interface/wireguard/print ?name=wg-a | grep -q 'name="wg-a"'; then
  log "creating demo objects"
  rq /system/identity/set =name=chr-dev >/dev/null
  rq /interface/wireguard/add =name=wg-a =listen-port=13231 -- /interface/wireguard/add =name=wg-b =listen-port=13232 >/dev/null
  pk() { rq /interface/wireguard/print "?name=$1" =.proplist=public-key | sed -n 's/^ *public-key="\(.*\)"/\1/p'; }
  PKA=$(pk wg-a); PKB=$(pk wg-b)
  # Two interfaces on the same router peered over 127.0.0.1: real handshakes and rx/tx counters.
  rq /interface/wireguard/peers/add =interface=wg-a =name=peer-b "=public-key=$PKB" =endpoint-address=127.0.0.1 =endpoint-port=13232 =allowed-address=10.99.2.0/24 =persistent-keepalive=10s \
     -- /interface/wireguard/peers/add =interface=wg-b =name=peer-a "=public-key=$PKA" =endpoint-address=127.0.0.1 =endpoint-port=13231 =allowed-address=10.99.1.0/24 =persistent-keepalive=10s \
     -- /interface/wireguard/peers/add =interface=wg-a =name=peer-dead =public-key=AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAE= =endpoint-address=192.0.2.1 =endpoint-port=51820 =allowed-address=10.99.9.0/24 \
     -- /ip/address/add =address=10.99.1.1/24 =interface=wg-a -- /ip/address/add =address=10.99.2.1/24 =interface=wg-b >/dev/null
  # DHCP server with static leases on an empty bridge (leases stay "waiting").
  rq /interface/bridge/add =name=br-lan -- /ip/address/add =address=192.168.88.1/24 =interface=br-lan \
     -- /ip/pool/add =name=lan =ranges=192.168.88.100-192.168.88.200 \
     -- /ip/dhcp-server/add =name=lan =interface=br-lan =address-pool=lan \
     -- /ip/dhcp-server/network/add =address=192.168.88.0/24 =gateway=192.168.88.1 \
     -- /ip/dhcp-server/lease/add =server=lan =address=192.168.88.10 =mac-address=02:00:00:00:00:10 =comment=demo-laptop \
     -- /ip/dhcp-server/lease/add =server=lan =address=192.168.88.11 =mac-address=02:00:00:00:00:11 =comment=demo-phone >/dev/null
fi
# The second (64M) virtio disk shows up in /disk as slot pcie1; format it so "free" is reported.
if [ "${CHR_DEMO:-1}" = 1 ] && rq /disk/print ?slot=pcie1 =.proplist=fs | grep -q 'fs="-"'; then
  log "formatting data disk pcie1 (ext4)"
  rq -t 120s /disk/format =numbers=pcie1 =file-system=ext4 =mbr-partition-table=no =label=data >/dev/null
fi

cat >&2 <<EOF
CHR $VER running: pid $(cat "$PIDFILE")
  API      127.0.0.1:$API_PORT (user admin, password '${PASS_NOW}')
  API-SSL  127.0.0.1:$APISSL_PORT   SSH 127.0.0.1:$SSH_PORT   WebFig http://127.0.0.1:$HTTP_PORT
  query:   $ROSQ -a 127.0.0.1:$API_PORT /system/resource/print
  stop:    $SCRIPTS/stop-chr.sh
EOF
