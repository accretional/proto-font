#!/usr/bin/env bash
# LET_IT_RIP.sh — top-level "ship it" entry point. Runs setup, build, tests,
# then renders chrome-testing screenshots into chrome-testing/screenshots/.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

log()  { printf '\033[1;32m[LET_IT_RIP]\033[0m %s\n' "$*"; }

"$ROOT/test.sh"

if [ "${SKIP_SCREENSHOTS:-0}" = "1" ]; then
  log "screenshots skipped (SKIP_SCREENSHOTS=1)"
else
  log "generating font HTML samples"
  bash "$ROOT/chrome-testing/gen_html.sh"

  log "capturing screenshots → chrome-testing/screenshots/"
  bash "$ROOT/chrome-testing/snap.sh" \
    "$ROOT/chrome-testing/html/" \
    "$ROOT/chrome-testing/screenshots/"

  log "screenshots written — eyeball any diffs before commit/push"
fi

log "all systems go"
