# googlefonts/axisregistry

<https://github.com/googlefonts/axisregistry>

Canonical definitions of **variable-font axes** (wght, wdth, opsz, slnt,
ital, GRAD, XTRA, etc.).

- The **schema** (`AxisProto`) lives in
  `googlefonts/gftools/Lib/gftools/axes.proto` — we vendor *that* file, not
  a separate one from this repo.
- The **data** (one text-proto per axis) lives under
  `Lib/axisregistry/data/<tag>.textproto`. These are authoritative for
  minimum / default / maximum values, UI labels, and fallback static instance
  names.
- `axisregistry` also exposes a Python package (`pip install axisregistry`)
  for in-process lookup, used by `gftools packager` and Font Bakery.

## Relevance

- When we model variable-font metadata we should resolve every axis tag we
  see against this registry to produce consistent labels.
- Axes missing from the registry are "private" and the Google Fonts catalog
  will not accept them without registration.
