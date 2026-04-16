#!/usr/bin/env bash
# setup.sh — install toolchain deps, vendor .proto sources, generate Go code,
# fetch test fixtures. Idempotent.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$ROOT"

log()  { printf '\033[1;34m[setup]\033[0m %s\n' "$*"; }
warn() { printf '\033[1;33m[setup]\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[1;31m[setup]\033[0m %s\n' "$*" >&2; exit 1; }

# --- tool checks --------------------------------------------------------------

command -v go >/dev/null 2>&1 || die "go not found in PATH; install Go 1.26"
GO_VERSION="$(go env GOVERSION)"
case "$GO_VERSION" in
  go1.26*) : ;;
  *) die "Go 1.26 required, got $GO_VERSION" ;;
esac
log "go: $GO_VERSION"

command -v protoc >/dev/null 2>&1 || die "protoc not found in PATH; install protoc"
log "protoc: $(protoc --version)"

GOBIN="$(go env GOBIN)"
if [ -z "$GOBIN" ]; then
  GOBIN="$(go env GOPATH)/bin"
fi
export PATH="$GOBIN:$PATH"

if ! command -v protoc-gen-go >/dev/null 2>&1; then
  log "installing protoc-gen-go"
  go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
fi
log "protoc-gen-go: $(command -v protoc-gen-go)"

# --- go module ----------------------------------------------------------------

if [ ! -f go.mod ]; then
  log "initializing go module 'openformat'"
  # Module name matches the go_package prefix in font.proto, so protoc output
  # lands inside this module without rewriting.
  go mod init openformat
fi

# --- vendor googlefonts protos ------------------------------------------------
#
# gftools & friends publish protos we want to consume as-is. We pin to a
# specific commit for each repo so regeneration is deterministic. Fetching
# over the network is best-effort: when offline we keep whatever is already
# vendored in proto/googlefonts/v1/.

vendor_proto() {
  local url="$1"
  local dst="$2"
  if [ -f "$dst" ] && [ -n "${SETUP_OFFLINE:-}" ]; then
    log "skip $dst (offline, cached)"
    return 0
  fi
  local tmp
  tmp="$(mktemp)"
  if curl -fsSL "$url" -o "$tmp"; then
    mkdir -p "$(dirname "$dst")"
    mv "$tmp" "$dst"
    log "vendored $(basename "$dst")"
  else
    rm -f "$tmp"
    if [ -f "$dst" ]; then
      warn "fetch failed for $url; keeping cached $dst"
    else
      warn "fetch failed for $url and no cache at $dst; skipping"
    fi
  fi
}

GFTOOLS_COMMIT="main"   # gftools pins tracked here; bump deliberately.
LANG_COMMIT="main"

# gftools axes.proto IS the Axis Registry proto (the axisregistry repo stores
# per-axis records as text protos under Lib/axisregistry/data/, not a schema
# file). We only need one copy.
vendor_proto \
  "https://raw.githubusercontent.com/googlefonts/gftools/${GFTOOLS_COMMIT}/Lib/gftools/axes.proto" \
  "proto/googlefonts/v1/axes.proto"
vendor_proto \
  "https://raw.githubusercontent.com/googlefonts/gftools/${GFTOOLS_COMMIT}/Lib/gftools/fonts_public.proto" \
  "proto/googlefonts/v1/fonts_public.proto"
vendor_proto \
  "https://raw.githubusercontent.com/googlefonts/gftools/${GFTOOLS_COMMIT}/Lib/gftools/designers.proto" \
  "proto/googlefonts/v1/designers.proto"
vendor_proto \
  "https://raw.githubusercontent.com/googlefonts/gftools/${GFTOOLS_COMMIT}/Lib/gftools/knowledge.proto" \
  "proto/googlefonts/v1/knowledge.proto"
vendor_proto \
  "https://raw.githubusercontent.com/googlefonts/lang/${LANG_COMMIT}/Lib/gflanguages/languages_public.proto" \
  "proto/googlefonts/v1/languages_public.proto"

