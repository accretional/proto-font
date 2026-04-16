# proto-font

## Instructions

Make sure you create a setup.sh, build.sh, test.sh, and LET_IT_RIP.sh that contain all project setup scripts/commands used - NEVER build/test/run the code in this repo outside of these scripts, NEVER commit or push without running these either. Make them idempotent so that each build.sh can run setup.sh and skip things already set up, each test.sh can run build.sh, each LET_IT_RIP runs test.sh

use go1.26

Encode the latest versions of the OpenType, TrueType, and woff font formats into protobuf messages, similarly to how we did it in this project: https://github.com/accretional/mime-proto/blob/main/pb/proto/openformat/v1/docx.proto

try to use protos you find in https://github.com/google/fonts if they are related to the font formats or Noto, we are going to set upa. validation/test set with all the noto fonts https://github.com/google/fonts/blob/b669b896a75927719f611ac76f329bbeab32dc61/lang/Lib/gflanguages/languages_public.proto https://github.com/google/fonts/blob/b669b896a75927719f611ac76f329bbeab32dc61/axisregistry/Lib/axisregistry/axes.proto

here's some stuff from other repos

<details><summary>googlefonts files with .proto, mangled formatting, try github googlefonts/gftools or similar</summary>
17 files  (568 ms)
17 files
in
googlefonts (press backspace or delete to remove)
Files with identical content are grouped together.
googlefonts/gftools · Lib/gftools/axes.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
// GF Axis Registry Protos
// An axis in the GF Axis Registry
message AxisProto {
  // Axis tag
  optional string tag = 1;
googlefonts/PFE-analysis · analysis/result.proto

    Protocol Buffer
    ·
    0 (0)

// Proto definition of used to store the results of
// the analysis.
syntax = "proto3";
package analysis;
message AnalysisResultProto {
googlefonts/gftools · Lib/gftools/designers.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
// GF Designer Profile Protos
// A designer listed on the catalog:
message DesignerInfoProto {
  // Designer or typefoundry name:
  optional string designer = 1;
googlefonts/gf-metadata · resources/protos/designers.proto

googlefonts/gf-metadata · resources/scripts/embed_data.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
message FloatVecProto {
    repeated float value = 1;
}
message MetadataProto {
googlefonts/PFE-analysis · analysis/pfe_methods/unicode_range_data/slicing_strategy.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto3";
package analysis.pfe_methods.unicode_range_data;
message SlicingStrategy {
  repeated Subset subsets = 1;
}
googlefonts/gftools · Lib/gftools/fonts_public.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
/**
 * Open Source'd font metadata proto formats.
 */
package google.fonts_public;
googlefonts/gf-metadata · resources/protos/fonts_public.proto

googlefonts/lang · Lib/gflanguages/languages_public.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
/**
 * languages/regions/scripts proto formats.
 */
package google.languages_public;
googlefonts/gf-metadata · resources/protos/languages_public.proto

googlefonts/FontClassificationTool · fonts_public.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
/**
 * Open Source'd font metadata proto formats.
 */
package google.fonts;
googlefonts/gf-metadata · resources/protos/axes.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
// GF Axis Registry Protos
// An axis in the GF Axis Registry
message AxisProto {
  // Axis tag
  optional string tag = 1;
googlefonts/PFE-analysis · analysis/page_view_sequence.proto

    Protocol Buffer
    ·
    0 (0)

// Proto format used by the open source incxfer analysis code.
syntax = "proto3";
package analysis;
message PageContentProto {
  string font_name = 1;
googlefonts/gftools · Lib/gftools/knowledge.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto2";
/**
 * Proto definitions for Fonts Knowledge metadata in the filesystem.
 */
package fonts;
googlefonts/gf-metadata · resources/protos/knowledge.proto

googlefonts/fontbakery-dashboardArchived · containers/base/protocolbuffers/shared.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto3";
package fontbakery.dashboard;
message File {
  string name = 1;
  bytes data = 2;
googlefonts/fontbakery-dashboardArchived · containers/base/protocolbuffers/messages.proto

    Protocol Buffer
    ·
    0 (0)

syntax = "proto3";
import "google/protobuf/any.proto";
import "google/protobuf/timestamp.proto";
import "google/protobuf/empty.proto";
import public "shared.proto";

</details>

do the same in https://github.com/googlefonts/axisregistry

get the fonts out of https://github.com/google/material-design-icons

document and build a client for the google fonts api with documentation at https://developers.google.com/fonts/docs/developer_api#api_url_specification 

integrate https://github.com/googlefonts/lang

Do this

```
you can download all Google Fonts in a simple ZIP snapshot (over 1GB) from https://github.com/google/fonts/archive/main.zip
Sync With Git

You can also sync the collection with git so that you can update by only fetching what has changed. To learn how to use git, GitHub provides illustrated guides, a youtube channel, and an interactive learning site. Free, open-source git applications are available for Windows and Mac OS X.
```

Go through https://developers.google.com/fonts/faq and document anything interesting in docs/googlefaq-tldr.md

do the same in https://googlefonts.github.io/gf-guide/ document it in docs/gf-guide-tldr.md

Do the same with https://github.com/orgs/googlefonts/repositories, don't go crazy importing random fonts there tho, docs/googlefonts-repos-tldr/

do 

## Project layout

See `AGENTS.md` / `CLAUDE.md` for the ground rules. Quick map:

- `proto/openformat/v1/` — authored proto sources, one file per
  subsystem (`container`, `sfnt_table`, `tables_core`, plus stub
  files for the table groups still waiting on gluon).
- `proto/googlefonts/v1/` — vendored upstream protos (gftools, lang).
- `gen/go/` — generated Go; do not hand-edit. One Go package per
  proto area.
- `internal/fontcodec/` — Decode/Encode for SFNT, WOFF1, WOFF2 (header-only),
  TTC, EOT.
- `internal/metadata/` — parses google/fonts `METADATA.pb` text-protos.
- `internal/gfapi/` — Google Fonts developer-API client. See `docs/gfapi.md`.
- `data/fonts/` — fixtures; `noto/` is fetched by `setup.sh`,
  `gfonts/` is the opt-in full corpus, `handwritten/` is checked in.
- `testing/` — validation, fuzz, benchmarks.
- `ui-e2e-validation/` — generates per-font HTML samples + chromerpc
  automation textprotos; drives headless Chrome under `UI_E2E=1`.
- `docs/` — TL;DRs for the FAQ, contributor guide, per-repo notes.

## Build / test

Do everything through the scripts:

```
./setup.sh       # install deps, vendor protos, generate Go, fetch fixtures
./build.sh       # setup + go vet + go build
./test.sh        # build + unit/validation/fuzz smoke/bench
./LET_IT_RIP.sh  # test. ship gate.
```

### Full Google Fonts corpus (opt-in)

The default setup pulls ~3 Noto TTFs + 7 `METADATA.pb` files (seconds).
Set `SETUP_FULL=1 ./setup.sh` to shallow-clone the entire `google/fonts`
repo into `data/fonts/gfonts/` (~1 GB, uses `--filter=blob:none`).
Re-runs fetch incrementally. Pin to a commit via `GFONTS_COMMIT=<sha>`.

Once the corpus is on disk, run the sweeps with `FULL_CORPUS=1`:

```
FULL_CORPUS=1 go test ./testing/validation/...  -run FullCorpus
FULL_CORPUS=1 go test ./internal/metadata/...   -run FullCorpus
```

The default `./test.sh` skips `data/fonts/gfonts/` so CI stays fast.

## NEXT STEPS

Findings and known gaps surfaced while building the codec / schema.
Append as new work comes up.

- **WOFF2 decode**: only the fixed-length header is parsed. Per-table
  directory (255UInt16 encoding) and transform reversal (`glyf`/`loca`)
  require a brotli decoder. Round-trip works today via `raw_bytes`. To
  finish: pull `github.com/andybalholm/brotli`, decode the compressed
  stream, walk the directory, reverse the glyf transform.
- **WOFF2 encode from structured fields**: blocked on the decode work —
  we can't compress what we haven't decomposed.
- **TTC synthesis**: re-emitting a `.ttc` without `raw_bytes` means
  laying out shared table bodies across fonts. Not implemented; `Encode`
  errors out.
- **EOT synthesis**: same story as TTC — round-trip via `raw_bytes` only.
- **cmap subtable parsing**: we expose the directory but keep subtable
  bodies as opaque bytes. Adding structured parsers for format 4, 6, 10,
  12, 13, 14 would let callers read character coverage without hauling
  the raw table out.
- **`head.checkSumAdjustment` recompute**: synthesis path copies the
  declared value. For a truly synthetic font we should recompute per
  OpenType §5.head (zero the field, checksum the whole file, subtract
  from 0xB1B0AFBA).
- **Structured parsers for currently-stubbed tables**: the proto
  schema now reserves typed messages for `glyf`, `CFF`, `CFF2`, `GSUB`,
  `GPOS`, `GDEF`, `BASE`, `JSTF`, `MATH`, `fvar`, `avar`, `STAT`,
  `HVAR`, `VVAR`, `MVAR`, `gvar`, `COLR`, `CPAL`, `sbix`, `CBDT`,
  `CBLC`, `SVG`, `EBDT`, `EBLC`, `EBSC`, `vhea`, `vmtx`, `kern`,
  `hdmx`, `LTSH`, `VDMX`, `gasp`, `PCLT`, `meta`, `DSIG`, `MERG`,
  `VORG`, `cvt`, `fpgm`, `prep`. The messages are intentionally empty
  — bytes still live in `SfntTable.raw_data` — and the **gluon**
  parser project will fill them in as parsers land. Adding fields is
  backward-compatible.
- **`METADATA.pb` ingestion** *(done — see `METADATA-IMPORT-LOG.md`)*:
  `internal/metadata` parses + validates the text-protos from
  `google/fonts`. Follow-up: cross-check `fonts[*].filename` against the
  actual binaries in the same family directory, ingest the sibling
  `DESCRIPTION.en_us.html`, and read the `axisregistry` per-axis
  `.textproto` records.
- **Material Design Icons ingestion**: README asks to "get the fonts out
  of https://github.com/google/material-design-icons". `setup.sh` does
  not fetch them today — add a fixture pull if we want to validate
  Material Symbols TTFs specifically (they stress variable-axis + colour
  tables).
- **`lang` data files**: we vendor the schema, not the `.textproto` data.
  If `gfapi` gains a "fonts covering language X" helper we need to pull
  the data.
- **Handwritten fixtures**: `data/fonts/handwritten/` is empty. Candidates:
  a TTC from `font-test-data`, a WOFF with metadata-XML, an EOT.
- **Font Bakery parity**: we don't run Font Bakery in CI; the validation
  suite only checks byte-exact round-trip + `head.magicNumber`. Wiring in
  `fontbakery check-googlefonts` on the Noto fixtures would catch codec
  regressions that re-emit "valid-but-Different" bytes.

