---
name: power-chess-log-investigator
description: Investigates bugs and protocol issues by reading Docker session logs and server stdout first; use when debugging gameplay, WebSocket, timers, pause/resume, or asking where logs live.
---

# Power Chess — log investigator

## Default workflow

1. **Session logs (authoritative for local Docker)**  
   - Each `docker compose up` creates a new append-only file under `logs/docker/` in the repo (bind mount; folder is gitignored but visible in the editor). `scripts/session-log-wrap.sh` prunes older `*.log` files per folder (default: keep the 5 newest; `POWER_CHESS_SESSION_LOG_KEEP`, `0` disables).  
   - **Server:** `logs/docker/server/<yyyy-mm-dd>:<hh:mm:ss>.log`  
   - **Postgres:** `logs/docker/postgres/<same pattern>.log`  
   - Logs are produced by `scripts/session-log-wrap.sh` (stdout/stderr tee). The **server** image runs the wrap as root (so `tee` can write bind-mounted host dirs) and starts `power-chess-server` with `su-exec` as `appuser`. Postgres uses the same script without `SESSION_LOG_RUN_AS`. Match debug lines use `log.Printf` with prefix `match_debug`.  
   - **`.dockerignore`:** `logs/` is ignored for `docker build` context only; it does **not** prevent bind mounts from writing logs to the host. Always read investigation files from the workspace path `logs/docker/...` on the host. After changing server code, rebuild/restart the container so logs match the current binary.

2. **Always use the single newest log file (mandatory)**  
   - **Do not** pick a log by memory, room id, or “a file you saw earlier.”  
   - **Do** use the **most recently modified** file in each directory (newest session wins).  
   - Shell (server): `ls -t logs/docker/server/*.log 2>/dev/null | head -1`  
   - Shell (postgres): `ls -t logs/docker/postgres/*.log 2>/dev/null | head -1`  
   - If that prints nothing, the folder is empty—say so and ask for a fresh `docker compose up` or manual `tee` capture.  
   - Rationale: filenames are timestamped; `ls -t` by mtime is reliable even if the clock or naming pattern changes slightly.

3. **Reproduce or narrow the time window**  
   Note room id, player seat, and approximate timestamp; **grep only that latest file** for `room=`, `match_debug`, `protocol_err`, `handler_err`, `queue_reaction`, `set_debug_pause`, `ConfirmMulligan`, or the WebSocket message type in question.

4. **Correlate with code**  
   After the log points to a handler or error string, open the matching code in `internal/server/` or `internal/match/` and add tests if the fix is non-trivial.  
   - If the error substring **no longer exists** in the Go tree (`rg` / grep), the log likely predates a fix—confirm with the user and capture a **new** session log after rebuild/restart.

5. **Host-run server (no Docker)**  
   If the user runs the Go binary directly, there is no automatic session file; use the terminal output they captured or run with shell redirection: `tee -a manual-session.log`.

## When not to rely on old per-room files

Legacy per-room files under `logs/*.txt` are not used; prefer session logs under `logs/docker/`.
