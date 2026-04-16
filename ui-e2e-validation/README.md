# ui-e2e-validation

Headless-browser validation: render every font in `data/fonts/` with
`@font-face`, screenshot it via
[accretional/chromerpc](https://github.com/accretional/chromerpc), and
keep the images for visual regression.

## What it does

- `generator.go` — walks `data/fonts/`, writes one `<slug>.html`
  sample page and one `<slug>.textproto` chromerpc automation per
  font into a temp directory.
- `e2e_test.go` — exercises the generator unconditionally, and under
  `UI_E2E=1` shells out to `chromerpc-automate` against a running
  chromerpc server to actually capture screenshots.

## Quick run

```sh
# 1. Install/start chromerpc (sibling accretional repo).
cd ../chromerpc && make run          # headless Chrome + gRPC on :50051

# 2. From this repo, opt in.
UI_E2E=1 go test ./ui-e2e-validation/...
```

By default the test looks for `chromerpc-automate` on PATH, then falls
back to `go run ../chromerpc/cmd/automate`. Override with
`CHROMERPC_AUTOMATE_CMD` (space-separated command) or point at a
different server with `CHROMERPC_ADDR=host:port`.

## How the URL wiring works

`TestChromerpcScreenshots` stands up `httptest.NewServer` with two
routes:

| Path      | Source                                             |
| --------- | -------------------------------------------------- |
| `/fonts/` | `data/fonts/` (so `@font-face src:` resolves)      |
| `/html/`  | generated samples in the temp dir                  |

Each automation textproto navigates to
`http://127.0.0.1:<port>/html/<slug>.html`, waits 500 ms for the font
to load, then writes a full-page PNG to
`<tmp>/screenshots/<slug>.png`.

## Why not use `file://`?

@font-face cross-origin rules make local file loading finicky. A tiny
loopback HTTP server is simpler and mirrors production behaviour.
