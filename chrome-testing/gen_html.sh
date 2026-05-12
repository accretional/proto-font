#!/usr/bin/env bash
# gen_html.sh — generate per-font HTML sample pages for chrome-testing.
#
# Walks data/fonts/ (skipping gfonts/), copies each supported font into
# chrome-testing/html/data/fonts/, and writes one HTML file per font into
# chrome-testing/html/.
#
# Usage (from the project root or from chrome-testing/):
#   bash chrome-testing/gen_html.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
FONTS_ROOT="$PROJECT_ROOT/data/fonts"
HTML_DIR="$SCRIPT_DIR/html"
FONTS_DEST="$HTML_DIR/data/fonts"

mkdir -p "$HTML_DIR" "$FONTS_DEST"

# ── Collect supported font paths ──────────────────────────────────────────────

fonts=()
while IFS= read -r f; do
  fonts+=("$f")
done < <(find "$FONTS_ROOT" -type f \
  \( -name "*.ttf" -o -name "*.otf" -o -name "*.woff" -o -name "*.woff2" \) \
  -not -path "*/gfonts/*" | sort)

if [ ${#fonts[@]} -eq 0 ]; then
  echo "No supported font files found under $FONTS_ROOT" >&2
  exit 1
fi

# ── Find duplicate stems (for collision-aware slugs) ──────────────────────────

dup_stems=$(for f in "${fonts[@]}"; do
  base=$(basename "$f"); echo "${base%.*}"
done | sort | uniq -d)

# ── Helpers ───────────────────────────────────────────────────────────────────

slugify() { echo "$1" | sed 's/[^a-zA-Z0-9_-]/_/g'; }

is_dup() { echo "$dup_stems" | grep -qx "$1"; }

# ── Generate ──────────────────────────────────────────────────────────────────

count=0
for font in "${fonts[@]}"; do
  ext="${font##*.}"
  case "$ext" in
    ttf)   fmt="truetype" ;;
    otf)   fmt="opentype" ;;
    woff)  fmt="woff"     ;;
    woff2) fmt="woff2"    ;;
    *)     continue        ;;
  esac

  rel_path="${font#$FONTS_ROOT/}"
  base=$(basename "$font")
  stem="${base%.*}"
  slug=$(slugify "$stem")

  if is_dup "$stem"; then
    slug="${slug}_${fmt}"
  fi

  # Copy font preserving sub-path.
  dest="$FONTS_DEST/$rel_path"
  mkdir -p "$(dirname "$dest")"
  cp "$font" "$dest"

  # Variable font badge.
  case "$stem" in
    *\[*\]*) variable_badge='<span class="badge">variable</span>' ;;
    *)       variable_badge='' ;;
  esac

  # Write HTML sample.
  cat > "$HTML_DIR/${slug}.html" << HTML
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>${stem}</title>
<style>
  @font-face {
    font-family: "Preview";
    src: url("data/fonts/${rel_path}") format("${fmt}");
    font-display: block;
  }
  *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
  html { background: #0e0e10; color: #e8e8ec; font-family: "Preview", system-ui, sans-serif; }
  body { padding: 40px 48px 64px; max-width: 1100px; }
  .meta {
    font-family: ui-monospace, "SFMono-Regular", Menlo, monospace;
    font-size: 11px; color: #66676e; letter-spacing: 0.06em;
    text-transform: uppercase; margin-bottom: 36px;
    padding-bottom: 14px; border-bottom: 1px solid #222228;
  }
  .meta span { color: #9a9ba5; }
  .waterfall { margin-bottom: 40px; }
  .wf-row { font-family: "Preview"; line-height: 1.08; padding: 6px 0;
             white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .wf-96  { font-size: 96px; }
  .wf-64  { font-size: 64px; }
  .wf-48  { font-size: 48px; }
  .wf-32  { font-size: 32px; }
  .wf-24  { font-size: 24px; line-height: 1.2; }
  .divider { border: none; border-top: 1px solid #1c1c22; margin: 32px 0; }
  .alphabet { font-family: "Preview"; font-size: 40px; line-height: 1.3;
               letter-spacing: 0.04em; color: #c8c8d2; }
  .digits { font-family: "Preview"; font-size: 36px; letter-spacing: 0.08em;
             font-variant-numeric: tabular-nums; color: #8a8b95; margin-top: 16px; }
  .prose { font-family: "Preview"; font-size: 18px; line-height: 1.65;
            max-width: 700px; color: #b0b0bc; margin-top: 28px; }
  .badge { display: inline-block; font-family: ui-monospace, monospace;
           font-size: 10px; background: #1e1e26; color: #7c7d87;
           border: 1px solid #2a2a34; border-radius: 4px; padding: 2px 7px;
           vertical-align: middle; margin-left: 6px; letter-spacing: 0.05em; }
</style>
</head>
<body>
  <div class="meta">
    proto-font / chrome-testing &nbsp;&bull;&nbsp;
    <span>${rel_path}</span> &nbsp;&bull;&nbsp;
    format: <span>${fmt}</span>
    ${variable_badge}
  </div>
  <div class="waterfall">
    <div class="wf-row wf-96">Hamburgefonstiv</div>
    <div class="wf-row wf-64">The quick brown fox</div>
    <div class="wf-row wf-48">jumps over the lazy dog.</div>
    <div class="wf-row wf-32">Pack my box with five dozen liquor jugs.</div>
    <div class="wf-row wf-24">Sphinx of black quartz, judge my vow.</div>
  </div>
  <hr class="divider">
  <div class="alphabet">ABCDEFGHIJKLMNOPQRSTUVWXYZ</div>
  <div class="alphabet">abcdefghijklmnopqrstuvwxyz</div>
  <div class="digits">0123456789 &middot; !?@#\$%&amp;*()[]{}</div>
  <hr class="divider">
  <div class="prose">
    &ldquo;Typography is the craft of endowing human language with a durable
    visual form.&rdquo; &mdash; Robert Bringhurst
  </div>
</body>
</html>
HTML

  printf "  %-50s  →  %s\n" "$rel_path" "${slug}.html"
  (( count++ )) || true
done

echo ""
echo "${count} HTML file(s) written to ${HTML_DIR}/"
