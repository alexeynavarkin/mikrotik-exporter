#!/usr/bin/env bash
# Stop the CHR started by run-chr.sh: ACPI power-down via the QEMU monitor, then SIGTERM.
# The writable overlay (run/chr-<ver>.qcow2) is kept; use CHR_FRESH=1 run-chr.sh to reset.
set -euo pipefail
SCRIPTS=$(cd "$(dirname "$0")" && pwd)
CHR_HOME=${CHR_HOME:-${XDG_CACHE_HOME:-$HOME/.cache}/mikrotik-exporter-chr}
RUN=$CHR_HOME/run
PIDFILE=$RUN/qemu.pid
[ -s "$PIDFILE" ] || { echo "no pidfile, not running"; exit 0; }
PID=$(cat "$PIDFILE")
if ! kill -0 "$PID" 2>/dev/null || ! grep -q qemu-system "/proc/$PID/cmdline" 2>/dev/null; then
  echo "pid $PID is not a running qemu; removing stale pidfile"; rm -f "$PIDFILE"; exit 0
fi
if [ -s "$RUN/monitor.port" ]; then
  python3 -I - "$(cat "$RUN/monitor.port")" <<'PY' || true
import socket, sys, time
s = socket.create_connection(("127.0.0.1", int(sys.argv[1])), timeout=5); time.sleep(0.2)
s.sendall(b"system_powerdown\n"); time.sleep(0.5); s.close()
PY
  for _ in $(seq 1 30); do kill -0 "$PID" 2>/dev/null || break; sleep 1; done
fi
if kill -0 "$PID" 2>/dev/null; then kill "$PID"; sleep 1; fi
kill -0 "$PID" 2>/dev/null && kill -9 "$PID"
rm -f "$PIDFILE"
echo "CHR (pid $PID) stopped"
