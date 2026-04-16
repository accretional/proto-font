# AGENTS.md

Ground rules for agents (and humans) working in this repo. These are distilled
from `README.md` — read that file for the source of truth.

## Cardinal rule: use the scripts

All build/test/run activity MUST go through these four scripts. Never invoke
`go build`, `go test`, `protoc`, etc. directly from the CLI in the normal
workflow.

- `setup.sh` — install toolchain, fetch deps, generate code from `.proto`, pull
  any external assets (e.g. Noto fixtures, googlefonts proto sources).
  Idempotent.
- `build.sh` — runs `setup.sh`, then compiles everything.
- `test.sh` — runs `build.sh`, then every test (unit, validation across font
  fixtures, fuzz smoke, benchmarks).
- `LET_IT_RIP.sh` — runs `test.sh`. Top-level "ship it" entry point.

Each script is a superset of the previous:
`LET_IT_RIP → test → build → setup`. Do NOT commit or push without running
`LET_IT_RIP.sh` successfully.

## Toolchain

- Go 1.26 (`go1.26`). Dev box has `go1.26.2`.
- `protoc` + `protoc-gen-go` for codegen.

## Layout

```
proto/openformat/v1/        vendored .proto sources (font.proto + mime.proto)
proto/googlefonts/v1/       vendored googlefonts protos (axes, languages, fonts_public, designers, knowledge)
gen/go/openformat/v1/       generated Go (package openformatv1)
gen/go/googlefonts/v1/      generated Go (package googlefontsv1)
internal/fontcodec/         encoder + decoder for SFNT (TTF/OTF) and WOFF containers
internal/gfapi/             Google Fonts developer-API client
data/fonts/                 font test fixtures (small hand-picked + fetched Noto)
testing/validation/         validation test suite across all data/fonts
testing/fuzz/               fuzz tests
testing/benchmarks/         benchmarks
testing/README.md           strategy + discrepancies
docs/googlefaq-tldr.md      TL;DR of https://developers.google.com/fonts/faq
docs/gf-guide-tldr.md       TL;DR of https://googlefonts.github.io/gf-guide/
docs/googlefonts-repos-tldr/ per-repo notes from github.com/orgs/googlefonts
```

## Proto source

The font proto is authored in this repo (mirroring the approach in
`github.com/accretional/mime-proto`'s `pb/proto/openformat/v1/docx.proto`).
It reuses `openformat/v1/mime.proto` for `MimeType`. The googlefonts protos
are vendored verbatim from upstream and regenerated into
`gen/go/googlefonts/v1/`.

`font.proto` declares
`option go_package = "openformat/gen/go/openformat/v1;openformatv1"`, matching
the pattern used by `xml.proto` / `mime.proto` in the sibling `proto-xml` repo.
Do not hand-edit generated code under `gen/`.

## Round-trip fidelity

`FontFileWithMetadata.raw_bytes` is always populated by `Decode` and required
by `Encode`. Opaque tables (e.g. glyf CFF hinting programs) are carried as
`bytes` inside the structured message so round-trips are byte-exact even when
we don't parse the inner table. See `internal/fontcodec/` for details.

## README responsibilities not yet automated

- `README.md` has a `## NEXT STEPS` section where agents record format
  irregularities, missing functionality, and other findings surfaced during
  implementation/testing.
- `testing/README.md` must document the overall test strategy and any
  discrepancies / irregularities.
- `docs/googlefaq-tldr.md`, `docs/gf-guide-tldr.md`, and
  `docs/googlefonts-repos-tldr/` must be populated as those upstream sources
  are reviewed.

## Style

- Keep comments minimal; use them only when the *why* is non-obvious.
- Prefer editing existing files over creating new ones.
- Do not introduce abstractions beyond what a task needs.

## Test fixture conventions

- `data/fonts/handwritten/NN_<aspect>.<ext>` — small, committed fonts that
  exercise a specific format corner (e.g. `01_truetype.ttf`, `02_woff1.woff`).
  Numbered prefix groups related files.
- `data/fonts/noto/` — populated by `setup.sh` via sparse-checkout of
  `github.com/notofonts/notofonts.github.io` (or a small curated subset).
  Never committed; gitignored.
- Any `.ttf` / `.otf` / `.woff` / `.woff2` file under `data/fonts/` is picked
  up automatically by validation, fuzz seed corpus, and benchmarks.

## Reporting findings

Anything surprising about a font format, an upstream proto, or the codec
implementation belongs in `README.md` `## NEXT STEPS`. Test-strategy
specifics (deliberate scope cuts, parser quirks) belong in
`testing/README.md` under "Known discrepancies and limitations".
