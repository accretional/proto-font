# googlefonts/glyphsLib

<https://github.com/googlefonts/glyphsLib>

Python bridge from Glyphs.app's `.glyphs` source format to the UFO format
consumed by fontmake / fontc.

Not a direct dependency, but worth knowing:

- Most new fonts land in `google/fonts` from a `.glyphs` source.
- The `.glyphs` format carries hints, kerning, and anchor data that must
  survive the build. Bugs in glyphsLib produce binaries where our codec
  will see unexpected table content — worth checking here first when a
  fixture decodes weirdly.
