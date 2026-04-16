# googlefonts/gftools

<https://github.com/googlefonts/gftools>

Python grab-bag of tooling. The most load-bearing pieces for proto-font:

- **`Lib/gftools/axes.proto`** — schema for the Axis Registry (tag, display
  name, min/default/max, fallbacks). We vendor this directly.
- **`Lib/gftools/fonts_public.proto`** — schema for `METADATA.pb` inside each
  `ofl/<family>/` directory on google/fonts. Required reading before touching
  our own `FontProvenance` / family-level metadata modelling.
- **`Lib/gftools/designers.proto`** — designer profile schema.
- **`Lib/gftools/knowledge.proto`** — Fonts Knowledge article metadata.
- **CLI `gftools fix-*`** — `fix-vertical-metrics`, `fix-nameids`,
  `fix-fstype`, `fix-gasp`, etc. These rewrite specific table fields. Our
  `OS2Table`, `NameTable`, `PostTable` protos must expose every field these
  tools touch so the round-trip is lossless.
- **CLI `gftools packager`** — bundles a family for a `google/fonts` PR.
  Produces the exact layout of binaries + METADATA.pb + article that our
  tooling should accept as input.

## Pitfalls

- `fonts_public.proto` is proto2; some fields are `required`. When consuming
  `METADATA.pb` we must tolerate missing required fields from hand-edited
  files.
- `axes.proto` uses top-level messages without a proto package. Our
  setup.sh re-writes `go_package` to keep it namespaced.