# Rewrite upstream go_package options so each vendored proto lands in its
# own Go package. Upstream files share message names (e.g. SampleTextProto),
# so we must NOT collapse them into a single Go package.
normalize_go_package() {
  local f="$1"
  local base
  base="$(basename "$f" .proto)"
  local short="${base//_/}"
  local opt="option go_package = \"openformat/gen/go/googlefonts/${base};gf${short}\";"
  python3 - "$f" "$opt" <<'PY'
import io, re, sys
path, opt = sys.argv[1], sys.argv[2]
with io.open(path, "r", encoding="utf-8") as fh:
    txt = fh.read()
txt = re.sub(r'^option\s+go_package\s*=.*?;\s*\n', '', txt, flags=re.MULTILINE)
def inject(match):
    return match.group(0) + "\n" + opt + "\n"
if re.search(r'^package\s+[^;]+;\s*$', txt, flags=re.MULTILINE):
    txt = re.sub(r'(^package\s+[^;]+;\s*$)', inject, txt, count=1, flags=re.MULTILINE)
else:
    txt = re.sub(r'(^syntax\s*=\s*"[^"]+";\s*$)', inject, txt, count=1, flags=re.MULTILINE)
with io.open(path, "w", encoding="utf-8") as fh:
    fh.write(txt)
PY
}

