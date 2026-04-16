# testing

Three layered suites all driven off the same `data/fonts/` fixture directory:

| suite | what it checks | how it runs |
| ----- | ------------- | ----------- |
| `validation/` | Decode+Encode byte-exact round-trip for every fixture, plus structural assertions (e.g. `head.magicNumber == 0x5F0F3CF5`). | `go test ./testing/validation/... -count=1` (wrapped by `./test.sh`) |
| `fuzz/`       | `FuzzDecode` must never panic on arbitrary bytes; `FuzzRoundTrip` asserts that anything that decodes also re-encodes bit-identical. | `go test ./testing/fuzz/... -fuzz=^FuzzX$` per target (wrapped by `./test.sh` with `FUZZ_TIME=3s` by default). |
| `benchmarks/` | `BenchmarkDecode` / `BenchmarkEncode` sized in bytes/op across every fixture. | `go test ./testing/benchmarks/... -bench=. -benchtime=1x` (wrapped by `./test.sh`) |

## Fixture source of truth

- `data/fonts/handwritten/` — (currently empty; placeholder for hand-crafted
  fixtures that exercise specific corners: otc files, eot, mac-platform name
  records, etc.).
- `data/fonts/noto/` — pulled at `setup.sh` time from the `google/fonts`
  repository. Three variable-font `.ttf`s are enough to exercise every
  container code path because each is ≥ 500 KB and carries the full
  complement of OpenType tables (`head`, `hhea`, `maxp`, `name`, `OS/2`,
  `post`, `cmap`, `hmtx`, `loca`/`glyf`, `GDEF`, `GSUB`, `GPOS`, `fvar`,
  `gvar`, `STAT`, `HVAR`). Not checked in; gitignored.
- The test harness uses `runtime.Caller` to locate the data dir, so tests
  work regardless of the caller's cwd.

## Known discrepancies and limitations

- **WOFF2**: only the fixed-length header and verbatim `compressed_stream`
  are decoded. The per-table directory uses WOFF2's 255UInt16 encoding and
  the per-table transforms require a brotli decoder. Round-trip is still
  byte-exact because Encode uses `raw_bytes`. Listed in README NEXT STEPS.
- **TTC synthesis**: constructing a `.ttc` from scratch (no `raw_bytes`)
  requires re-packing shared table bodies across fonts. Deferred; Encode
  errors out. Round-trip from `raw_bytes` works.
- **EOT synthesis**: same as TTC — round-trip OK, synthesis returns an
  error.
- **cmap subtables**: the directory and subtable-first-2-bytes are parsed,
  but subtable bodies are kept as opaque bytes. `SfntTable.RawData` remains
  authoritative for round-trip.
- **head.checkSumAdjustment**: the decoder captures the declared value, but
  any synthesised SFNT that regenerates this field from scratch must match
  OpenType §5.head's whole-file folding recipe. Round-trip via `raw_bytes`
  preserves the original value.

## How to add a handwritten fixture

Drop the file in `data/fonts/handwritten/NN_<what>.<ext>`; re-running
`./test.sh` will pick it up. Numbered prefix keeps related files grouped.
