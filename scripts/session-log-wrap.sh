#!/bin/sh
# Wraps a container main command: tee all stdout/stderr to one append-only session file.
# Filename: <yyyy-mm-dd>:<hh:mm:ss>.log (POSIX sh + busybox; no bash process substitution).
# Set POWER_CHESS_SESSION_LOG_DIR to override log directory (default /var/log/power-chess).
# Set POWER_CHESS_SESSION_LOG_KEEP to retain only the N newest *.log files in that directory
# (default 5). Set to 0 to disable pruning. Applies to each bind-mounted log folder on the host.

set -e
LOG_ROOT="${POWER_CHESS_SESSION_LOG_DIR:-/var/log/power-chess}"
mkdir -p "$LOG_ROOT"
STAMP="$(date +%Y-%m-%d:%H:%M:%S)"
LOG_FILE="${LOG_ROOT}/${STAMP}.log"
touch "$LOG_FILE"
KEEP_RAW="${POWER_CHESS_SESSION_LOG_KEEP:-5}"
case "$KEEP_RAW" in '' | *[!0-9]*) KEEP_N=5 ;; *) KEEP_N="$KEEP_RAW" ;; esac
if [ "$KEEP_N" -gt 0 ]; then
	(
		cd "$LOG_ROOT" || exit 0
		i=0
		for f in $(ls -tA 2>/dev/null); do
			case "$f" in *.log) ;; *) continue ;; esac
			i=$((i + 1))
			if [ "$i" -gt "$KEEP_N" ]; then
				rm -f "$f"
			fi
		done
	)
fi
echo "[session-log-wrap] appending stdout/stderr to $LOG_FILE" >&2

fifo="/tmp/pc-session-log-$$.fifo"
rm -f "$fifo"
mkfifo "$fifo"
tee -a "$LOG_FILE" <"$fifo" &
exec >"$fifo" 2>&1
# When set (server image), drop privileges so the app user is not root; tee stays root and can write bind-mounted log dirs.
if [ -n "$SESSION_LOG_RUN_AS" ]; then
	if ! command -v su-exec >/dev/null 2>&1; then
		echo "session-log-wrap: SESSION_LOG_RUN_AS=$SESSION_LOG_RUN_AS but su-exec is not installed" >&2
		exit 1
	fi
	exec su-exec "$SESSION_LOG_RUN_AS" "$@"
fi
exec "$@"