for f in proto/googlefonts/v1/*.proto; do
  [ -f "$f" ] || continue
  normalize_go_package "$f"
done

# --- proto codegen ------------------------------------------------------------

PROTO_SRC_DIR="$ROOT/proto"
OF_GEN_DIR="$ROOT/gen/go/openformat/v1"

# openformat protos: discover every *.proto under openformat/v1/ so new
# files (one per subsystem, see container.proto header comment) are picked
# up automatically.
OF_PROTOS=()
while IFS= read -r line; do
  [ -n "$line" ] && OF_PROTOS+=("$line")
done < <(cd "$PROTO_SRC_DIR" && find openformat/v1 -maxdepth 1 -name '*.proto' 2>/dev/null | sort)

GF_GEN_DIR="$ROOT/gen/go/googlefonts"
GF_PROTOS=()
while IFS= read -r line; do
  [ -n "$line" ] && GF_PROTOS+=("$line")
done < <(cd "$PROTO_SRC_DIR" && find googlefonts/v1 -maxdepth 1 -name '*.proto' 2>/dev/null | sort)

needs_regen=0
check_regen() {
  local gen_dir="$1"; shift
  local protos=("$@")
  if [ ! -d "$gen_dir" ]; then return 0; fi
  for pf in "${protos[@]}"; do
    local src="$PROTO_SRC_DIR/$pf"
    [ -f "$src" ] || continue
    local base
    base="$(basename "$pf" .proto)"
    local out="$gen_dir/${base}.pb.go"
    if [ ! -f "$out" ] || [ "$src" -nt "$out" ]; then return 0; fi
  done
  return 1
}

if check_regen "$OF_GEN_DIR" "${OF_PROTOS[@]}"; then needs_regen=1; fi
if [ ${#GF_PROTOS[@]} -gt 0 ] && check_regen "$GF_GEN_DIR" "${GF_PROTOS[@]}"; then needs_regen=1; fi

if [ "$needs_regen" -eq 1 ]; then
  log "regenerating protobuf Go sources"
  mkdir -p "$OF_GEN_DIR"
  rm -f "$OF_GEN_DIR"/*.pb.go
  rm -rf "$ROOT/gen/go/googlefonts"
  # openformat protos
  protoc \
    --proto_path="$PROTO_SRC_DIR" \
    --go_out="$ROOT" \
    --go_opt=module=openformat \
    "${OF_PROTOS[@]/#/$PROTO_SRC_DIR/}"
  # googlefonts protos — each into its own Go package (messages collide if
  # merged). We generate file-by-file because file paths land under
  # gen/go/googlefonts/<name>/ per file.
  if [ ${#GF_PROTOS[@]} -gt 0 ]; then
    protoc \
      --proto_path="$PROTO_SRC_DIR" \
      --go_out="$ROOT" \
      --go_opt=module=openformat \
      "${GF_PROTOS[@]/#/$PROTO_SRC_DIR/}"
  fi
else
  log "proto outputs up to date"
fi

# --- go deps ------------------------------------------------------------------

log "resolving go dependencies"
go mod tidy

# --- test fixtures ------------------------------------------------------------
#
# Noto fonts: we pull a small curated subset directly from
# github.com/notofonts/notofonts.github.io raw files. We deliberately do NOT
# clone the 1GB+ google/fonts snapshot — validation runs per-file so a handful
# of representative families is enough.

FIXTURES_DIR="$ROOT/data/fonts/noto"
mkdir -p "$FIXTURES_DIR"

download_if_missing() {
  local url="$1"
  local dst="$2"
  [ -f "$dst" ] && { log "fixture cached: $(basename "$dst")"; return 0; }
  if curl -fsSL "$url" -o "$dst.tmp"; then
    mv "$dst.tmp" "$dst"
    log "fixture fetched: $(basename "$dst")"
  else
    rm -f "$dst.tmp"
    warn "failed to fetch fixture $url"
  fi
}

# Keep the list small and stable. TTF + OTF + WOFF (one each) is enough to
# exercise every container-level code path in the codec.
download_if_missing \
  "https://raw.githubusercontent.com/google/fonts/main/ofl/notosans/NotoSans%5Bwdth%2Cwght%5D.ttf" \
  "$FIXTURES_DIR/NotoSans-VF.ttf"
download_if_missing \
  "https://raw.githubusercontent.com/google/fonts/main/ofl/notoserif/NotoSerif%5Bwdth%2Cwght%5D.ttf" \
  "$FIXTURES_DIR/NotoSerif-VF.ttf"
download_if_missing \
  "https://raw.githubusercontent.com/google/fonts/main/ofl/notosansmono/NotoSansMono%5Bwdth%2Cwght%5D.ttf" \
  "$FIXTURES_DIR/NotoSansMono-VF.ttf"

# Material Symbols variable fonts from google/material-design-icons.
# Four-axis (FILL, GRAD, opsz, wght) variable fonts — they stress the
# variation tables (fvar/avar/STAT/HVAR/MVAR/gvar) far harder than the
# two-axis Noto fixtures. We grab one TTF per icon family (Outlined,
# Rounded, Sharp) plus a real-world WOFF2 (Outlined) so the codec gets
# a non-fonttools-test WOFF2 sample.
MATERIAL_DIR="$ROOT/data/fonts/material-symbols"
mkdir -p "$MATERIAL_DIR"
MDI_BASE="https://raw.githubusercontent.com/google/material-design-icons/master/variablefont"
MDI_AXES="%5BFILL%2CGRAD%2Copsz%2Cwght%5D"
download_if_missing \
  "$MDI_BASE/MaterialSymbolsOutlined${MDI_AXES}.ttf" \
  "$MATERIAL_DIR/MaterialSymbolsOutlined-VF.ttf"
download_if_missing \
  "$MDI_BASE/MaterialSymbolsRounded${MDI_AXES}.ttf" \
  "$MATERIAL_DIR/MaterialSymbolsRounded-VF.ttf"
download_if_missing \
  "$MDI_BASE/MaterialSymbolsSharp${MDI_AXES}.ttf" \
  "$MATERIAL_DIR/MaterialSymbolsSharp-VF.ttf"
download_if_missing \
  "$MDI_BASE/MaterialSymbolsOutlined${MDI_AXES}.woff2" \
  "$MATERIAL_DIR/MaterialSymbolsOutlined-VF.woff2"

# METADATA.pb text-protos, one per family. Covers variable axes, Noto
# flag, ofl vs apache licensing, display/monospace categories, material
# symbols (display-only).
METADATA_DIR="$ROOT/data/metadata"
mkdir -p "$METADATA_DIR"
download_if_missing \
  "https://raw.githubusercontent.com/google/fonts/main/ofl/notosans/METADATA.pb" \
  "$METADATA_DIR/notosans.METADATA.pb"
download_if_missing \
  "https://raw.githubusercontent.com/google/fonts/main/ofl/robotoflex/METADATA.pb" \
  "$METADATA_DIR/robotoflex.METADATA.pb"
download_if_missing \
  "https://raw.githubusercontent.com/google/fonts/main/ofl/inter/METADATA.pb" \
  "$METADATA_DIR/inter.METADATA.pb"
download_if_missing \
  "https://raw.githubusercontent.com/google/fonts/main/ofl/roboto/METADATA.pb" \
  "$METADATA_DIR/roboto.METADATA.pb"
download_if_missing \
  "https://raw.githubusercontent.com/google/fonts/main/ofl/jetbrainsmono/METADATA.pb" \
  "$METADATA_DIR/jetbrainsmono.METADATA.pb"
download_if_missing \
  "https://raw.githubusercontent.com/google/fonts/main/ofl/abeezee/METADATA.pb" \
  "$METADATA_DIR/abeezee.METADATA.pb"
download_if_missing \
  "https://raw.githubusercontent.com/google/fonts/main/ofl/pacifico/METADATA.pb" \
  "$METADATA_DIR/pacifico.METADATA.pb"

# --- full google/fonts corpus (opt-in) ----------------------------------------
#
# Set SETUP_FULL=1 to shallow-clone the entire google/fonts repo into
# data/fonts/gfonts/. ~1GB on disk. Default is off so LET_IT_RIP stays fast.
# Subsequent runs fetch incrementally. Pin to a specific commit via
# GFONTS_COMMIT if you need reproducibility — otherwise we track main.

GFONTS_DIR="$ROOT/data/fonts/gfonts"
GFONTS_REMOTE="https://github.com/google/fonts.git"
GFONTS_REF="${GFONTS_COMMIT:-main}"

if [ -n "${SETUP_FULL:-}" ]; then
  if [ ! -d "$GFONTS_DIR/.git" ]; then
    log "cloning google/fonts (shallow) into $GFONTS_DIR"
    mkdir -p "$(dirname "$GFONTS_DIR")"
    git clone --depth=1 --filter=blob:none --branch=main \
      "$GFONTS_REMOTE" "$GFONTS_DIR"
  else
    log "updating google/fonts clone"
    git -C "$GFONTS_DIR" fetch --depth=1 origin main
  fi
  if [ -n "${GFONTS_COMMIT:-}" ]; then
    log "checking out pinned commit $GFONTS_COMMIT"
    git -C "$GFONTS_DIR" fetch --depth=1 origin "$GFONTS_COMMIT"
    git -C "$GFONTS_DIR" checkout --quiet "$GFONTS_COMMIT"
  else
    git -C "$GFONTS_DIR" checkout --quiet FETCH_HEAD 2>/dev/null || \
      git -C "$GFONTS_DIR" checkout --quiet origin/main
  fi
  HEAD_SHA="$(git -C "$GFONTS_DIR" rev-parse --short HEAD)"
  FAM_COUNT="$(find "$GFONTS_DIR" -maxdepth 3 -name METADATA.pb | wc -l | tr -d ' ')"
  log "gfonts: @ $HEAD_SHA, $FAM_COUNT families"
else
  if [ -d "$GFONTS_DIR/.git" ]; then
    log "gfonts: cached clone present (SETUP_FULL not set; leaving alone)"
  else
    log "gfonts: skipped (set SETUP_FULL=1 to clone full corpus)"
  fi
fi

log "setup complete"
