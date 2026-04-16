# googlefonts/fontc

<https://github.com/googlefonts/fontc>

Rust-based replacement for `fontmake` (the Python source→binary compiler).

- Consumes `.glyphs` / UFO sources and emits TTF / variable TTF.
- Built on top of `fontations` (shared tables).
- Much faster than fontmake; early but usable.

Relevance: not a direct consumer of our proto, but a useful reference for
what "correctly written" tables look like when we synthesise a font from
scratch via `Encode(nil raw_bytes)`.
