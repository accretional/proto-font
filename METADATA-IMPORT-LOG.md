# METADATA.pb Ingestion Log

Captures the work of teaching this repo to read the `METADATA.pb`
text-protos that live next to every family in
[google/fonts](https://github.com/google/fonts). Written so the next
pass — DESCRIPTION ingestion, axis-registry text-proto reads, font ↔
metadata cross-validation — has the context it needs.

## Goal

Take a `METADATA.pb` byte blob, decode it into a typed Go value, run a
small structural validator, and do this for every family directory in
the upstream repo without choking on schema drift.

## Approach

- Vendor the upstream schema (`gftools/Lib/gftools/fonts_public.proto`)
  via `setup.sh` — generated into `gen/go/googlefonts/fonts_public/`.
- `internal/metadata/metadata.go` wraps it with four entry points:
  `Parse`, `ReadFile`, `ReadFS`, `Validate`.
- Use `google.golang.org/protobuf/encoding/prototext` rather than JSON
  — these files are protobuf text format (not JSON, not binary).
- Set `prototext.UnmarshalOptions{DiscardUnknown: true}` so unknown
  fields don't fail the parse. Real-world `METADATA.pb` regularly
  carries fields ahead of the .proto schema being updated.
- Layer a structural `Validate(*FamilyProto) []string` on top, since
  proto3/prototext happily decodes absent `required` fields and we
  still want to flag them.

## Fixture acquisition

The full `google/fonts` checkout is over 1 GB; we cherry-pick. Seven
families chosen to cover the variation surface:

| Family             | Path                       | Why                                  |
| ------------------ | -------------------------- | ------------------------------------ |
| `notosans`         | `ofl/notosans`             | Variable axes, `is_noto: true`       |
| `robotoflex`       | `ofl/robotoflex`           | Many axes, lots of optional fields   |
| `inter`            | `ofl/inter`                | Popular variable family              |
| `jetbrainsmono`    | `ofl/jetbrainsmono`        | Monospace category                   |
| `roboto`           | `ofl/roboto`               | Recently relicensed (was `apache/`)  |
| `abeezee`          | `ofl/abeezee`              | Static, single style                 |
| `pacifico`         | `ofl/pacifico`             | Handwriting / display category       |

`setup.sh` writes them into `data/metadata/<family>.METADATA.pb` (flat
fixture layout, not the nested upstream tree).

### 404s and gotchas during fetch

- **`apache/roboto/METADATA.pb` → 404.** Roboto was relicensed and
  moved to `ofl/roboto`. Updated the URL.
- **`apache/materialsymbolsoutlined/METADATA.pb` → 404.** Material
  Symbols isn't in `google/fonts`; it ships from
  `google/material-design-icons`. Dropped from the fixture set; it'll
  reappear when we tackle Material icons separately.
- **`googlefonts/lang` `languages_public.proto` and gftools
  `fonts_public.proto` both define `SampleTextProto`.** Generating
  them into the same Go package collided. Resolved by giving each
  upstream proto its own Go package via a per-file
  `option go_package = "openformat/gen/go/googlefonts/<base>;gf<short>"`,
  rewritten by `normalize_go_package` in setup.sh.

## Schema observations

Things worth knowing the next time someone reads `fonts_public.proto`
without context:

- **`syntax = "proto2"`** — `required` is meaningful in the schema
  but `prototext` will still decode files missing those fields
  without erroring. Hence the explicit `Validate`.
- **`category` is a `repeated string`**, not an enum, and the schema
  comment says only the LAST value is consumed. Treat it as
  effectively scalar but defensive code should walk the slice.
- **Inline `# ...` comments are valid** in the text proto. The
  `TestParseInline` fixture exercises this.
- **`date_added` is a free-form string.** Real-world files use
  `YYYY-MM-DD`; we enforce that shape in `Validate` rather than
  trusting the schema.
- **Per-family files reference sibling binaries** via
  `fonts[i].filename` (e.g. `NotoSans-Regular.ttf`). Cross-validation
  against the directory contents is a follow-up — we currently only
  check that `filename` is non-empty.
- **`AxisSegmentProto` reserves field 3 (`default_value`)** — it
  used to exist, was removed, and isn't coming back. Don't add a
  Go-side accessor for it.

## ReadFS dual-path filename handling

The walker accepts two layouts:

1. The real upstream tree: `<root>/ofl/notosans/METADATA.pb`. Every
   leaf is literally named `METADATA.pb`.
2. Our flat fixture layout: `<root>/notosans.METADATA.pb`. Lets us
   keep many families in one directory without a deep tree.

The discriminator in `metadata.go`:

```go
base := filepath.Base(path)
if base != "METADATA.pb" && !strings.HasSuffix(base, ".METADATA.pb") {
    return nil
}
```

First commit only checked the exact `"METADATA.pb"` form, which made
`TestReadFS` silently skip every fixture. Caught when running
`-v` and seeing zero subtests under the table-driven walker.

## Validation rationale

`Validate` enforces the bare minimum a downstream consumer needs:

- `name`, `designer`, `license` non-empty (the `required` triad).
- Every `fonts[i]` has a `filename` (otherwise we can't locate the
  binary).
- `date_added`, when present, parses as `YYYY-MM-DD`.

Deliberately NOT in scope: Font Bakery-style checks (license string
matches a known SPDX, weights are canonical, axis ranges sane). Those
belong in a separate suite that can grow without coupling to the
parser.

## Test surface

| Test                          | What it proves                                                                |
| ----------------------------- | ----------------------------------------------------------------------------- |
| `TestParseInline`             | Hand-crafted text proto with `#` comments parses & validates                  |
| `TestValidateCatchesMissing`  | Empty required fields and bad date are flagged                                |
| `TestRealWorldFixtures`       | All 7 upstream fixtures parse with no validation issues                       |
| `TestReadFS`                  | Walker returns every `*.METADATA.pb` with the relative path as the map key   |

Real-world test was the most informative — every fixture passed on
first run, which suggests either the schema is well-maintained or our
validator is too lax. Probably both.

## Forward work

In rough priority order:

1. **Cross-check `fonts[*].filename` against actual files** in the
   sibling directory once we ingest a full family checkout (not just
   the metadata blob). Today nothing catches a typo'd filename.
2. **Ingest `DESCRIPTION.en_us.html`** — short HTML blurb that lives
   beside `METADATA.pb`. Useful for the corpus surface even if we
   don't render it.
3. **Read `axisregistry` text protos**
   (`googlefonts/axisregistry/Lib/axisregistry/data/*.textproto`)
   using the same prototext + per-axis-record approach. Schema is
   already vendored at `proto/googlefonts/v1/axes.proto`.
4. **Reconcile `category` vs the new `classifications`/`stroke`
   fields** — the schema is mid-migration; `Validate` should
   eventually warn when both are set and disagree.
5. **Surface `is_noto`, `subsets`, `languages`, `primary_script`** at
   query time so a corpus consumer can filter without re-decoding the
   whole `FamilyProto`.

## Process notes for future me

- Keep fixture additions in `setup.sh` ordered & commented; the file
  is the contract for "what does our test corpus look like."
- When `prototext.Unmarshal` fails, the error string usually
  pinpoints the line/column in the source. No need to reach for a
  custom diagnostic.
- The vendored `.proto` files get rewritten on every `setup.sh` run
  by `normalize_go_package`. Don't hand-edit the `option go_package`
  line — it'll bounce on the next setup.
