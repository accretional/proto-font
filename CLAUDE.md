# CLAUDE.md

See `AGENTS.md` — same rules apply. This file is specifically for Claude Code
sessions.

## Quick reference

- Go toolchain: `go1.26` (box has `go1.26.2`).
- Build/test ONLY via `./setup.sh`, `./build.sh`, `./test.sh`,
  `./LET_IT_RIP.sh`. Each wraps the previous. All idempotent.
- Never run `go test ./...` or `go build ./...` directly for CI-style
  validation — use the scripts.
- Before committing or pushing: `./LET_IT_RIP.sh` must pass.
- `./LET_IT_RIP.sh` also re-renders e2e screenshots into `screenshots/`
  (top-level dir). It auto-builds and starts headless `chromerpc` from
  `../chromerpc` if needed. Skip with `SKIP_SCREENSHOTS=1`. After the run,
  `git diff -- screenshots/` and visually open every changed PNG — diffs
  are expected when you add a fixture or change a codec/render path, but
  you must confirm the rendering still looks right before pushing.
- WOFF2 decode landed (header + variable-length directory + brotli
  decompress + per-table data on `Woff2TableDirectoryEntry`). `glyf`/`loca`
  remain in transformed form — see `transformed`/`transform_length` and
  the `## NEXT STEPS` bullet.

## Proto

- Local sources live in `proto/openformat/v1/`, split by subsystem.
  `container.proto` wraps everything; `sfnt_table.proto` owns the
  `SfntTable` parsed oneof; `tables_core.proto` holds fully-decomposed
  tables (head/hhea/maxp/OS_2/post/name/cmap/hmtx/loca);
  `tables_{glyphs,layout,variations,color,metrics,meta}.proto` are
  **stubs reserved for the gluon parser project** (sibling repo). Stub
  messages are intentionally empty — bytes live in `SfntTable.raw_data`
  until gluon lands structured parsers. Regenerate with `./setup.sh`.
- `mime.proto` is unchanged (MimeType wrapper).
- Vendored googlefonts protos live in `proto/googlefonts/v1/` and are
  pulled by `setup.sh` from the upstream GitHub repos (fonts_public,
  axes, languages_public, designers, knowledge). They use `proto2`
  syntax — do not edit them; update via re-running `setup.sh`. Each
  vendored proto gets its own Go sub-package
  (`gen/go/googlefonts/<base>/`) because upstream has message-name
  collisions across files (e.g. `SampleTextProto`).
- Generated Go for authored schema lives in `gen/go/openformat/v1/`,
  all within a single Go package `openformatv1` — the `.proto` split
  is purely for authoring clarity.
- Proto `go_package` option is
  `"openformat/gen/go/openformat/v1;openformatv1"`, so
  `protoc --go_opt=module=openformat` resolves paths inside this
  module.
- Accretional sibling repos live at `../` on this machine (e.g.
  `../chromerpc`, `../mime-proto`) — handy when integrating rather
  than fetching over HTTPS.

## Font format scope

- SFNT container (TrueType + OpenType) is modelled in full: offset table,
  table directory, and a per-table payload that keeps `tag`, `checksum`,
  declared `offset` + `length`, and `raw_data`. Structured sub-messages are
  provided for the *integrity-critical* tables (`head`, `hhea`, `maxp`,
  `name`, `OS/2`, `post`, `hmtx`, `cmap` encoding records, `loca` offsets).
  Everything else rides as opaque bytes — this keeps round-trip byte-for-byte
  while still exposing enough structure for the typical use cases.
- WOFF 1.0 container is modelled (header + per-table compressed blocks +
  metadata XML + private block). WOFF 2.0 is now modelled too (header +
  255UInt16 directory + brotli-decompressed per-table bytes); the
  `glyf`/`loca` transform reversal is the remaining gap — see
  `README.md` `## NEXT STEPS`.
- Collections (`.ttc`) are modelled as a top-level `FontCollection` wrapping
  repeated `SfntFont`.

## Code layout

- `internal/fontcodec/` — encoder (proto → font bytes) and decoder (font
  bytes → proto). `FontFileWithMetadata.raw_bytes` is always set by Decode
  and required by Encode for round-trip fidelity.
- `internal/gfapi/` — tiny Go client for `https://www.googleapis.com/webfonts/v1/webfonts`.
  Exposes `ListWebfonts(ctx, key, opts)` and returns strongly typed
  `WebfontList` structs.
- `data/fonts/` — font fixtures. Hand-picked fixtures committed under
  `handwritten/`; Noto subset fetched at `setup.sh` time under `noto/`.
- `testing/validation/` — walks every file in `data/fonts` (skipping
  the opt-in `gfonts/` corpus by default), decodes, re-encodes, diffs
  bytes. `FULL_CORPUS=1` exercises the full google/fonts clone.
- `testing/fuzz/` — `go test -fuzz` targets over the decoder.
- `testing/benchmarks/` — `Benchmark*` functions run across `data/fonts`.
  Also gated by `FULL_CORPUS=1` for the big sweep.
- `ui-e2e-validation/` — generates per-font HTML samples + chromerpc
  automation textprotos; opt-in `UI_E2E=1` + a running chromerpc server
  drives headless Chrome screenshots.

## Documentation outputs

- `README.md` `## NEXT STEPS` — append findings (format quirks, missing
  features, bugs in upstream proto, WOFF2 gap).
- `testing/README.md` — overall test strategy + any discrepancies.
- `docs/googlefaq-tldr.md` — TL;DR of
  https://developers.google.com/fonts/faq.
- `docs/gf-guide-tldr.md` — TL;DR of https://googlefonts.github.io/gf-guide/.
- `docs/googlefonts-repos-tldr/` — one file per repo under the `googlefonts`
  GitHub org that is worth summarising. Only add a repo if it is actively
  relevant (do not mass-import random repos).

## Gotchas

- SFNT table directory entries are sorted by tag; Encode MUST emit them in
  that order regardless of how they appear in the proto, because the head
  table checksum depends on it.
- `head.checkSumAdjustment` is computed from the whole file; the decoder
  records the observed value but the encoder recomputes it, then — if
  round-trip parity is needed — patches the field back to the original.
  `raw_bytes` is the safety net when a file uses quirky padding that we
  don't perfectly reproduce.
- WOFF tables are individually zlib-compressed ONLY when compressed length
  is smaller than the uncompressed length. Otherwise they are stored raw.
  The encoder must mirror the decoder's original choice (we record
  `was_compressed` per table).
- `setup.sh` fetches googlefonts protos over HTTPS. When offline it falls
  back to skipping the refresh and using whatever is already vendored.
