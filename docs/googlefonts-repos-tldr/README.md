# `googlefonts` org repos — TL;DR index

Source: <https://github.com/orgs/googlefonts/repositories>

Only the repos that are directly relevant to proto-font (format tooling,
axis registry, language metadata, variable fonts, QA) are summarised here.
Repos in the org that host *individual font families* are out of scope on
purpose — we care about the ecosystem, not about mass-importing every face.

See the per-repo files in this directory.

| file | repo | why we care |
| ---- | ---- | ----------- |
| [gftools.md](gftools.md) | `googlefonts/gftools` | Canonical protos (`axes`, `fonts_public`, `designers`, `knowledge`) and the `gftools fix-*` helpers that normalise binaries |
| [lang.md](lang.md) | `googlefonts/lang` | `languages_public.proto` — language/region/script metadata used to filter catalog queries |
| [axisregistry.md](axisregistry.md) | `googlefonts/axisregistry` | Canonical text-proto data files for registered variable-font axes |
| [fontations.md](fontations.md) | `googlefonts/fontations` | Rust read/write font libs; good reference for edge cases in OpenType parsing |
| [fontc.md](fontc.md) | `googlefonts/fontc` | Rust rewrite of `fontmake`; reference for the source → binary pipeline |
| [fontbakery.md](fontbakery.md) | `googlefonts/fontbakery` | QA checks that font files must pass |
| [diffenator3.md](diffenator3.md) | `googlefonts/diffenator3` | Binary diffing tool; models of "what differs meaningfully between two builds" |
| [glyphsLib.md](glyphsLib.md) | `googlefonts/glyphsLib` | Glyphs file → UFO, upstream of most new fonts |
| [nototools.md](nototools.md) | `googlefonts/nototools` | Noto-specific QA & packaging helpers |

## Not summarised on purpose

- Every `noto-*` font family repo — we pull sample binaries via setup.sh.
- Language-showcase repos (`japanese`, `korean`, `thai`, etc.) — marketing
  surfaces, not tooling.
- `fb-variable-*` — experimental spacing axes; interesting but not yet used
  by any catalog family.
