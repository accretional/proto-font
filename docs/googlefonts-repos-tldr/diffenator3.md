# googlefonts/diffenator3

<https://github.com/googlefonts/diffenator3>

Binary-level font-diffing tool, successor to Diffenator and Diffenator2,
implemented in Rust.

Useful as:

- A reference for "which differences between two binaries are meaningful"
  (glyph-outline changes, cmap coverage, OS/2 flag flips) vs noise
  (checkSumAdjustment, pad bytes).
- Validation: after we round-trip a file through Decode+Encode we can diff
  against the original with diffenator3; any reported difference is a codec
  bug. Not wired into CI — call it manually from a shell.
