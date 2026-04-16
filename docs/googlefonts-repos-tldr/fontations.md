# googlefonts/fontations

<https://github.com/googlefonts/fontations>

Rust family of crates for **reading and writing font files** — `read-fonts`,
`write-fonts`, `skrifa` (shaping-adjacent metrics), `font-test-data`.

Why we care:

- When our decoder hits an edge case (weird subtable, obscure table), the
  `read-fonts` traits are a good second opinion on what the field is
  supposed to contain.
- `font-test-data` in this repo ships small, carefully-crafted fixtures
  covering specific format corners. If we need a hand-curated fixture for
  our `data/fonts/handwritten/` dir, grab it from here (OFL-compatible).
- Active upstream; changes land quickly. Good place to watch for spec
  interpretation drift.
