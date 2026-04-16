# googlefonts/fontbakery

<https://github.com/googlefonts/fontbakery>

Python QA tool. Runs hundreds of checks against a font binary and flags
anything that would block inclusion in the Google Fonts catalog.

Checks that map onto proto-font fields:

- **Name table** (`com.google.fonts/check/name/*`) — constrains `NameRecord`
  content per platform.
- **OS/2** (`check/fsselection`, `check/usweightclass`, `check/xavgcharwidth`)
  — constrains the `OS2Table` fields we model.
- **head / hhea** — magic number 0x5F0F3CF5, sanity-checked ascender/descender
  vs OS/2 vertical metrics.
- **Post** — italicAngle, underlinePosition, isFixedPitch conventions.
- **cmap** — must include format-4 BMP subtable and format-12 when any
  codepoint ≥ 0x10000.

Treat Font Bakery's output as the definitive answer to "does this font
binary match the catalog's expectations". Our codec should round-trip the
same bytes it saw, so any Font Bakery failure on decoded+re-encoded output
is a codec bug.
