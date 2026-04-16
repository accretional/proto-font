#!/usr/bin/env bash
# LET_IT_RIP.sh — top-level "ship it" entry point. Runs the full test suite
# and renders the e2e screenshot sweep into screenshots/.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

log()  { printf '\033[1;32m[LET_IT_RIP]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[LET_IT_RIP]\033[0m %s\n' "$*" >&2; }

"$ROOT/test.sh"

# E2E screenshots: render every web-loadable fixture under data/fonts/ in
# headless Chrome via the sibling chromerpc repo and write fresh PNGs into
# screenshots/. Visually review any diffs before pushing.
#
# Opt-out:  SKIP_SCREENSHOTS=1
# Override sibling repo location: CHROMERPC_REPO=/path/to/chromerpc
if [ "${SKIP_SCREENSHOTS:-0}" = "1" ]; then
  log "e2e screenshots skipped (SKIP_SCREENSHOTS=1)"
else
  CHROMERPC_REPO="${CHROMERPC_REPO:-$ROOT/../chromerpc}"
  if [ ! -d "$CHROMERPC_REPO" ]; then
    warn "e2e screenshots skipped: $CHROMERPC_REPO not found (set CHROMERPC_REPO or SKIP_SCREENSHOTS=1)"
  else
    chromerpc_bin="$CHROMERPC_REPO/bin/chromerpc"
    automate_bin="$CHROMERPC_REPO/bin/automate"
    if [ ! -x "$chromerpc_bin" ] || [ ! -x "$automate_bin" ]; then
      log "building chromerpc binaries"
      (cd "$CHROMERPC_REPO" \
        && go build -o bin/chromerpc ./cmd/chromerpc \
        && go build -o bin/automate  ./cmd/automate)
    fi

    chromerpc_pid=""
    cleanup_chromerpc() {
      if [ -n "$chromerpc_pid" ]; then
        kill "$chromerpc_pid" 2>/dev/null || true
      fi
    }
    trap cleanup_chromerpc EXIT

    if lsof -iTCP:50051 -sTCP:LISTEN -nP >/dev/null 2>&1; then
      log "chromerpc already running on :50051 — reusing"
    else
      log "starting chromerpc on :50051"
      "$chromerpc_bin" --headless --addr :50051 >/tmp/chromerpc.log 2>&1 &
      chromerpc_pid=$!
      until lsof -iTCP:50051 -sTCP:LISTEN -nP >/dev/null 2>&1; do
        sleep 1
      done
    fi

    log "rendering e2e screenshots → screenshots/"
    UI_E2E=1 \
      SCREENSHOT_OUT_DIR="$ROOT/screenshots" \
      CHROMERPC_AUTOMATE_CMD="$automate_bin" \
      go test -count=1 -run TestChromerpcScreenshots ./ui-e2e-validation/...

    log "screenshots written — eyeball any diffs before commit/push"
  fi
fi

log "all systems go"
