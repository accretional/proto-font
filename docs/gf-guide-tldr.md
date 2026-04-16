# Google Fonts contributor guide — TL;DR

Source: <https://googlefonts.github.io/gf-guide/>

A contributor-side workflow guide. Useful to us because the proto schema and
codec need to survive files produced by this pipeline.

## Section map

1. **Introduction: getting familiar with the basics**
   Glyphs/FontLab/UFO, Python 3.x, `gftools`, `fontmake`, and Font Bakery.
   New contributors start here.
2. **The upstream repo**
   Canonical project layout for a family's *source* repo:
   `sources/`, `fonts/`, `tests/`, `OFL.txt`, `FONTLOG.txt`, `README.md`,
   version tags, release zips.
3. **Pre-production: getting your fonts ready**
   Mastering conventions for static, variable, and colour fonts; vertical
   metrics (`OS/2.sTypoAscender` etc.), language shaping tests, QA gates.
4. **Production: compiling your fonts**
   Building with `fontmake`, post-processing (`gftools fix-*`), QA with
   Font Bakery, Diffenator, shaperglot.
5. **The `google/fonts` repository**
   Per-family subdir under `ofl/<family>/` with `METADATA.pb`,
   `DESCRIPTION.en_us.html`, `article/ARTICLE.en_us.html`, and the packaged
   binaries.
6. **Onboarding fonts to Google Fonts**
   PR workflow, packaging via `gftools packager`, reviewer checklist,
   promotion / social-media material.

## Labels

Each page carries one of: `start`, `must→`, `learn`, `nerd`, `team`, `templ`.
Contributors can skim by label — we mostly want pages marked `nerd` or
`must→`, which document invariants our codec/proto has to respect.

## Things load-bearing for proto-font

- **`METADATA.pb`** in each family directory is a text-proto that matches
  `fonts_public.proto` (our vendored copy in
  `proto/googlefonts/v1/fonts_public.proto`). This is the schema upstream
  consumers will round-trip through.
- **`gftools fix-*`** normalises vertical metrics, name records, and PANOSE.
  Our `OS2Table` + `NameTable` protos expose the fields these tools
  manipulate.
- **`axisregistry`** drives default axis ranges; its canonical proto is
  `gftools/Lib/gftools/axes.proto` (we vendor it).
- **Font Bakery** enforces license-record boilerplate and checksums. Our
  tests should not fight Font Bakery expectations.
